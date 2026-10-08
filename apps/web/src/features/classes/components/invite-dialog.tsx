import { ApiError, isApiError } from "@/shared/api/errors";
import { Alert } from "@/shared/ui/alert";
import { Field } from "@/shared/ui/field";
import { FormDialog } from "@/shared/ui/form-dialog";
import { Input } from "@/shared/ui/input";
import { useInvite } from "../hooks/use-member-mutations";
import { inviteErrorMessage, inviteFormSchema } from "../model/invite-form";
import type { ClassDetail } from "../model/schemas";

export interface InviteDialogProps {
  cls: Pick<ClassDetail, "id" | "code">;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/** Invites a student by email: a new account with a temporary password, or an existing one added. */
export function InviteDialog({ cls, open, onOpenChange }: InviteDialogProps) {
  const invite = useInvite(cls.id);

  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={`Mời học viên vào ${cls.code}`}
      schema={inviteFormSchema}
      defaultValues={{ email: "", fullName: "" }}
      submitLabel="Gửi lời mời"
      onSubmit={async ({ email, fullName }) => {
        try {
          await invite.mutateAsync(fullName ? { email, fullName } : { email });
        } catch (error) {
          // The dialog shows the error's message: give it the copy for this refusal.
          if (!isApiError(error)) throw error;
          throw new ApiError(error.status, error.code, inviteErrorMessage(error) ?? error.message, error.details);
        }
      }}
    >
      {(form) => (
        <>
          <Field
            label="Email"
            required
            help="Email được chuẩn hóa (cắt khoảng trắng, chữ thường) trước khi so khớp."
            error={form.formState.errors.email?.message}
          >
            <Input type="email" required autoComplete="off" placeholder="hocvien@example.com" {...form.register("email")} />
          </Field>
          <Field label="Họ tên" help="Bắt buộc khi email chưa có tài khoản." error={form.formState.errors.fullName?.message}>
            <Input autoComplete="off" placeholder="Bỏ trống nếu email đã có tài khoản" {...form.register("fullName")} />
          </Field>
          <Alert variant="info">
            Email mới: tạo tài khoản, gửi mật khẩu tạm hiệu lực 72 giờ. Email đã có tài khoản: chỉ thêm vào lớp và gửi
            thông báo. Mật khẩu tạm không bao giờ hiển thị trên giao diện này.
          </Alert>
        </>
      )}
    </FormDialog>
  );
}
