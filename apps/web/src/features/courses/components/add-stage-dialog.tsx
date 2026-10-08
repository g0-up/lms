import { Controller } from "react-hook-form";
import { z } from "zod";
import { Field } from "@/shared/ui/field";
import { FormDialog } from "@/shared/ui/form-dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui/select";
import type { StageOption } from "../model/publish-blocker";

const schema = z.object({ stageVersionId: z.string().min(1, "Chọn phiên bản chặng.") });

export interface AddStageDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Never empty: the caller refuses to open the dialog without options. */
  options: readonly StageOption[];
  /** Throw to keep the dialog open with the server's message. */
  onAdd: (stageVersionId: string) => Promise<unknown>;
}

/** "Thêm chặng vào khóa học": one published stage version, appended to the draft. */
export function AddStageDialog({ open, onOpenChange, options, onAdd }: AddStageDialogProps) {
  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Thêm chặng vào khóa học"
      schema={schema}
      defaultValues={{ stageVersionId: options[0]?.value ?? "" }}
      submitLabel="Thêm"
      onSubmit={({ stageVersionId }) => onAdd(stageVersionId)}
    >
      {(form) => (
        <Controller
          control={form.control}
          name="stageVersionId"
          render={({ field, fieldState }) => (
            <Select value={field.value} onValueChange={field.onChange}>
              <Field
                label="Phiên bản chặng"
                help="Chỉ liệt kê phiên bản đã phát hành của các chặng chưa có trong khóa học."
                error={fieldState.error?.message}
              >
                <SelectTrigger onBlur={field.onBlur}>
                  <SelectValue />
                </SelectTrigger>
              </Field>
              <SelectContent>
                {options.map((o) => (
                  <SelectItem key={o.value} value={o.value}>
                    {o.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
      )}
    </FormDialog>
  );
}
