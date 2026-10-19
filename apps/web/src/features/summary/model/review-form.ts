import { isApiError } from "@/shared/api/errors";

export const MAX_REVIEW = 5000;

/** Copy of the review form's errors, word for word the API's. */
export const STAGE_REVIEW_MESSAGES = {
  required: "Nhập nhận xét tổng kết chặng.",
  tooLong: `Nhận xét tối đa ${String(MAX_REVIEW)} ký tự.`,
} as const;

/** The API's checks on a stage review (trimmed, counted in code points as it counts runes); null when valid. */
export function reviewProblem(body: string): string | null {
  const text = body.trim();
  if (text === "") return STAGE_REVIEW_MESSAGES.required;
  if (Array.from(text).length > MAX_REVIEW) return STAGE_REVIEW_MESSAGES.tooLong;
  return null;
}

/** Where an API refusal of the review belongs: the field for a 422, the form for anything else. */
export function reviewErrorField(error: unknown): "body" | "form" {
  return isApiError(error) && error.status === 422 ? "body" : "form";
}

/** "đã sửa 2 lần" once the review has been saved more than once, else null. */
export function editsLabel(revision: number): string | null {
  return revision > 1 ? `đã sửa ${String(revision - 1)} lần` : null;
}
