import type { AssessmentScore, HomeworkScore } from "./schemas";

const HOMEWORK_LABEL: Record<HomeworkScore["status"], string> = {
  passed: "Đạt",
  rework: "Cần làm lại",
  pending: "Chờ chấm",
  not_submitted: "Chưa nộp",
};

/** An assessment in the course table: "Tốt nhất 80% · Đạt", or "Chưa làm" before any attempt. */
export function assessmentScoreText(score: AssessmentScore): string {
  if (score.bestPercent === null) return "Chưa làm";
  return `Tốt nhất ${String(score.bestPercent)}% · ${score.passed ? "Đạt" : "Chưa đạt"}`;
}

/** A homework in the course table: "94/100 · Đạt" once graded, the state alone before. */
export function homeworkScoreText(score: HomeworkScore): string {
  const label = HOMEWORK_LABEL[score.status];
  return score.total === null || score.max === null ? label : `${String(score.total)}/${String(score.max)} · ${label}`;
}

/** What a stage still misses in the course table: "còn 2 học liệu", or nothing once none is missing. */
export function missingText(missingCount: number): string | null {
  return missingCount === 0 ? null : `còn ${String(missingCount)} học liệu`;
}
