import { describe, expect, it } from "vitest";
import { GRANT_MESSAGES, grantDefaults, grantFormSchema, grantedMessage, type GrantFormInput } from "./grant-form";

const errorsOf = (value: GrantFormInput) => {
  const result = grantFormSchema.safeParse(value);
  return result.success ? {} : Object.fromEntries(result.error.issues.map((i) => [i.path.join("."), i.message]));
};

describe("grant form", () => {
  it("starts on one attempt and no reason", () => {
    expect(grantDefaults).toEqual({ extraAttempts: "1", reason: "" });
  });

  it("sends the number of attempts and the trimmed reason", () => {
    expect(grantFormSchema.parse({ extraAttempts: "3", reason: "  Mất kết nối khi đang làm bài \n" })).toEqual({
      extraAttempts: 3,
      reason: "Mất kết nối khi đang làm bài",
    });
  });

  it("requires a reason, blank or whitespace alike", () => {
    expect(errorsOf({ extraAttempts: "1", reason: "" })).toEqual({ reason: GRANT_MESSAGES.reasonRequired });
    expect(errorsOf({ extraAttempts: "1", reason: " \t\n " })).toEqual({ reason: GRANT_MESSAGES.reasonRequired });
  });

  it("counts the reason in code points, up to 500", () => {
    expect(errorsOf({ extraAttempts: "2", reason: "ữ".repeat(500) })).toEqual({});
    expect(errorsOf({ extraAttempts: "2", reason: "ữ".repeat(501) })).toEqual({ reason: GRANT_MESSAGES.reasonTooLong });
    expect(GRANT_MESSAGES.reasonTooLong).toBe("Lý do tối đa 500 ký tự.");
  });

  it("only takes one to three attempts", () => {
    expect(grantFormSchema.safeParse({ extraAttempts: "4", reason: "Lỗi đề" }).success).toBe(false);
  });

  it("toasts how many attempts went to whom", () => {
    expect(grantedMessage(2, "Vũ Đức Khang")).toBe("Đã cấp thêm 2 lượt cho Vũ Đức Khang.");
  });
});
