import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import { hasCode, isApiError } from "@/shared/api/errors";
import { learningApi, learningKeys } from "../api/learning-api";
import { NOT_MEMBER_MESSAGE, NOT_OPENED_MESSAGE } from "../model/messages";
import type { LessonPage, ReadOnlyReason, Roadmap } from "../model/schemas";
import { tickBlocker } from "../model/tick-rule";
import { updateRoadmapLesson } from "./use-my-class";

export interface ToggleCompletionInput {
  lessonId: string;
  completed: boolean;
}

interface Snapshot {
  roadmap: Roadmap | undefined;
  lesson: LessonPage | undefined;
}

/** Toast copy for a refused tick; `null` when the app's session handling already took over. */
export function completionErrorMessage(error: Error, readOnlyReason: ReadOnlyReason | undefined): string | null {
  if (!isApiError(error)) return error.message;
  if (error.status === 401 || hasCode(error, "PASSWORD_CHANGE_REQUIRED")) return null;
  if (hasCode(error, "CONFLICT")) return NOT_OPENED_MESSAGE;
  if (hasCode(error, "INVALID_TRANSITION")) {
    return readOnlyReason ? tickBlocker({ readOnlyReason, opened: true }) : error.message;
  }
  if (error.status === 404) return NOT_MEMBER_MESSAGE;
  return error.message;
}

/**
 * Tick / untick a lesson. Optimistic on the roadmap and lesson caches, rolled back on error; the
 * course-wide totals come from the response, never from a client recount.
 */
export function useToggleCompletion(classId: string) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const roadmapKey = learningKeys.myClass(classId);

  function setCompletedAt(lessonId: string, completedAt: string | undefined) {
    queryClient.setQueryData<Roadmap>(roadmapKey, (r) =>
      r ? updateRoadmapLesson(r, lessonId, (l) => ({ ...l, completedAt })) : r,
    );
    queryClient.setQueryData<LessonPage>(learningKeys.lesson(classId, lessonId), (p) =>
      p ? { ...p, progress: { ...p.progress, completedAt } } : p,
    );
  }

  return useMutation({
    mutationFn: ({ lessonId, completed }: ToggleCompletionInput) =>
      learningApi.setCompletion(classId, lessonId, completed),
    meta: { silent: true },
    onMutate: async ({ lessonId, completed }): Promise<Snapshot> => {
      const lessonKey = learningKeys.lesson(classId, lessonId);
      await Promise.all([
        queryClient.cancelQueries({ queryKey: roadmapKey }),
        queryClient.cancelQueries({ queryKey: lessonKey }),
      ]);
      const snapshot = {
        roadmap: queryClient.getQueryData<Roadmap>(roadmapKey),
        lesson: queryClient.getQueryData<LessonPage>(lessonKey),
      };
      setCompletedAt(lessonId, completed ? new Date().toISOString() : undefined);
      return snapshot;
    },
    onSuccess: (result, { lessonId, completed }) => {
      setCompletedAt(lessonId, result.completedAt ?? undefined);
      queryClient.setQueryData<Roadmap>(roadmapKey, (r) =>
        r
          ? { ...r, percent: result.percent, requiredDone: result.requiredDone, requiredTotal: result.requiredTotal }
          : r,
      );
      toast.success(completed ? "Đã ghi nhận hoàn thành." : "Đã bỏ tích.");
    },
    onError: (error, { lessonId }, snapshot) => {
      if (snapshot) {
        queryClient.setQueryData(roadmapKey, snapshot.roadmap);
        queryClient.setQueryData(learningKeys.lesson(classId, lessonId), snapshot.lesson);
      }
      const message = completionErrorMessage(error, snapshot?.roadmap?.readOnlyReason);
      if (message === null) return;
      if (isApiError(error) && error.status === 404) {
        toast.error(message, { id: NOT_MEMBER_MESSAGE });
        void navigate("/learn", { replace: true });
        return;
      }
      toast.error(message);
      // The class may have just started or ended: reload it so the right banner and blockers show.
      if (hasCode(error, "INVALID_TRANSITION")) void queryClient.invalidateQueries({ queryKey: roadmapKey });
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: learningKeys.myClasses }),
  });
}
