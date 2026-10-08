import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { reportsKeys } from "@/features/reports";
import { hasCode, isApiError } from "@/shared/api/errors";
import { classesApi, classesKeys, type CreateClassBody, type UpdateClassBody } from "../api/classes-api";
import type { ClassDetail } from "../model/schemas";

/** Toast copy for a failed action; `null` when the app's session handling already took over. */
export function failureMessage(error: Error): string | null {
  if (!isApiError(error)) return error.message;
  if (error.status === 401 || hasCode(error, "PASSWORD_CHANGE_REQUIRED")) return null;
  return error.message;
}

export function toastFailure(error: Error): void {
  const message = failureMessage(error);
  if (message !== null) toast.error(message);
}

/** Everything that shows the class or its members: list, detail, members, reports and dashboard. */
export function invalidateClass(queryClient: QueryClient, classId: string) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: classesKeys.list }),
    queryClient.invalidateQueries({ queryKey: classesKeys.detail(classId) }),
    queryClient.invalidateQueries({ queryKey: classesKeys.members(classId) }),
    queryClient.invalidateQueries({ queryKey: reportsKeys.reports(classId) }),
    queryClient.invalidateQueries({ queryKey: reportsKeys.memberReports(classId) }),
    queryClient.invalidateQueries({ queryKey: ["dashboard"] }),
  ]);
}

/** Creates a draft class; errors are shown by the dialog. */
export function useCreateClass() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateClassBody) => classesApi.create(body),
    meta: { silent: true },
    onSuccess: (created) => {
      queryClient.setQueryData(classesKeys.detail(created.id), created);
      void invalidateClass(queryClient, created.id);
    },
  });
}

/** Saves the settings form; errors are shown in the form. */
export function useUpdateClass(classId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdateClassBody) => classesApi.update(classId, body),
    meta: { silent: true },
    onSuccess: (updated) => {
      queryClient.setQueryData(classesKeys.detail(classId), updated);
      void invalidateClass(queryClient, classId);
    },
    // The class left draft elsewhere: reload it so the version select locks.
    onError: (error) => {
      if (hasCode(error, "INVALID_TRANSITION")) void invalidateClass(queryClient, classId);
    },
  });
}

export type Transition = "activate" | "end";

/**
 * Lifecycle step. A refused step toasts the server message; INVALID_TRANSITION means the class
 * already moved on elsewhere, so it is reloaded.
 */
export function useClassTransition(classId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (step: Transition): Promise<ClassDetail> =>
      step === "activate" ? classesApi.activate(classId) : classesApi.end(classId),
    meta: { silent: true },
    onSuccess: (updated, step) => {
      queryClient.setQueryData(classesKeys.detail(classId), updated);
      toast.success(step === "activate" ? `Lớp ${updated.code} đang chạy.` : `Lớp ${updated.code} đã kết thúc.`);
      void invalidateClass(queryClient, classId);
    },
    onError: (error) => {
      toastFailure(error);
      if (hasCode(error, "INVALID_TRANSITION")) void invalidateClass(queryClient, classId);
    },
  });
}
