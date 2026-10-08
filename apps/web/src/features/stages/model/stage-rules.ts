import type { StageVersion } from "./schemas";

export const PUBLISH_NEEDS_LESSON = "Cần ít nhất một học liệu";
export const VERSION_IMMUTABLE_MESSAGE = "Phiên bản đã phát hành, không sửa được.";
export const REORDER_FAILED_MESSAGE = "Không đổi được thứ tự.";

/** Why the draft cannot be published yet (button title); `null` when it can. */
export function stagePublishBlocker(version: Pick<StageVersion, "status" | "lessons">): string | null {
  return version.status === "draft" && version.lessons.length === 0 ? PUBLISH_NEEDS_LESSON : null;
}

/** Why a published/archived version cannot be deleted; `null` when no course version references it. */
export function stageDeleteBlocker(version: Pick<StageVersion, "status" | "usedBy">): string | null {
  if (version.status === "draft" || version.usedBy.length === 0) return null;
  const refs = version.usedBy.map((u) => `${u.courseName} v${String(u.versionNo)}`).join(", ");
  return `Đang được dùng trong ${refs}. Hãy lưu trữ thay vì xóa.`;
}

/** Toast for DRAFT_EXISTS on clone, built from the error's `details.draftVersionNo`. */
export function draftExistsMessage(draftVersionNo: number): string {
  return `Chặng đã có bản nháp v${String(draftVersionNo)}. Mở bản nháp đó để sửa.`;
}
