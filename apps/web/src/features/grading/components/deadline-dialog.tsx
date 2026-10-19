import { Controller } from "react-hook-form";
import { toast } from "sonner";
import { shortTitle } from "@/shared/domain";
import { Checkbox } from "@/shared/ui/checkbox";
import { Field } from "@/shared/ui/field";
import { FormDialog } from "@/shared/ui/form-dialog";
import { Input } from "@/shared/ui/input";
import { useDeadline } from "../hooks/use-class-homework";
import { deadlineDefaults, deadlineFormSchema } from "../model/deadline-form";
import type { ClassHomework, ClassHomeworkItem } from "../model/schemas";

export interface DeadlineDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  cls: ClassHomework["class"];
  item: ClassHomeworkItem;
}

/** Sets, changes or removes the deadline of one homework for this class only. */
export function DeadlineDialog({ open, onOpenChange, cls, item }: DeadlineDialogProps) {
  const deadline = useDeadline(cls.id, item.lesson.id);
  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={`Hạn nộp: ${shortTitle(item.lesson.title)}`}
      description={`Lớp ${cls.code}. Hạn nộp chỉ áp dụng cho lớp này.`}
      schema={deadlineFormSchema}
      defaultValues={deadlineDefaults(item.deadline)}
      submitLabel="Lưu hạn nộp"
      onSubmit={async ({ dueAt }) => {
        await deadline.mutateAsync(dueAt);
        toast.success(dueAt === null ? "Đã bỏ hạn nộp." : "Đã lưu hạn nộp.");
      }}
    >
      {(form) => (
        <>
          <Field
            label="Hạn nộp"
            required
            help="Theo giờ Việt Nam (GMT+7). Nộp sau hạn vẫn được nhận nhưng đánh dấu nộp muộn."
            error={form.formState.errors.dueAt?.message}
          >
            <Input type="datetime-local" required {...form.register("dueAt")} />
          </Field>
          {item.deadline ? (
            <Controller
              control={form.control}
              name="clear"
              render={({ field }) => (
                <label className="flex min-h-11 cursor-pointer items-center gap-2 text-sm text-ink">
                  <Checkbox
                    checked={field.value}
                    onCheckedChange={(checked) => {
                      field.onChange(checked === true);
                    }}
                    onBlur={field.onBlur}
                  />
                  Bỏ hạn nộp
                </label>
              )}
            />
          ) : null}
        </>
      )}
    </FormDialog>
  );
}
