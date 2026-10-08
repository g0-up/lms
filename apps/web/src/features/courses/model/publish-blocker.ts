import type { StageList, VersionLike } from "@/features/stages";
import { latestPublished, sortVersions } from "@/features/stages";
import type { CourseStage, CourseVersion } from "./schemas";

type VersionStatus = VersionLike["status"];

export const NEEDS_STAGE = "Khóa học cần ít nhất một chặng.";

/**
 * Why the draft cannot be published yet (button title and warning); `null` when it can.
 * `statusOf` resolves a stage version's status from the stage list; a version it does not know
 * yet is not reported here, the server still refuses the publish.
 */
export function coursePublishBlocker(
  version: Pick<CourseVersion, "status" | "stages">,
  statusOf: (stageVersionId: string) => VersionStatus | undefined,
): string | null {
  if (version.status !== "draft") return null;
  if (version.stages.length === 0) return NEEDS_STAGE;
  const unpublished = version.stages.filter((s) => {
    const status = statusOf(s.stageVersionId);
    return status !== undefined && status !== "published";
  });
  if (unpublished.length === 0) return null;
  return `Chặng chưa phát hành: ${unpublished.map((s) => `${s.stageName} v${String(s.stageVersionNo)}`).join(", ")}.`;
}

/** Why a published/archived version cannot be deleted; `null` when no class is attached to it. */
export function courseDeleteBlocker(version: Pick<CourseVersion, "status" | "classes">): string | null {
  if (version.status === "draft" || version.classes.length === 0) return null;
  return `Đang được dùng bởi lớp ${version.classes.map((c) => c.code).join(", ")}. Hãy lưu trữ thay vì xóa.`;
}

/** Bottom-bar summary of a course version. */
export function requiredTotalLabel(stages: readonly Pick<CourseStage, "requiredCount">[]): string {
  return `${String(stages.reduce((sum, s) => sum + s.requiredCount, 0))} học liệu bắt buộc toàn khóa`;
}

/** Status lookup over every version in the stage list. */
export function stageVersionStatuses(list: StageList | undefined): (stageVersionId: string) => VersionStatus | undefined {
  const statuses = new Map(list?.items.flatMap((s) => s.versions.map((v) => [v.id, v.status] as const)));
  return (stageVersionId) => statuses.get(stageVersionId);
}

export interface StageOption {
  value: string;
  label: string;
}

/**
 * "Thêm chặng" choices: every published version of the stages the course version does not use
 * yet, oldest first per stage; the highest one is marked "(mới nhất)".
 */
export function addStageOptions(list: StageList, used: readonly Pick<CourseStage, "stageId">[]): StageOption[] {
  const usedIds = new Set(used.map((s) => s.stageId));
  return list.items
    .filter((stage) => !usedIds.has(stage.id))
    .flatMap((stage) => {
      const latest = latestPublished(stage.versions);
      return sortVersions(stage.versions.filter((v) => v.status === "published")).map((v) => ({
        value: v.id,
        label: `${stage.name} v${String(v.versionNo)}${v.id === latest?.id ? " (mới nhất)" : ""}`,
      }));
    });
}

/** Id of the published version `versionNo` of `stageId` (the target of "Dùng v{n}"), if the list has it. */
export function publishedVersionId(list: StageList, stageId: string, versionNo: number): string | undefined {
  const stage = list.items.find((s) => s.id === stageId);
  return stage?.versions.find((v) => v.versionNo === versionNo && v.status === "published")?.id;
}

/** Stage version ids of `stages` with the one at `index` replaced by `replacement`. */
export function replaceAt(stages: readonly CourseStage[], index: number, replacement: string): string[] {
  return stages.map((s, i) => (i === index ? replacement : s.stageVersionId));
}

/** Stage version ids of `stages` without the one at `index`. */
export function removeAt(stages: readonly CourseStage[], index: number): string[] {
  return stages.filter((_s, i) => i !== index).map((s) => s.stageVersionId);
}
