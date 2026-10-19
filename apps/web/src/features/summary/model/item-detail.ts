import { quizReviewBadge } from "@/shared/domain";
import { dueIn, fmtDateTime } from "@/shared/lib/format";
import type { HomeworkFacts, QuizFacts, SummaryItem } from "./schemas";

function quizDetail(quiz: QuizFacts): string {
  const limit = quiz.limit === null ? "" : `/${String(quiz.limit)}`;
  if (quiz.submitted === 0 || quiz.best === null) {
    const start = quiz.open ? "Đang làm" : "Chưa làm";
    return quiz.limit === null ? start : `${start} · ${String(quiz.limit)} lượt`;
  }
  // An open attempt already uses up one of a limited quiz's attempts.
  const used = quiz.limit === null ? quiz.submitted : quiz.attemptsUsed;
  const parts = [
    `Tốt nhất ${String(quiz.best.percent)}% (${String(quiz.best.score)}/${String(quiz.best.max)})`,
    `${String(used)}${limit} lượt`,
  ];
  const { latest } = quiz;
  if (quiz.mode === "practice" && latest) {
    parts.push(`Gần nhất ${String(latest.percent)}% (${String(latest.score)}/${String(latest.max)})`);
  }
  if (quiz.mode === "assessment" && latest?.reviewStatus) {
    parts.push(quizReviewBadge(quiz.mode, latest.reviewStatus).label);
  }
  return parts.join(" · ");
}

function homeworkDetail(homework: HomeworkFacts, now: Date): string {
  const { latest, dueAt } = homework;
  if (latest) {
    const parts = [`Lần nộp #${String(latest.attemptNo)}`, fmtDateTime(latest.submittedAt)];
    if (latest.isLate) parts.push("nộp muộn");
    if (latest.total !== null && latest.max !== null) parts.push(`${String(latest.total)}/${String(latest.max)} điểm`);
    return parts.join(" · ");
  }
  return dueAt ? `Hạn ${fmtDateTime(dueAt)} · ${dueIn(dueAt, now)}` : "Chưa nộp · chưa đặt hạn";
}

/**
 * The detail line of one lesson in a stage summary: best score and attempts of a quiz (with the latest score of a
 * practice quiz and the review state of an assessment's latest attempt), the latest submission or the deadline of a
 * homework, and when a video or reading was ticked or first opened. Falls back to the API's own sentence when the
 * facts of a quiz or homework are missing.
 */
export function itemDetail(item: SummaryItem, now: Date = new Date()): string {
  if (item.type === "quiz") return item.quiz ? quizDetail(item.quiz) : item.status.detail;
  if (item.type === "homework") return item.homework ? homeworkDetail(item.homework, now) : item.status.detail;
  if (item.completedAt) return `Tích hoàn thành ${fmtDateTime(item.completedAt)}`;
  if (item.firstOpenedAt) return `Mở lần đầu ${fmtDateTime(item.firstOpenedAt)}`;
  return "Chưa mở";
}
