import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { useLocation, useNavigate } from "react-router";
import { toast } from "sonner";
import { Alert } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { authApi } from "../api/auth-api";
import { AuthHeading } from "../components/auth-heading";
import { AuthLink } from "../components/auth-link";
import { FormActions } from "../components/form-actions";
import { NewPasswordFields } from "../components/new-password-fields";
import { LOGIN_PATH, tokenFromHash } from "../model/navigation";
import { newPasswordFormSchema, type NewPasswordFormValues } from "../model/schemas";

const RESET_PATH = "/reset-password";

/**
 * The reset token arrives in the URL fragment (`#token=…`), which browsers never send to the
 * server, proxies or Referer. It is read once into state and wiped from the address bar.
 */
export function Component() {
  const location = useLocation();
  const navigate = useNavigate();
  const [token] = useState(() => tokenFromHash(location.hash));

  useEffect(() => {
    if (window.location.hash) {
      window.history.replaceState(window.history.state, "", RESET_PATH);
    }
  }, []);

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<NewPasswordFormValues, unknown, NewPasswordFormValues>({
    resolver: zodResolver(newPasswordFormSchema),
    defaultValues: { newPassword: "", confirmPassword: "" },
  });
  const resetPassword = useMutation({
    mutationFn: authApi.resetPassword,
    meta: { silent: true },
    onSuccess: () => {
      toast.success("Đã đặt lại mật khẩu. Hãy đăng nhập bằng mật khẩu mới.");
      void navigate(LOGIN_PATH, { replace: true });
    },
  });

  if (!token) {
    return (
      <>
        <AuthHeading title="Đặt lại mật khẩu" />
        <Alert variant="danger" role="alert" className="mt-4">
          Đường dẫn không hợp lệ. Yêu cầu đường dẫn mới.
        </Alert>
        <AuthLink to="/forgot" className="mt-2">
          Quên mật khẩu?
        </AuthLink>
      </>
    );
  }

  const onSubmit = handleSubmit((values) => {
    resetPassword.mutate({ token, ...values });
  });

  return (
    <>
      <AuthHeading title="Đặt lại mật khẩu" />
      <form noValidate onSubmit={(event) => void onSubmit(event)} className="mt-5 grid gap-3">
        <NewPasswordFields register={register} errors={errors} />
        {resetPassword.error ? (
          <Alert variant="danger" role="alert">
            {resetPassword.error.message}
          </Alert>
        ) : null}
        <FormActions>
          <AuthLink to="/login">Quay lại đăng nhập</AuthLink>
          <Button type="submit" variant="gradient" disabled={resetPassword.isPending}>
            Đặt lại mật khẩu
          </Button>
        </FormActions>
      </form>
    </>
  );
}
