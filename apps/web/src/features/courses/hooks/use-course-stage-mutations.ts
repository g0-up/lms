import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ApiError, isApiError } from "@/shared/api/errors";
import { VERSION_IMMUTABLE_MESSAGE } from "@/features/stages";
import { courseKeys, coursesApi, relatedKeys } from "../api/courses-api";
import type { CourseVersion } from "../model/schemas";

export type StageEdit = "add" | "swap" | "remove" | "move";

export interface SetStagesInput {
  kind: StageEdit;
  /** The draft's complete stage version list after the edit, in order. */
  stageVersionIds: string[];
}

const SUCCESS: Record<StageEdit, string | null> = {
  add: "Đã thêm chặng.",
  swap: "Đã đổi sang phiên bản mới.",
  remove: "Đã gỡ chặng khỏi bản nháp.",
  move: null,
};

/** The stages of `previous` in the order of `ids`, or `undefined` when an id is new (only the server knows its row). */
function reordered(previous: CourseVersion, ids: readonly string[]): CourseVersion | undefined {
  const byId = new Map(previous.stages.map((s) => [s.stageVersionId, s]));
  const stages = ids.map((id) => byId.get(id));
  if (stages.some((s) => s === undefined)) return undefined;
  return { ...previous, stages: stages.flatMap((s, i) => (s ? [{ ...s, position: i + 1 }] : [])) };
}

/**
 * Edits to a draft course version's stage list, all through `PUT /course-versions/{vid}/stages`.
 * Removing and moving update the list at once and roll back on failure; adding and swapping wait
 * for the server's row. Add errors stay in its dialog; the others become a toast.
 */
export function useCourseStageMutations(courseId: string, vid: string) {
  const queryClient = useQueryClient();
  const versionKey = courseKeys.version(vid);

  return useMutation({
    mutationFn: async ({ stageVersionIds }: SetStagesInput) => {
      try {
        return await coursesApi.setStages(vid, stageVersionIds);
      } catch (error) {
        if (isApiError(error) && error.code === "VERSION_IMMUTABLE") {
          throw new ApiError(error.status, error.code, VERSION_IMMUTABLE_MESSAGE, error.details);
        }
        throw error;
      }
    },
    meta: { silent: true },
    onMutate: async ({ stageVersionIds }) => {
      await queryClient.cancelQueries({ queryKey: versionKey });
      const previous = queryClient.getQueryData<CourseVersion>(versionKey);
      const optimistic = previous ? reordered(previous, stageVersionIds) : undefined;
      if (optimistic) queryClient.setQueryData(versionKey, optimistic);
      return { previous };
    },
    onSuccess: (version, { kind }) => {
      queryClient.setQueryData(versionKey, version);
      const message = SUCCESS[kind];
      if (message) toast.success(message);
    },
    onError: (error, { kind }, context) => {
      if (context?.previous) queryClient.setQueryData(versionKey, context.previous);
      if (kind !== "add") toast.error(error.message);
    },
    // Stage counts show on the course detail and list; usage shows on the stage pages.
    onSettled: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: versionKey }),
        queryClient.invalidateQueries({ queryKey: courseKeys.detail(courseId) }),
        queryClient.invalidateQueries({ queryKey: courseKeys.list }),
        queryClient.invalidateQueries({ queryKey: relatedKeys.stages }),
        queryClient.invalidateQueries({ queryKey: relatedKeys.stageDetails }),
      ]),
  });
}
