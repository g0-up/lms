import { useMutation, useQueryClient } from "@tanstack/react-query";
import { authApi, meKey } from "../api/auth-api";

/**
 * Signs in. On success the whole cache is dropped before the new user is stored,
 * so nothing fetched for a previous user on this tab survives the switch.
 * Errors are rendered by the form (server copy verbatim), not toasted.
 */
export function useLogin() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: authApi.login,
    meta: { silent: true },
    onSuccess: (user) => {
      queryClient.clear();
      queryClient.setQueryData(meKey, user);
    },
  });
}
