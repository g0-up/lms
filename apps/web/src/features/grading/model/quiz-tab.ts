import type { QuizMode } from "@/shared/domain";
import type { MemberQuiz } from "./quiz-schemas";

export interface MemberQuizBadge {
  tone: "muted" | "info" | "ok" | "warn" | "danger";
  label: string;
}

const best = (m: MemberQuiz) => (m.bestPercent === null ? "" : ` ${String(m.bestPercent)}%`);

/** The prototype's status label of a member against one quiz; an assessment shows the best attempt's score. */
export function memberQuizBadge(member: MemberQuiz): MemberQuizBadge {
  switch (member.status) {
    case "todo":
      return { tone: "muted", label: "Chưa làm" };
    case "inProgress":
      return { tone: "info", label: "Đang làm" };
    case "done":
      return { tone: "ok", label: "Đã làm" };
    case "passed":
      return { tone: "ok", label: `Đạt${best(member)}` };
    case "failed":
      return { tone: "warn", label: `Chưa đạt${best(member)}` };
    case "outOfAttempts":
      return { tone: "danger", label: `Chưa đạt${best(member)} · hết lượt` };
  }
}

export interface MemberQuizAction {
  label: string;
  /** Accessible name of the link, without the member and attempt. */
  verb: string;
  /** The attempt waits for confirmation: the primary action of the row. */
  primary: boolean;
}

/**
 * What the teacher does with a member's latest attempt: confirm it while it waits, else look at it (and comment,
 * for practice). Null without a submitted attempt.
 */
export function memberQuizAction(member: MemberQuiz, mode: QuizMode): MemberQuizAction | null {
  const { latest } = member;
  if (!latest) return null;
  if (latest.reviewStatus === "pending") return { label: "Xác nhận", verb: "Xác nhận", primary: true };
  return mode === "assessment"
    ? { label: "Xem", verb: "Xem", primary: false }
    : { label: "Xem, nhận xét", verb: "Xem và nhận xét", primary: false };
}

/** Below this share of right answers a question is flagged for the teacher to look at again. */
export const LOW_CORRECT_PERCENT = 30;

export const isLowCorrect = (percent: number | null): boolean => percent !== null && percent < LOW_CORRECT_PERCENT;

/** Extra attempts are for an assessment the member has not passed yet; the API would accept more, the UI does not offer it. */
export const canGrantAttempts = (member: MemberQuiz, mode: QuizMode): boolean =>
  mode === "assessment" && member.status !== "passed";
