import { describe, expect, it } from "vitest";
import { DEADLINE_MESSAGES, deadlineDefaults, deadlineFormSchema } from "./deadline-form";

const errorsOf = (value: { dueAt: string; clear: boolean }) => {
  const result = deadlineFormSchema.safeParse(value);
  return result.success ? {} : Object.fromEntries(result.error.issues.map((i) => [i.path.join("."), i.message]));
};

describe("deadline form", () => {
  it("starts from the current deadline in Vietnam time, or blank", () => {
    expect(deadlineDefaults("2026-10-05T08:00:00Z")).toEqual({ dueAt: "2026-10-05T15:00", clear: false });
    expect(deadlineDefaults(null)).toEqual({ dueAt: "", clear: false });
  });

  it("asks for a date and time unless the deadline is being removed", () => {
    expect(errorsOf({ dueAt: " ", clear: false })).toEqual({ dueAt: DEADLINE_MESSAGES.required });
    expect(errorsOf({ dueAt: "2026-02-30T10:00", clear: false })).toEqual({ dueAt: "Hạn nộp không hợp lệ." });
    expect(errorsOf({ dueAt: "", clear: true })).toEqual({});
  });

  it("sends the time with the +07:00 offset, or null to remove the deadline", () => {
    expect(deadlineFormSchema.parse({ dueAt: "2026-10-20T23:59", clear: false })).toEqual({
      dueAt: "2026-10-20T23:59:00+07:00",
    });
    expect(deadlineFormSchema.parse({ dueAt: "2026-10-20T23:59", clear: true })).toEqual({ dueAt: null });
  });
});
