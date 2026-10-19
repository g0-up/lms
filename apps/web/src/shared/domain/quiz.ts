/** Quiz mode, as the API reports it. */
export type QuizMode = "practice" | "assessment";

/** Teacher review state of a submitted quiz attempt; practice attempts only ever get `confirmed`. */
export type QuizReviewStatus = "pending" | "confirmed" | "superseded";

export const QUIZ_KIND_VI: Record<QuizMode, string> = {
  practice: "Quiz học liệu",
  assessment: "Quiz bài tập",
};

/** Badge for a review state: `info`, `muted` and `ok` match the badge variants of the same name. */
export interface QuizReviewBadge {
  tone: "info" | "muted" | "ok";
  label: string;
}

/** The prototype's label of a review state; a confirmed practice attempt was commented, not confirmed. */
export function quizReviewBadge(mode: QuizMode, status: QuizReviewStatus): QuizReviewBadge {
  switch (status) {
    case "pending":
      return { tone: "info", label: "Chờ xác nhận" };
    case "superseded":
      return { tone: "muted", label: "Đã có lượt mới hơn" };
    case "confirmed":
      return { tone: "ok", label: mode === "assessment" ? "Đã xác nhận" : "Đã nhận xét" };
  }
}
