import { Field } from "@/shared/ui/field";
import { FormDialog } from "@/shared/ui/form-dialog";
import { Input } from "@/shared/ui/input";
import { criterionFormDefaults, criterionFormSchema, type CriterionBody } from "../model/criterion-form";
import type { Criterion } from "../model/schemas";

export interface CriterionFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Absent when adding. */
  criterion?: Criterion;
  /** Throw an ApiError to keep the dialog open with the server's message. */
  onSave: (body: CriterionBody) => Promise<unknown>;
}

/** "Thêm tiêu chí chấm"/"Sửa tiêu chí chấm": what is graded and its maximum points. */
export function CriterionFormDialog({ open, onOpenChange, criterion, onSave }: CriterionFormDialogProps) {
  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={criterion ? "Sửa tiêu chí chấm" : "Thêm tiêu chí chấm"}
      schema={criterionFormSchema}
      defaultValues={criterionFormDefaults(criterion)}
      submitLabel={criterion ? "Lưu" : "Thêm tiêu chí"}
      onSubmit={onSave}
    >
      {(form) => (
        <div className="grid grid-cols-[2fr_1fr] gap-4 max-[560px]:grid-cols-1">
          <Field label="Tiêu chí" required error={form.formState.errors.description?.message}>
            <Input required placeholder="vd: Chạy đúng yêu cầu" autoComplete="off" {...form.register("description")} />
          </Field>
          <Field
            label="Điểm tối đa"
            required
            help="Số nguyên từ 1 đến 100."
            error={form.formState.errors.maxPoints?.message}
          >
            <Input
              type="number"
              inputMode="numeric"
              min={1}
              max={100}
              step={1}
              required
              {...form.register("maxPoints")}
            />
          </Field>
        </div>
      )}
    </FormDialog>
  );
}
