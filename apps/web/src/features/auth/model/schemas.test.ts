import { describe, expect, it } from "vitest";
import { forgotFormSchema, loginFormSchema, newPasswordFormSchema } from "./schemas";

function messages(result: { success: boolean; error?: { issues: { path: PropertyKey[]; message: string }[] } }) {
  return Object.fromEntries((result.error?.issues ?? []).map((i) => [i.path.join("."), i.message]));
}

describe("auth form schemas", () => {
  it("trims the email and requires both login fields", () => {
    expect(loginFormSchema.parse({ email: "  quan.tran@goup.vn ", password: "x" }).email).toBe("quan.tran@goup.vn");
    expect(messages(loginFormSchema.safeParse({ email: "", password: "" }))).toEqual({
      email: "Nhập email đăng nhập.",
      password: "Nhập mật khẩu.",
    });
  });

  it("rejects a malformed email", () => {
    expect(messages(forgotFormSchema.safeParse({ email: "quan.tran" }))).toEqual({ email: "Email không hợp lệ." });
  });

  it("requires 8 characters and a matching confirmation", () => {
    expect(messages(newPasswordFormSchema.safeParse({ newPassword: "short", confirmPassword: "short" }))).toEqual({
      newPassword: "Mật khẩu mới cần tối thiểu 8 ký tự.",
    });
    expect(messages(newPasswordFormSchema.safeParse({ newPassword: "longenough", confirmPassword: "different" }))).toEqual({
      confirmPassword: "Hai mật khẩu không khớp.",
    });
    expect(newPasswordFormSchema.safeParse({ newPassword: "longenough", confirmPassword: "longenough" }).success).toBe(true);
  });
});
