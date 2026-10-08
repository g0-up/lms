import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { hasCode } from "@/shared/api/errors";
import { classesApi, type InviteBody } from "../api/classes-api";
import type { Member } from "../model/schemas";
import { invalidateClass, toastFailure } from "./use-class-mutations";

/** Invites by email; the dialog shows the error, the toast copy depends on whether the account existed. */
export function useInvite(classId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: InviteBody) => classesApi.invite(classId, body),
    meta: { silent: true },
    onSuccess: ({ kind, member }) => {
      toast.success(
        kind === "added"
          ? `${member.fullName} đã có tài khoản: đã thêm vào lớp và gửi thông báo.`
          : `Đã tạo tài khoản cho ${member.fullName}, lời mời đang được gửi.`,
      );
      void invalidateClass(queryClient, classId);
    },
    // The class ended elsewhere: reload it so the page stops offering invitations.
    onError: (error) => {
      if (hasCode(error, "INVALID_TRANSITION")) void invalidateClass(queryClient, classId);
    },
  });
}

export const ALREADY_ACTIVATED = "Học viên đã đổi mật khẩu; không cần gửi lại lời mời.";
export const RESEND_RATE_LIMITED = "Đã gửi lại quá nhiều lần. Thử lại sau.";

/** New temporary password and invitation; the previous password stops working at once. */
export function useResend(classId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (member: Member) => classesApi.resend(classId, member.id),
    meta: { silent: true },
    onSuccess: () => {
      toast.success("Lời mời mới đang được gửi.");
    },
    onError: (error) => {
      if (hasCode(error, "CONFLICT")) toast.error(ALREADY_ACTIVATED);
      else if (hasCode(error, "RATE_LIMITED")) toast.error(RESEND_RATE_LIMITED);
      else toastFailure(error);
    },
    onSettled: () => invalidateClass(queryClient, classId),
  });
}

/** Moves the member to "left the class"; progress is kept. */
export function useRemoveMember(classId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (member: Member) => classesApi.removeMember(classId, member.id),
    meta: { silent: true },
    onSuccess: (_removed, member) => {
      toast.success(`Đã gỡ ${member.fullName} khỏi lớp.`);
    },
    onError: toastFailure,
    onSettled: () => invalidateClass(queryClient, classId),
  });
}
