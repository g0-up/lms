import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { hasCode } from "@/shared/api/errors";
import { stageKeys, stagesApi } from "../api/stages-api";
import { toLessonBody, toLessonPatch, type LessonValues } from "../model/lesson-form";
import { applyLayout, type UnitLessons } from "../model/order";
import type { Lesson, LessonType, StageVersion } from "../model/schemas";
import { REORDER_FAILED_MESSAGE, VERSION_IMMUTABLE_MESSAGE } from "../model/stage-rules";

export interface SaveLessonInput {
  /** Absent when adding. */
  lessonId?: string;
  values: LessonValues;
  /** Leave an existing markdown lesson's source out of the patch, so the stored one stays as is. */
  keepSource?: boolean;
  /** The edited lesson's type before the save; the patch changes the type only when it differs. */
  from?: LessonType;
}

/** Toast after a save; a new quiz or homework points at what is still to write on its page. */
function savedMessage(lesson: Lesson, added: boolean): string {
  if (!added) return "Đã lưu học liệu.";
  if (lesson.type === "quiz") return "Đã thêm học liệu. Tiếp theo: soạn câu hỏi.";
  if (lesson.type === "homework") return "Đã thêm học liệu. Tiếp theo: soạn tiêu chí chấm.";
  return "Đã thêm học liệu.";
}

/** Refetches a version after a content change; lesson counts show on the stage detail and list too. */
export function useContentRefresh(stageId: string, vid: string) {
  const queryClient = useQueryClient();
  return () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: stageKeys.version(vid) }),
      queryClient.invalidateQueries({ queryKey: stageKeys.detail(stageId) }),
      queryClient.invalidateQueries({ queryKey: stageKeys.list }),
    ]);
}

/** Puts a lesson the server answered with into the cached version at once, then refetches what counts it. */
export function useLessonResult(stageId: string, vid: string) {
  const queryClient = useQueryClient();
  const refresh = useContentRefresh(stageId, vid);
  return async (lesson: Lesson) => {
    queryClient.setQueryData<StageVersion>(stageKeys.version(vid), (version) =>
      version ? { ...version, lessons: version.lessons.map((l) => (l.id === lesson.id ? lesson : l)) } : version,
    );
    await refresh();
  };
}

/**
 * Optimistic reorder of one lesson's questions or criteria: the page shows `arrange(lesson, ids)` at
 * once and rolls back if the server refuses, then refetches either way.
 */
export function useLessonItemsReorder(
  stageId: string,
  vid: string,
  lessonId: string,
  send: (ids: string[]) => Promise<Lesson>,
  arrange: (lesson: Lesson, ids: string[]) => Lesson,
) {
  const queryClient = useQueryClient();
  const store = useLessonResult(stageId, vid);
  const versionKey = stageKeys.version(vid);
  return useMutation({
    mutationFn: send,
    meta: { silent: true },
    onMutate: async (ids: string[]) => {
      await queryClient.cancelQueries({ queryKey: versionKey });
      const previous = queryClient.getQueryData<StageVersion>(versionKey);
      if (previous) {
        queryClient.setQueryData<StageVersion>(versionKey, {
          ...previous,
          lessons: previous.lessons.map((l) => (l.id === lessonId ? arrange(l, ids) : l)),
        });
      }
      return { previous };
    },
    onSuccess: store,
    onError: (_error, _ids, context) => {
      if (context?.previous) queryClient.setQueryData(versionKey, context.previous);
      toast.error(REORDER_FAILED_MESSAGE);
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: versionKey }),
  });
}

/** Toast for a failed delete on a draft that may have been published meanwhile; refetches in either case. */
export function useDeleteFailed(stageId: string, vid: string) {
  const refresh = useContentRefresh(stageId, vid);
  return (error: Error) => {
    toast.error(hasCode(error, "VERSION_IMMUTABLE") ? VERSION_IMMUTABLE_MESSAGE : error.message);
    void refresh();
  };
}

/**
 * Optimistic reorder of a version's units and lessons: the editor shows `layout` at once and rolls
 * back if the server refuses, then refetches either way so a stale layout cannot linger.
 */
export function useLayoutMutation<T>(
  vid: string,
  send: (input: T) => Promise<StageVersion>,
  layoutFor: (version: StageVersion, input: T) => UnitLessons[],
) {
  const queryClient = useQueryClient();
  const versionKey = stageKeys.version(vid);
  return useMutation({
    mutationFn: send,
    meta: { silent: true },
    onMutate: async (input: T) => {
      await queryClient.cancelQueries({ queryKey: versionKey });
      const previous = queryClient.getQueryData<StageVersion>(versionKey);
      if (previous)
        queryClient.setQueryData<StageVersion>(versionKey, applyLayout(previous, layoutFor(previous, input)));
      return { previous };
    },
    onSuccess: (version) => {
      queryClient.setQueryData(versionKey, version);
    },
    onError: (_error, _input, context) => {
      if (context?.previous) queryClient.setQueryData(versionKey, context.previous);
      toast.error(REORDER_FAILED_MESSAGE);
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: versionKey }),
  });
}

/** Add, edit, delete and reorder lessons of one draft stage version. */
export function useLessonMutations(stageId: string, vid: string) {
  const refresh = useContentRefresh(stageId, vid);
  const deleteFailed = useDeleteFailed(stageId, vid);

  // The lesson dialog and editor page show save errors themselves (VERSION_IMMUTABLE with its own copy).
  const save = useMutation({
    mutationFn: ({ lessonId, values, keepSource, from }: SaveLessonInput) =>
      lessonId
        ? stagesApi.updateLesson(vid, lessonId, toLessonPatch(values, { keepSource, from }))
        : stagesApi.addLesson(vid, toLessonBody(values)),
    meta: { silent: true },
    onSuccess: async (lesson, { lessonId }) => {
      await refresh();
      toast.success(savedMessage(lesson, !lessonId));
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
    onError: deleteFailed,
  });

  // Takes the full layout the server wants: every unit in order with every one of its lessons.
  const reorder = useLayoutMutation(
    vid,
    (layout: UnitLessons[]) => stagesApi.reorderLessons(vid, layout),
    (_version, layout) => layout,
  );

  return { save, remove, reorder };
}
