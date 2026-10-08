import { MutationCache, QueryCache, QueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { FIRST_LOGIN_PATH, LOGIN_PATH, loginUrl, meKey, resetSession } from "@/features/auth";
import { hasCode, isApiError } from "@/shared/api/errors";
import type { User } from "@/shared/api/schemas";

export interface QueryClientDeps {
  /** Router navigation (bound after the router exists, which itself needs the client). */
  navigate: (to: string) => void;
  /** Current in-app location, read when an error arrives. */
  getLocation: () => { pathname: string; search: string };
}

/**
 * 4xx answers are final and retrying cannot change them; network failures (status 0) and 5xx
 * get two more attempts.
 */
export function shouldRetry(failureCount: number, error: Error): boolean {
  if (isApiError(error) && error.status > 0 && error.status < 500) return false;
  return failureCount < 2;
}

/**
 * Query client with the app's central auth handling:
 * - 401 from any query or mutation: drop the whole cache (no data of the previous user survives),
 *   mark the user signed out and go to `/login?next=<here>`. Skipped on /login itself, where 401
 *   is the answer to wrong credentials.
 * - 403 PASSWORD_CHANGE_REQUIRED: go to /first-login.
 * - Other mutation errors: toast the server message unless the mutation sets `meta.silent`.
 */
export function createAppQueryClient({ navigate, getLocation }: QueryClientDeps): QueryClient {
  // Returns true when the error was a session problem that has been dealt with here.
  function handleSessionError(error: Error): boolean {
    if (!isApiError(error)) return false;
    const { pathname, search } = getLocation();
    if (error.status === 401) {
      if (pathname === LOGIN_PATH) return false;
      resetSession(queryClient);
      navigate(loginUrl(pathname + search));
      return true;
    }
    if (error.status === 403 && hasCode(error, "PASSWORD_CHANGE_REQUIRED")) {
      queryClient.setQueryData<User | null>(meKey, (user) => (user ? { ...user, mustChangePassword: true } : user));
      if (pathname !== FIRST_LOGIN_PATH) navigate(FIRST_LOGIN_PATH);
      return true;
    }
    return false;
  }

  const queryClient: QueryClient = new QueryClient({
    queryCache: new QueryCache({
      onError: (error) => {
        handleSessionError(error);
      },
    }),
    mutationCache: new MutationCache({
      onError: (error, _variables, _context, mutation) => {
        if (handleSessionError(error)) return;
        if (mutation.meta?.silent) return;
        toast.error(error.message);
      },
    }),
    defaultOptions: {
      queries: { staleTime: 30_000, retry: shouldRetry },
      mutations: { retry: false },
    },
  });
  return queryClient;
}
