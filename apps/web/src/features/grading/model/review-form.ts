import { z } from "zod";
import { isApiError } from "@/shared/api/errors";
import { GRADE_MESSAGES, MAX_COMMENT } from "./grade-form";
import type { AttemptDetail, QuizReview, ReviewOutcome } from "./quiz-schemas";

const MAX_NOTE = 1000;

/** Length in code points, as the API counts runes. */
const runeCount = (text: string) => Array.from(text).length;

/** Copy of the review form's errors, word for word the API's. */
export const REVIEW_MESSAGES = {
  commentRequired: GRADE_MESSAGES.commentRequired,
  commentTooLong: GRADE_MESSAGES.commentTooLong,
  noteTooLong: `Ghi chú từng câu tối đa ${String(MAX_NOTE)} ký tự.`,
} as const;

/** Form fields: the general comment and one note per question, in the order the student saw them. */
export interface ReviewFormInput {
  comment: string;
  notes: { text: string }[];
}

/** Body of `POST /quiz-attempts/{aid}/reviews`. */
export interface ReviewBody {
  comment: string;
  notes: { questionId: string; note: string }[];
}

/** Fields prefilled from the current review, blank before the first one. */
export function reviewDefaults(detail: Pick<AttemptDetail, "questions">, current: QuizReview | null): ReviewFormInput {
  return {
    comment: current?.comment ?? "",
    notes: detail.questions.map((q) => ({ text: current?.notes.find((n) => n.questionId === q.id)?.note ?? "" })),
  };
}

/** The API's checks for the review form, mapped to the request body; blank notes are left out. */
export function reviewFormSchema(questionIds: readonly string[]) {
  return z
    .object({
      comment: z.string().trim(),
      notes: z.array(z.object({ text: z.string().trim() })).length(questionIds.length),
    })
    .superRefine((v, ctx) => {
      if (v.comment === "") {
        ctx.addIssue({ code: "custom", path: ["comment"], message: REVIEW_MESSAGES.commentRequired });
      } else if (runeCount(v.comment) > MAX_COMMENT) {
        ctx.addIssue({ code: "custom", path: ["comment"], message: REVIEW_MESSAGES.commentTooLong });
      }
      v.notes.forEach((n, i) => {
        if (runeCount(n.text) > MAX_NOTE) {
          ctx.addIssue({ code: "custom", path: ["notes", i, "text"], message: REVIEW_MESSAGES.noteTooLong });
        }
      });
    })
    .transform(
      (v): ReviewBody => ({
        comment: v.comment,
        notes: questionIds.flatMap((questionId, i) => {
          const note = v.notes[i].text;
          return note ? [{ questionId, note }] : [];
        }),
      }),
    );
}

/** Where an API refusal of the review belongs: the general comment, or the form as a whole. */
export function reviewErrorField(error: unknown): "comment" | "form" {
  if (!isApiError(error) || error.status !== 400) return "form";
  return error.message === REVIEW_MESSAGES.commentRequired || error.message === REVIEW_MESSAGES.commentTooLong
    ? "comment"
    : "form";
}

/** Title and submit label of the form: confirming an assessment, commenting a practice attempt, or editing. */
export function reviewFormCopy(mode: AttemptDetail["mode"], editing: boolean): { title: string; submit: string } {
  if (editing) return { title: "Sửa nhận xét", submit: "Lưu nhận xét mới" };
  return mode === "assessment"
    ? { title: "Xác nhận kết quả", submit: "Xác nhận" }
    : { title: "Nhận xét cho học viên", submit: "Gửi nhận xét" };
}

/** Success message of a saved review, before what happens next. */
export function reviewedMessage(mode: AttemptDetail["mode"], outcome: Pick<ReviewOutcome, "updated">): string {
  if (outcome.updated) return "Đã lưu nhận xét mới. Học viên nhận email cập nhật.";
  return mode === "assessment"
    ? "Đã xác nhận kết quả. Email nhận xét đã gửi cho học viên."
    : "Đã gửi nhận xét cho học viên.";
}
