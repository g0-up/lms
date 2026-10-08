import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { classesApi } from "../api/classes-api";
import type { Member } from "../model/schemas";
import { invalidateClass, toastFailure } from "./use-class-mutations";

/**
 * Disables or re-enables the member's account (not just the membership): a disabled account
 * cannot sign in and its sessions are revoked.
 */
export function useSetUserEnabled(classId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ member, enabled }: { member: Member; enabled: boolean }) =>
      enabled ? classesApi.enableUser(member.userId) : classesApi.disableUser(member.userId),
    meta: { silent: true },
    onSuccess: (_user, { enabled }) => {
      toast.success(enabled ? "Đã kích hoạt lại tài khoản." : "Đã vô hiệu hóa tài khoản.");
    },
    onError: toastFailure,
    onSettled: () => invalidateClass(queryClient, classId),
  });
}
