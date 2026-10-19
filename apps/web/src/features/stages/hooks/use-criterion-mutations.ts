import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { hasCode } from "@/shared/api/errors";
import { stagesApi } from "../api/stages-api";
import type { CriterionBody } from "../model/criterion-form";
import type { HomeworkSettingsBody } from "../model/lesson-form";
import { orderByIds } from "../model/order";
import type { Lesson } from "../model/schemas";
import { useContentRefresh, useDeleteFailed, useLessonItemsReorder, useLessonResult } from "./use-lesson-mutations";

export interface SaveCriterionInput {
  /** Absent when adding. */
  criterionId?: string;
  body: CriterionBody;
}

/** `lesson` with its criteria in the order of `ids`. */
function arrangeCriteria(lesson: Lesson, ids: string[]): Lesson {
  if (!lesson.homework) return lesson;
  return { ...lesson, homework: { ...lesson.homework, criteria: orderByIds(lesson.homework.criteria, ids) } };
}

/** Add, edit, delete and reorder the grading criteria of one homework in a draft version. */
export function useCriterionMutations(stageId: string, vid: string, lessonId: string) {
  const refresh = useContentRefresh(stageId, vid);
  const store = useLessonResult(stageId, vid);
  const deleteFailed = useDeleteFailed(stageId, vid);

  // The criterion dialog shows save errors itself.
  const save = useMutation({
    mutationFn: ({ criterionId, body }: SaveCriterionInput) =>
      criterionId
        ? stagesApi.updateCriterion(vid, lessonId, criterionId, body)
        : stagesApi.addCriterion(vid, lessonId, body),
    meta: { silent: true },
    onSuccess: async (lesson, { criterionId }) => {
      await store(lesson);
      toast.success(criterionId ? "Đã lưu tiêu chí." : "Đã thêm tiêu chí.");
    },
    onError: (error) => {
      if (hasCode(error, "VERSION_IMMUTABLE")) void refresh();
    },
  });

  const remove = useMutation({
    mutationFn: (criterionId: string) => stagesApi.removeCriterion(vid, lessonId, criterionId),
    meta: { silent: true },
    onSuccess: async () => {
      await refresh();
      toast.success("Đã xóa tiêu chí.");
    },
    onError: deleteFailed,
  });

  // Takes every criterion id of the homework in the new order.
  const reorder = useLessonItemsReorder(
    stageId,
    vid,
    lessonId,
    (ids) => stagesApi.reorderCriteria(vid, lessonId, ids),
    arrangeCriteria,
  );

  return { save, remove, reorder };
}

/**
 * Saves part of a homework's settings, the brief or the attachment list; fields left out keep what
 * the server has. The caller shows the error.
 */
export function useHomeworkPatch(stageId: string, vid: string, lessonId: string) {
  const refresh = useContentRefresh(stageId, vid);
  const store = useLessonResult(stageId, vid);
  return useMutation({
    mutationFn: (homework: HomeworkSettingsBody) => stagesApi.updateLesson(vid, lessonId, { homework }),
    meta: { silent: true },
    onSuccess: store,
    onError: (error) => {
      if (hasCode(error, "VERSION_IMMUTABLE")) void refresh();
    },
  });
}
