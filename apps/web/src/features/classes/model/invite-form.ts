import { z } from "zod";
import { isApiError } from "@/shared/api/errors";

export const INVALID_EMAIL = "Email không hợp lệ.";
export const NAME_REQUIRED = "Nhập họ tên học viên.";
export const ENDED_CLASS = "Không mời được vào lớp đã kết thúc.";

/** Same shape check the API uses; the server stays the authority. */
const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/** Email trimmed and lowercased before matching (the API normalizes again); a blank name is left out. */
export const inviteFormSchema = z.object({
  email: z.string().trim().toLowerCase().regex(EMAIL_RE, INVALID_EMAIL),
  fullName: z
    .string()
    .trim()
    .transform((name) => (name === "" ? undefined : name)),
});
export type InviteFormInput = z.input<typeof inviteFormSchema>;
export type InviteFormValues = z.output<typeof inviteFormSchema>;

function namesFullName(details: unknown): boolean {
  if (typeof details !== "object" || details === null) return false;
  return (details as { field?: unknown }).field === "fullName" || "fullName" in details;
}

/** Copy for a refused invitation, mapped from the API error code. */
export function inviteErrorMessage(error: unknown): string | null {
  if (!isApiError(error)) return null;
  switch (error.code) {
    case "VALIDATION_FAILED":
      return namesFullName(error.details) || error.message === NAME_REQUIRED ? NAME_REQUIRED : INVALID_EMAIL;
    case "INVALID_TRANSITION":
      return ENDED_CLASS;
    case "FORBIDDEN":
      return "Email này thuộc tài khoản nội bộ, không mời làm học viên được.";
    case "ACCOUNT_DISABLED":
      return "Tài khoản đã bị vô hiệu hóa. Kích hoạt lại trước khi mời.";
    case "CONFLICT":
      return "Học viên đã có trong lớp.";
    default:
      return error.message;
  }
}
