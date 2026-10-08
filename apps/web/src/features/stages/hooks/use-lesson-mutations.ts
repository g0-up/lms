import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { hasCode } from "@/shared/api/errors";
import { stageKeys, stagesApi } from "../api/stages-api";
import { toLessonBody, toLessonPatch, type LessonValues } from "../model/lesson-form";
import { orderByIds } from "../model/order";
import type { Lesson, StageVersion } from "../model/schemas";
import { REORDER_FAILED_MESSAGE, VERSION_IMMUTABLE_MESSAGE } from "../model/stage-rules";

export interface SaveLessonInput {
  /** Absent when adding. */
  lessonId?: string;
  values: LessonValues;
  /** Leave an existing markdown lesson's source out of the patch, so the stored one stays as is. */
  keepSource?: boolean;
}

/** Add, edit, delete and reorder lessons of one draft stage version. */
export function useLessonMutations(stageId: string, vid: string) {
  const queryClient = useQueryClient();
  const versionKey = stageKeys.version(vid);

  // Lesson counts show on the stage detail and list too.
  function refresh() {
    return Promise.all([
      queryClient.invalidateQueries({ queryKey: versionKey }),
      queryClient.invalidateQueries({ queryKey: stageKeys.detail(stageId) }),
      queryClient.invalidateQueries({ queryKey: stageKeys.list }),
    ]);
  }

  // The lesson dialog and editor page show save errors themselves (VERSION_IMMUTABLE with its own copy).
  const save = useMutation({
    mutationFn: ({ lessonId, values, keepSource }: SaveLessonInput) =>
      lessonId
        ? stagesApi.updateLesson(vid, lessonId, toLessonPatch(values, { keepSource }))
        : stagesApi.addLesson(vid, toLessonBody(values)),
    meta: { silent: true },
    onSuccess: async (_lesson, { lessonId }) => {
      await refresh();
      toast.success(lessonId ? "Đã lưu học liệu." : "Đã thêm học liệu.");
    },
    onError: (error) => {
      if (hasCode(error, "VERSION_IMMUTABLE")) void refresh();
    },
  });

  const remove = useMutation({
    mutationFn: (lesson: Lesson) => stagesApi.removeLesson(vid, lesson.id),
    meta: { silent: true },
    onSuccess: async () => {
      await refresh();
      toast.success("Đã xóa học liệu.");
    },
    onError: (error) => {
      toast.error(hasCode(error, "VERSION_IMMUTABLE") ? VERSION_IMMUTABLE_MESSAGE : error.message);
      void refresh();
    },
  });

  // Optimistic: the list reorders at once and rolls back if the server refuses.
  const reorder = useMutation({
    mutationFn: (lessonIds: string[]) => stagesApi.reorderLessons(vid, lessonIds),
    meta: { silent: true },
    onMutate: async (lessonIds) => {
      await queryClient.cancelQueries({ queryKey: versionKey });
      const previous = queryClient.getQueryData<StageVersion>(versionKey);
      if (previous) queryClient.setQueryData<StageVersion>(versionKey, { ...previous, lessons: orderByIds(previous.lessons, lessonIds) });
      return { previous };
    },
    onSuccess: (version) => {
      queryClient.setQueryData(versionKey, version);
    },
    onError: (_error, _ids, context) => {
      if (context?.previous) queryClient.setQueryData(versionKey, context.previous);
      toast.error(REORDER_FAILED_MESSAGE);
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: versionKey }),
  });

  return { save, remove, reorder };
}
