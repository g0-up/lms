import type { QueueKind } from "./schemas";

/** Area of the grader: `/admin` or `/teach`. */
export type GradingBase = "/admin" | "/teach";

export const queueHref = (base: GradingBase) => `${base}/grading`;

/** Grading page of a homework submission. */
export const gradeHref = (base: GradingBase, submissionId: string) =>
  `${queueHref(base)}/${encodeURIComponent(submissionId)}`;

/** Page of a queue item: the grading page of a submission or the review page of a quiz attempt. */
export const itemHref = (base: GradingBase, kind: QueueKind, itemId: string) =>
  kind === "quiz" ? `${queueHref(base)}/quiz/${encodeURIComponent(itemId)}` : gradeHref(base, itemId);

/** Where a quiz review page was opened from: the queue (default) or the class's quiz tab. */
export type ReviewFrom = "queue" | "quiz";

/** `?from=` of the review page; anything else is the queue. */
export const parseReviewFrom = (value: string | null): ReviewFrom => (value === "quiz" ? "quiz" : "queue");

/** Review page of a quiz attempt; opened from the class's quiz tab, its way back leads there. */
export const quizReviewHref = (base: GradingBase, attemptId: string, from: ReviewFrom = "queue") =>
  `${itemHref(base, "quiz", attemptId)}${from === "quiz" ? "?from=quiz" : ""}`;

/** Class page of the area, on its quiz tab. */
export const classQuizHref = (base: GradingBase, classId: string) =>
  `${base}/classes/${encodeURIComponent(classId)}?tab=quiz`;

/** Class list of the area: every class for an admin, the teacher's own classes on their home. */
export const classesHref = (base: GradingBase) => (base === "/admin" ? "/admin/classes" : "/teach");
