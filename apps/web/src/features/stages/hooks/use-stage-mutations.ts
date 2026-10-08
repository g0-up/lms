import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import { hasCode } from "@/shared/api/errors";
import { relatedKeys, stageKeys, stagesApi } from "../api/stages-api";
import { draftExistsDetailsSchema, type StageVersion } from "../model/schemas";
import { draftExistsMessage } from "../model/stage-rules";

export const stagePath = (stageId: string) => `/admin/stages/${stageId}`;

/** `POST /stages`; the dialog maps CONFLICT to the code field. */
export function useCreateStage() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: stagesApi.create,
    meta: { silent: true },
    onSuccess: async (stage) => {
      queryClient.setQueryData(stageKeys.detail(stage.id), stage);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: stageKeys.list }),
        queryClient.invalidateQueries({ queryKey: relatedKeys.dashboard }),
      ]);
    },
  });
}

/** Toast for a refused clone. */
export function cloneErrorMessage(error: Error): string {
  if (hasCode(error, "DRAFT_EXISTS") && "details" in error) {
    const details = draftExistsDetailsSchema.safeParse(error.details);
    if (details.success) return draftExistsMessage(details.data.draftVersionNo);
  }
  return error.message;
}

/** Clone, publish, archive and delete of one stage's versions; each refreshes the stage, the list and the dashboard. */
export function useStageVersionActions(stageId: string) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const detailPath = stagePath(stageId);

  function refresh() {
    return Promise.all([
      queryClient.invalidateQueries({ queryKey: stageKeys.detail(stageId) }),
      queryClient.invalidateQueries({ queryKey: stageKeys.list }),
      queryClient.invalidateQueries({ queryKey: relatedKeys.dashboard }),
    ]);
  }

  function settle(version: StageVersion) {
    queryClient.setQueryData(stageKeys.version(version.id), version);
    return refresh();
  }

  const clone = useMutation({
    mutationFn: stagesApi.clone,
    meta: { silent: true },
    onSuccess: async (version) => {
      await settle(version);
      toast.success("Đã tạo bản nháp mới. Học liệu được sao chép, giữ nguyên lesson_key.");
      await navigate(`${detailPath}?v=${version.id}`);
    },
    onError: (error) => {
      toast.error(cloneErrorMessage(error));
      void refresh();
    },
  });

  // Publish and archive errors are shown inside their confirm dialog by the caller.
  const publish = useMutation({
    mutationFn: stagesApi.publish,
    meta: { silent: true },
    onSuccess: async (version) => {
      await settle(version);
      toast.success(`Đã phát hành v${String(version.versionNo)}.`);
    },
    onError: () => void refresh(),
  });

  const archive = useMutation({
    mutationFn: stagesApi.archive,
    meta: { silent: true },
    onSuccess: async (version) => {
      await settle(version);
      toast.success("Đã lưu trữ.");
    },
    onError: () => void refresh(),
  });

  const remove = useMutation({
    mutationFn: stagesApi.remove,
    meta: { silent: true },
    onSuccess: async ({ stageDeleted }, vid) => {
      toast.success("Đã xóa.");
      // Leave the page before dropping its caches so nothing refetches the deleted version.
      if (stageDeleted) {
        await navigate("/admin/stages");
        queryClient.removeQueries({ queryKey: stageKeys.detail(stageId) });
        await Promise.all([
          queryClient.invalidateQueries({ queryKey: stageKeys.list }),
          queryClient.invalidateQueries({ queryKey: relatedKeys.dashboard }),
        ]);
      } else {
        await refresh();
        await navigate(detailPath);
      }
      queryClient.removeQueries({ queryKey: stageKeys.version(vid) });
    },
    onError: (error) => {
      toast.error(error.message);
      void refresh();
    },
  });

  return { clone, publish, archive, remove };
}

/** `POST /stage-versions/{vid}/apply` for one course; the dialog reads the per-course rows. */
export function useApplyStageVersion(stageId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ vid, courseId }: { vid: string; courseId: string }) => stagesApi.apply(vid, [courseId]),
    meta: { silent: true },
    onSettled: (_data, _error, { courseId }) =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: stageKeys.detail(stageId) }),
        queryClient.invalidateQueries({ queryKey: stageKeys.list }),
        queryClient.invalidateQueries({ queryKey: relatedKeys.dashboard }),
        queryClient.invalidateQueries({ queryKey: relatedKeys.courses }),
        queryClient.invalidateQueries({ queryKey: relatedKeys.course(courseId) }),
      ]),
  });
}
