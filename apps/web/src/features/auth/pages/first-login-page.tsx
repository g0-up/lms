import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import { homeOf } from "@/shared/domain";
import { fmtDateTime } from "@/shared/lib/format";
import { Alert } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { authApi, meKey } from "../api/auth-api";
import { AuthHeading } from "../components/auth-heading";
import { FormActions } from "../components/form-actions";
import { NewPasswordFields } from "../components/new-password-fields";
import { useLogout } from "../hooks/use-logout";
import { useMe } from "../hooks/use-me";
import { newPasswordFormSchema, type NewPasswordFormValues } from "../model/schemas";

/**
 * Replaces the temporary password. The current password is not asked again: the user has just
 * signed in with it, and the API only requires it for accounts that are already active.
 */
export function Component() {
  const { data: user } = useMe();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const logout = useLogout();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<NewPasswordFormValues, unknown, NewPasswordFormValues>({
    resolver: zodResolver(newPasswordFormSchema),
    defaultValues: { newPassword: "", confirmPassword: "" },
  });
  const changePassword = useMutation({
    mutationFn: authApi.changePassword,
    meta: { silent: true },
    onSuccess: (updated) => {
      // Store the fresh user before navigating so the route guards see mustChangePassword=false.
      queryClient.setQueryData(meKey, updated);
      void queryClient.invalidateQueries({ queryKey: meKey });
      toast.success("Đã lưu mật khẩu. Chào mừng bạn vào lớp.");
      void navigate(homeOf(updated.role), { replace: true });
    },
  });

  if (!user) return null;

  const onSubmit = handleSubmit(({ newPassword, confirmPassword }) => {
    changePassword.mutate({ newPassword, confirmPassword });
  });

  const expiry = user.tempPasswordExpiresAt
    ? ` Mật khẩu tạm còn hiệu lực đến ${fmtDateTime(user.tempPasswordExpiresAt)}.`
    : "";

  return (
    <>
      <AuthHeading title="Đặt mật khẩu của bạn">
        {`Xin chào ${user.name}. Trước khi vào lớp, hãy thay mật khẩu tạm bằng mật khẩu riêng của bạn.${expiry}`}
      </AuthHeading>
      <form noValidate onSubmit={(event) => void onSubmit(event)} className="grid gap-3">
        <NewPasswordFields register={register} errors={errors} />
        {changePassword.error ? (
          <Alert variant="danger" role="alert">
            {changePassword.error.message}
          </Alert>
        ) : null}
        <FormActions>
          <Button
            variant="link"
            className="min-h-11"
            disabled={logout.isPending}
            onClick={() => {
              logout.mutate();
            }}
          >
            Đăng xuất
          </Button>
          <Button type="submit" variant="gradient" disabled={changePassword.isPending}>
            Lưu mật khẩu
          </Button>
        </FormActions>
      </form>
    </>
  );
}
