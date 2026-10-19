import { Field } from "@/shared/ui/field";
import { FormDialog } from "@/shared/ui/form-dialog";
import { Input } from "@/shared/ui/input";
import type { Unit } from "../model/schemas";
import { unitFormSchema } from "../model/unit-form";

export interface UnitFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Absent when adding. */
  unit?: Unit;
  /** Throw an ApiError (a taken title, say) to keep the dialog open with the server's message. */
  onSave: (title: string) => Promise<unknown>;
}

/** "Thêm bài học" or "Đổi tên bài học": the unit's title only. */
export function UnitFormDialog({ open, onOpenChange, unit, onSave }: UnitFormDialogProps) {
  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={unit ? "Đổi tên bài học" : "Thêm bài học"}
      schema={unitFormSchema}
      defaultValues={{ title: unit?.title ?? "" }}
      submitLabel={unit ? "Lưu" : "Thêm"}
      onSubmit={(values) => onSave(values.title)}
    >
      {(form) => (
        <Field
          label="Tên bài học"
          required
          help="Một bài học gom nhiều học liệu: video, bài đọc, quiz và bài tập. Học viên hoàn thành bài học khi xong mọi học liệu bắt buộc trong đó."
          error={form.formState.errors.title?.message}
        >
          <Input required placeholder="vd: Cài đặt và Hello World" autoComplete="off" {...form.register("title")} />
        </Field>
      )}
    </FormDialog>
  );
}
