import { zodResolver } from "@hookform/resolvers/zod";
import { useForm } from "react-hook-form";
import { useNavigate, useSearchParams } from "react-router";
import { homeOf } from "@/shared/domain";
import { Alert } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Field } from "@/shared/ui/field";
import { Input } from "@/shared/ui/input";
import { PendingLabel } from "@/shared/ui/pending-label";
import { AuthHeading } from "../components/auth-heading";
import { AuthLink } from "../components/auth-link";
import { FormActions } from "../components/form-actions";
import { useLogin } from "../hooks/use-login";
import { FIRST_LOGIN_PATH, safeNext } from "../model/navigation";
import { loginFormSchema, type LoginFormValues } from "../model/schemas";

const TAGLINE = "Nền tảng học nội bộ của GoUp: chặng → khóa học có phiên bản → lớp học → tiến độ học viên.";

export function Component() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const login = useLogin();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginFormValues, unknown, LoginFormValues>({
    resolver: zodResolver(loginFormSchema),
    defaultValues: { email: "", password: "" },
  });

  const onSubmit = handleSubmit((values) => {
    login.mutate(values, {
      onSuccess: (user) => {
        const to = user.mustChangePassword
          ? FIRST_LOGIN_PATH
          : (safeNext(searchParams.get("next")) ?? homeOf(user.role));
        void navigate(to, { replace: true });
      },
    });
  });

  return (
    <>
      <p className="-mt-4 mb-6 max-w-[40ch] text-sm text-ink-3">{TAGLINE}</p>
      <AuthHeading title="Đăng nhập">Dùng email và mật khẩu trong thư mời của bạn.</AuthHeading>
      <form noValidate onSubmit={(event) => void onSubmit(event)} className="grid gap-3">
        <Field label="Email" required error={errors.email?.message}>
          <Input type="email" autoComplete="username" required {...register("email")} />
        </Field>
        <Field label="Mật khẩu" required error={errors.password?.message}>
          <Input type="password" autoComplete="current-password" required {...register("password")} />
        </Field>
        {login.error ? (
          <Alert variant="danger" role="alert">
            {login.error.message}
          </Alert>
        ) : null}
        <FormActions>
          <AuthLink to="/forgot">Quên mật khẩu?</AuthLink>
          <Button type="submit" variant="gradient" disabled={login.isPending}>
            <PendingLabel pending={login.isPending} idle="Đăng nhập" busy="Đang đăng nhập…" />
          </Button>
        </FormActions>
      </form>
    </>
  );
}
