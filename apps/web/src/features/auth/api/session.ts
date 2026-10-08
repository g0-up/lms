import type { QueryClient } from "@tanstack/react-query";
import { meKey } from "./auth-api";

/**
 * Forgets everything fetched for the current user and records "signed out", so the next
 * user on this tab starts from an empty cache and guest routes need no extra request.
 */
export function resetSession(queryClient: QueryClient): void {
  queryClient.clear();
  queryClient.setQueryData(meKey, null);
}
