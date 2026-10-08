import { describe, expect, it } from "vitest";
import { ApiError } from "@/shared/api/errors";
import { errors } from "../test/golden";
import { inviteErrorMessage, inviteFormSchema } from "./invite-form";

const apiError = (status: number, envelope: { error: { code: string; message: string; details?: unknown } }) =>
  new ApiError(status, envelope.error.code, envelope.error.message, envelope.error.details);

describe("inviteFormSchema", () => {
  it("normalizes the email and drops a blank name", () => {
    expect(inviteFormSchema.parse({ email: "  Moi.Hoc.Vien@Gmail.COM ", fullName: "   " })).toEqual({
      email: "moi.hoc.vien@gmail.com",
      fullName: undefined,
    });
  });

  it("keeps a trimmed name", () => {
    expect(inviteFormSchema.parse({ email: "a@b.vn", fullName: " Học Viên Mới " }).fullName).toBe("Học Viên Mới");
  });

  it.each(["", "abc", "a@b", "a b@c.vn"])("rejects the email %j", (email) => {
    const result = inviteFormSchema.safeParse({ email, fullName: "" });
    expect(result.error?.issues[0]?.message).toBe("Email không hợp lệ.");
  });
});

describe("inviteErrorMessage", () => {
  it.each([
    ["422 name required", apiError(422, errors.nameRequired), "Nhập họ tên học viên."],
    [
      "422 with a fullName field",
      new ApiError(422, "VALIDATION_FAILED", "Dữ liệu không hợp lệ", { field: "fullName" }),
      "Nhập họ tên học viên.",
    ],
    [
      "422 on the email",
      new ApiError(422, "VALIDATION_FAILED", "Dữ liệu không hợp lệ", { email: "Email không hợp lệ" }),
      "Email không hợp lệ.",
    ],
    ["409 INVALID_TRANSITION", apiError(409, errors.invalidTransition), "Không mời được vào lớp đã kết thúc."],
    ["403 FORBIDDEN", apiError(403, errors.internalEmail), "Email này thuộc tài khoản nội bộ, không mời làm học viên được."],
    ["409 ACCOUNT_DISABLED", apiError(409, errors.accountDisabled), "Tài khoản đã bị vô hiệu hóa. Kích hoạt lại trước khi mời."],
    ["409 CONFLICT", apiError(409, errors.alreadyMember), "Học viên đã có trong lớp."],
    ["429 RATE_LIMITED", apiError(429, errors.rateLimited), "Đã gửi lại quá nhiều lần. Thử lại sau."],
  ])("maps %s", (_name, error, expected) => {
    expect(inviteErrorMessage(error)).toBe(expected);
  });

  it("leaves network failures to the dialog", () => {
    expect(inviteErrorMessage(new TypeError("fetch failed"))).toBeNull();
  });
});
