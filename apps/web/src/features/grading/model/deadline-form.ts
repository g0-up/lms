import { z } from "zod";
import { fromLocalInput, toLocalInput } from "./deadline-time";

export const DEADLINE_MESSAGES = {
  required: "Chọn ngày giờ hạn nộp.",
  invalid: "Hạn nộp không hợp lệ.",
};

/** `dueAt` is the `datetime-local` value in Vietnam time; `clear` removes the deadline whatever `dueAt` holds. */
export interface DeadlineFormInput {
  dueAt: string;
  clear: boolean;
}

/** RFC 3339 deadline to send, or null to remove it. */
export interface DeadlineBody {
  dueAt: string | null;
}

export const deadlineDefaults = (deadline: string | null): DeadlineFormInput => ({
  dueAt: toLocalInput(deadline),
  clear: false,
});

export const deadlineFormSchema = z
  .object({ dueAt: z.string(), clear: z.boolean() })
  .superRefine((value, ctx) => {
    if (value.clear) return;
    if (!value.dueAt.trim()) ctx.addIssue({ code: "custom", path: ["dueAt"], message: DEADLINE_MESSAGES.required });
    else if (!fromLocalInput(value.dueAt)) {
      ctx.addIssue({ code: "custom", path: ["dueAt"], message: DEADLINE_MESSAGES.invalid });
    }
  })
  .transform((value): DeadlineBody => ({ dueAt: value.clear ? null : fromLocalInput(value.dueAt) }));
