import { Controller } from "react-hook-form";
import { toast } from "sonner";
import { shortTitle } from "@/shared/domain";
import { Field } from "@/shared/ui/field";
import { FormDialog } from "@/shared/ui/form-dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui/select";
import { Textarea } from "@/shared/ui/textarea";
import { useGrantAttempts } from "../hooks/use-quiz-review";
import { GRANT_ATTEMPT_CHOICES, grantDefaults, grantedMessage, grantFormSchema } from "../model/grant-form";
import type { ClassQuizItem, MemberQuiz } from "../model/quiz-schemas";

export interface GrantDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  classId: string;
  member: MemberQuiz;
  item: ClassQuizItem;
}

/** Grants one member 1–3 more attempts at an assessment, with the reason kept in the class's grant history. */
export function GrantDialog({ open, onOpenChange, classId, member, item }: GrantDialogProps) {
  const grant = useGrantAttempts(classId, member.memberId);
  const used = `${String(member.used)}${member.limit === null ? "" : `/${String(member.limit)}`}`;
  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={`Cấp thêm lượt cho ${member.name}`}
      description={`${shortTitle(item.lesson.title)} · đã dùng ${used} lượt.`}
      schema={grantFormSchema}
      defaultValues={grantDefaults}
      submitLabel="Cấp thêm lượt"
      onSubmit={async (body) => {
        await grant.mutateAsync({ lessonId: item.lesson.id, ...body });
        toast.success(grantedMessage(body.extraAttempts, member.name));
      }}
    >
      {(form) => (
        <>
          <Controller
            control={form.control}
            name="extraAttempts"
            render={({ field }) => (
              <Select value={field.value} onValueChange={field.onChange}>
                <Field label="Số lượt cấp thêm">
                  <SelectTrigger onBlur={field.onBlur}>
                    <SelectValue />
                  </SelectTrigger>
                </Field>
                <SelectContent>
                  {GRANT_ATTEMPT_CHOICES.map((n) => (
                    <SelectItem key={n} value={n}>
                      {n} lượt
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
          <Field
            label="Lý do"
            required
            help="Lưu vào lịch sử cấp lượt của lớp."
            error={form.formState.errors.reason?.message}
          >
            <Textarea required rows={3} placeholder="vd: Mất kết nối khi đang làm bài" {...form.register("reason")} />
          </Field>
        </>
      )}
    </FormDialog>
  );
}
