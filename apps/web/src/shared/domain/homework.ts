/** Status of one homework submission, as the API reports it. */
export type SubmissionStatus = "pending" | "passed" | "rework" | "superseded";

export const SUBMISSION_STATUS_VI: Record<SubmissionStatus, string> = {
  pending: "Chờ chấm",
  passed: "Đạt",
  rework: "Cần làm lại",
  superseded: "Đã thay bằng lần nộp sau",
};

/** A submission waiting longer than this many days is overdue for grading (the API's 72h). */
export const GRADING_SLA_DAYS = 3;

/** Least total that passes: the API's rule `total * 100 >= passPercent * maxPoints`. */
export function passPoints(maxPoints: number, passPercent: number): number {
  return Math.ceil((maxPoints * passPercent) / 100);
}

export function isPassing(total: number, maxPoints: number, passPercent: number): boolean {
  return total * 100 >= passPercent * maxPoints;
}

/** Lesson title without its "Bài tập:" style prefix, for crumbs and table cells. */
export function shortTitle(title: string): string {
  return title.replace(/^(Bài tập|Luyện tập|Kiểm tra):\s*/, "");
}
