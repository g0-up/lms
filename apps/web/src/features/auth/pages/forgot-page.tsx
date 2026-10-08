import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { hasCode } from "@/shared/api/errors";
import { Alert } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Field } from "@/shared/ui/field";
import { Input } from "@/shared/ui/input";
import { authApi } from "../api/auth-api";
import { AuthHeading } from "../components/auth-heading";
import { AuthLink } from "../components/auth-link";
import { FormActions } from "../components/form-actions";
import { forgotFormSchema, type ForgotFormValues } from "../model/schemas";

const SENT_MESSAGE =
  "Nếu email tồn tại trong hệ thống, chúng tôi đã gửi đường dẫn đặt lại mật khẩu. Vui lòng kiểm tra hộp thư.";
const RATE_LIMITED_MESSAGE = "Bạn gửi quá nhiều yêu cầu. Thử lại sau ít phút.";

export function Component() {
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<ForgotFormValues, unknown, ForgotFormValues>({
    resolver: zodResolver(forgotFormSchema),
    defaultValues: { email: "" },
  });
  const forgot = useMutation({
    mutationFn: authApi.forgotPassword,
    meta: { silent: true },
    // The answer never reveals whether the account exists: every accepted request shows the same notice.
    onSuccess: () => {
      reset({ email: "" });
    },
  });

  const onSubmit = handleSubmit((values) => {
    forgot.mutate(values);
  });

  const errorMessage = forgot.error
    ? hasCode(forgot.error, "RATE_LIMITED")
      ? RATE_LIMITED_MESSAGE
      : forgot.error.message
    : null;

  return (
    <>
      <AuthHeading title="Quên mật khẩu">
        Nhập email đăng nhập. Nếu email tồn tại, bạn sẽ nhận được đường dẫn đặt lại mật khẩu, hiệu lực 30 phút.
      </AuthHeading>
      <form noValidate onSubmit={(event) => void onSubmit(event)} className="grid gap-3">
        <Field label="Email" required error={errors.email?.message}>
          <Input type="email" autoComplete="username" required {...register("email")} />
        </Field>
        {forgot.isSuccess ? (
          <Alert variant="ok" role="status">
            {SENT_MESSAGE}
          </Alert>
        ) : null}
        {errorMessage ? (
          <Alert variant="danger" role="alert">
            {errorMessage}
          </Alert>
        ) : null}
        <FormActions>
          <AuthLink to="/login">Quay lại đăng nhập</AuthLink>
          <Button type="submit" variant="gradient" disabled={forgot.isPending}>
            Gửi đường dẫn
          </Button>
        </FormActions>
      </form>
    </>
  );
}
