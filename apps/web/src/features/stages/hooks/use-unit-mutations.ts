import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { hasCode } from "@/shared/api/errors";
import { stagesApi } from "../api/stages-api";
import { layoutOf, type UnitLessons } from "../model/order";
import type { StageVersion, Unit } from "../model/schemas";
import { VERSION_IMMUTABLE_MESSAGE } from "../model/stage-rules";
import { useContentRefresh, useLayoutMutation } from "./use-lesson-mutations";

/** The version's layout with its units in the order of `unitIds`. */
function unitOrder(version: StageVersion, unitIds: readonly string[]): UnitLessons[] {
  const layout = layoutOf(version);
  return unitIds.flatMap((id) => layout.filter((u) => u.unitId === id));
}

/** Add, rename, delete and reorder the bài học of one draft stage version. */
export function useUnitMutations(stageId: string, vid: string) {
  const refresh = useContentRefresh(stageId, vid);

  // The unit dialog shows save errors itself (a taken title, a refused length).
  const save = useMutation({
    mutationFn: ({ unitId, title }: { unitId?: string; title: string }) =>
      unitId ? stagesApi.renameUnit(vid, unitId, title) : stagesApi.createUnit(vid, title),
    meta: { silent: true },
    onSuccess: async (_unit, { unitId }) => {
      await refresh();
      toast.success(unitId ? "Đã lưu bài học." : "Đã thêm bài học. Tiếp theo: thêm học liệu vào bài học.");
    },
    onError: (error) => {
      if (hasCode(error, "VERSION_IMMUTABLE")) void refresh();
    },
  });

  // Deleting a unit deletes its lessons too.
  const remove = useMutation({
    mutationFn: (unit: Unit) => stagesApi.removeUnit(vid, unit.id),
    meta: { silent: true },
    onSuccess: async () => {
      await refresh();
      toast.success("Đã xóa bài học.");
    },
    onError: (error) => {
      toast.error(hasCode(error, "VERSION_IMMUTABLE") ? VERSION_IMMUTABLE_MESSAGE : error.message);
      void refresh();
    },
  });

  const reorder = useLayoutMutation(vid, (unitIds: string[]) => stagesApi.reorderUnits(vid, unitIds), unitOrder);

  return { save, remove, reorder };
}
