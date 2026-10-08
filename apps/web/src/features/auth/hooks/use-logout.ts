import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router";
import { authApi } from "../api/auth-api";
import { LOGIN_PATH } from "../model/navigation";
import { resetSession } from "../api/session";

/**
 * Signs out and returns to /login. Local state is dropped even when the request fails
 * (offline, session already gone): the user asked to leave, so the UI never keeps them in.
 */
export function useLogout() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  return useMutation({
    mutationFn: () => authApi.logout(),
    meta: { silent: true },
    onSettled: () => {
      resetSession(queryClient);
      void navigate(LOGIN_PATH, { replace: true });
    },
  });
}
