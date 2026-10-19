import { z } from "zod";

export const GRANT_ATTEMPT_CHOICES = ["1", "2", "3"] as const;
const MAX_REASON = 500;

/** Length in code points, as the API counts runes. */
const runeCount = (text: string) => Array.from(text).length;

/** Copy of the grant form's errors, word for word the API's. */
export const GRANT_MESSAGES = {
  reasonRequired: "Nhập lý do cấp thêm lượt. Lý do được lưu vào lịch sử.",
  reasonTooLong: `Lý do tối đa ${String(MAX_REASON)} ký tự.`,
} as const;

/** Form fields: the select's value ("1"–"3") and the free-text reason. */
export interface GrantFormInput {
  extraAttempts: (typeof GRANT_ATTEMPT_CHOICES)[number];
  reason: string;
}

/** Body of `POST /classes/{id}/members/{mid}/grants`. */
export interface GrantBody {
  lessonId: string;
  extraAttempts: number;
  reason: string;
}

export const grantDefaults: GrantFormInput = { extraAttempts: "1", reason: "" };

/** The API's checks for the grant form; the reason is sent trimmed. */
export const grantFormSchema = z
  .object({ extraAttempts: z.enum(GRANT_ATTEMPT_CHOICES), reason: z.string().trim() })
  .superRefine((v, ctx) => {
    if (v.reason === "") ctx.addIssue({ code: "custom", path: ["reason"], message: GRANT_MESSAGES.reasonRequired });
    else if (runeCount(v.reason) > MAX_REASON) {
      ctx.addIssue({ code: "custom", path: ["reason"], message: GRANT_MESSAGES.reasonTooLong });
    }
  })
  .transform((v) => ({ extraAttempts: Number(v.extraAttempts), reason: v.reason }));

/** The toast once the grant is saved. */
export const grantedMessage = (extraAttempts: number, name: string) =>
  `Đã cấp thêm ${String(extraAttempts)} lượt cho ${name}.`;
