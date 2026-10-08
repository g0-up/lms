import type { FieldErrors, UseFormRegister } from "react-hook-form";
import { Field } from "@/shared/ui/field";
import { Input } from "@/shared/ui/input";
import type { NewPasswordFormValues } from "../model/schemas";

interface NewPasswordFieldsProps {
  register: UseFormRegister<NewPasswordFormValues>;
  errors: FieldErrors<NewPasswordFormValues>;
}

/** "Mật khẩu mới" + "Nhập lại mật khẩu mới", shared by first login and password reset. */
export function NewPasswordFields({ register, errors }: NewPasswordFieldsProps) {
  return (
    <>
      <Field label="Mật khẩu mới" required help="Tối thiểu 8 ký tự." error={errors.newPassword?.message}>
        <Input type="password" autoComplete="new-password" minLength={8} required {...register("newPassword")} />
      </Field>
      <Field label="Nhập lại mật khẩu mới" required error={errors.confirmPassword?.message}>
        <Input type="password" autoComplete="new-password" required {...register("confirmPassword")} />
      </Field>
    </>
  );
}
