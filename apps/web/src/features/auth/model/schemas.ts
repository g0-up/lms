import { z } from "zod";

/** Same shape check the API uses before looking an email up; the server stays the authority. */
const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export const MIN_PASSWORD_LENGTH = 8;

const emailField = z
  .string()
  .trim()
  // abort: an empty field reports only "required", not also "invalid".
  .min(1, { message: "Nhập email đăng nhập.", abort: true })
  .regex(EMAIL_RE, "Email không hợp lệ.");

export const loginFormSchema = z.object({
  email: emailField,
  password: z.string().min(1, "Nhập mật khẩu."),
});
export type LoginFormValues = z.input<typeof loginFormSchema>;

export const forgotFormSchema = z.object({ email: emailField });
export type ForgotFormValues = z.input<typeof forgotFormSchema>;

/** New password + confirmation, shared by first login and reset. Mirrors the server's checks. */
export const newPasswordFormSchema = z
  .object({
    newPassword: z.string().min(MIN_PASSWORD_LENGTH, "Mật khẩu mới cần tối thiểu 8 ký tự."),
    confirmPassword: z.string(),
  })
  .refine((v) => v.newPassword === v.confirmPassword, {
    message: "Hai mật khẩu không khớp.",
    path: ["confirmPassword"],
  });
export type NewPasswordFormValues = z.input<typeof newPasswordFormSchema>;
