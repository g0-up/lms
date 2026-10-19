import { lacksCorrectAnswer } from "./question-form";
import type { Lesson, StageVersion } from "./schemas";

export const PUBLISH_NEEDS_UNIT = "Chặng cần ít nhất một bài học trước khi phát hành.";
export const VERSION_IMMUTABLE_MESSAGE = "Phiên bản đã phát hành, không sửa được.";
export const REORDER_FAILED_MESSAGE = "Không đổi được thứ tự.";

export function emptyUnitMessage(title: string): string {
  return `Bài học "${title}" chưa có học liệu.`;
}

/**
 * What a quiz or homework still lacks before its stage can be published, in the server's words: a quiz
 * needs questions and a correct answer on each, a homework a brief and at least one criterion. Other
 * types have nothing to add here.
 */
export function lessonPublishBlockers(lesson: Pick<Lesson, "title" | "type" | "quiz" | "homework">): string[] {
  const title = `"${lesson.title}"`;
  if (lesson.type === "quiz") {
    const questions = lesson.quiz?.questions ?? [];
    if (questions.length === 0) return [`${title} chưa có câu hỏi.`];
    const unanswered = questions.filter(lacksCorrectAnswer).length;
    return unanswered > 0 ? [`${title} có ${String(unanswered)} câu chưa đánh dấu đáp án đúng.`] : [];
  }
  if (lesson.type === "homework") {
    const blockers: string[] = [];
    if ((lesson.homework?.briefSource ?? "").trim() === "") blockers.push(`${title} chưa có đề bài.`);
    if ((lesson.homework?.criteria ?? []).length === 0) blockers.push(`${title} chưa có tiêu chí chấm.`);
    return blockers;
  }
  return [];
}

/**
 * Why the draft cannot be published yet, in the server's words and order: no unit at all, else every
 * unit without a lesson, then what each quiz or homework lacks in lesson order. Empty when it can be
 * published, and always for a published or archived version.
 */
export function stagePublishBlockers(version: Pick<StageVersion, "status" | "units" | "lessons">): string[] {
  if (version.status !== "draft") return [];
  if (version.units.length === 0) return [PUBLISH_NEEDS_UNIT];
  return [
    ...version.units.filter((u) => u.lessonCount === 0).map((u) => emptyUnitMessage(u.title)),
    ...version.lessons.flatMap(lessonPublishBlockers),
  ];
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
