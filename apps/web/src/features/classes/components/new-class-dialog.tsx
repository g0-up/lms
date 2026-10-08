import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useRef, useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { useNavigate } from "react-router";
import { toast } from "sonner";
import { Alert } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Dialog, DialogBody, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/shared/ui/dialog";
import { Field } from "@/shared/ui/field";
import { Input } from "@/shared/ui/input";
import { useCreateClass } from "../hooks/use-class-mutations";
import { usePublishedCourseVersions } from "../hooks/use-published-course-versions";
import { useTeachers } from "../hooks/use-teachers";
import {
  classFormError,
  classFormSchema,
  defaultDates,
  defaultVersionId,
  type ClassFormInput,
  type ClassFormValues,
} from "../model/class-form";
import { OptionSelect } from "./option-select";

export interface NewClassDialogProps {
  /** Open it only once the published course versions are known to exist. */
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const wide = "col-span-2 max-[720px]:col-span-1";

/** "Tạo lớp": a draft class on the latest published version, then its page. */
export function NewClassDialog({ open, onOpenChange }: NewClassDialogProps) {
  const navigate = useNavigate();
  const create = useCreateClass();
  const versions = usePublishedCourseVersions();
  const teachers = useTeachers();
  const contentRef = useRef<HTMLDivElement>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm<ClassFormInput, unknown, ClassFormValues>({
    resolver: zodResolver(classFormSchema),
    defaultValues: { code: "", name: "", courseVersionId: "", teacherId: "", ...defaultDates() },
  });
  const { reset, register, control } = form;
  const { errors, isSubmitting } = form.formState;
  const versionOptions = versions.data ?? [];
  const latest = defaultVersionId(versionOptions);

  // A stale server error from the previous opening must not reappear.
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) setFormError(null);
  }

  // Every opening starts from the defaults: dates from today, the newest published version.
  useEffect(() => {
    if (open) reset({ code: "", name: "", courseVersionId: latest, teacherId: "", ...defaultDates() });
  }, [open, latest, reset]);

  const submit = form.handleSubmit(async (values) => {
    setFormError(null);
    try {
      const created = await create.mutateAsync(values);
      toast.success("Đã tạo lớp ở trạng thái nháp.");
      onOpenChange(false);
      void navigate(`/admin/classes/${created.id}`);
    } catch (error) {
      const failure = classFormError(error);
      if (failure.field) form.setError(failure.field, { message: failure.message }, { shouldFocus: true });
      else setFormError(failure.message);
    }
  });

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!isSubmitting) onOpenChange(next); }}>
      <DialogContent
        ref={contentRef}
        aria-describedby={undefined}
        onOpenAutoFocus={(event) => {
          const first = contentRef.current?.querySelector<HTMLElement>("input:not([disabled])");
          if (first) {
            event.preventDefault();
            first.focus();
          }
        }}
      >
        <form noValidate onSubmit={(e) => void submit(e)}>
          <DialogHeader showClose={!isSubmitting}>
            <DialogTitle>Tạo lớp</DialogTitle>
          </DialogHeader>
          <DialogBody>
            <div className="grid grid-cols-2 gap-4 max-[720px]:grid-cols-1">
              <Field label="Mã lớp" required error={errors.code?.message}>
                <Input required autoComplete="off" placeholder="vd: basic04" {...register("code")} />
              </Field>
              <Field label="Tên lớp" required error={errors.name?.message}>
                <Input required autoComplete="off" placeholder="vd: Lập trình cơ bản – khóa 4" {...register("name")} />
              </Field>
              <Controller
                control={control}
                name="courseVersionId"
                render={({ field, fieldState }) => (
                  <Field
                    label="Phiên bản khóa học"
                    required
                    className={wide}
                    help="Chỉ phiên bản đã phát hành. Đổi được khi lớp còn ở trạng thái nháp."
                    error={fieldState.error?.message}
                  >
                    <OptionSelect
                      value={field.value}
                      onValueChange={field.onChange}
                      options={versionOptions.map((v) => ({ value: v.id, label: v.label }))}
                      placeholder="Chọn phiên bản"
                    />
                  </Field>
                )}
              />
              <Field label="Ngày bắt đầu dự kiến" required error={errors.startDate?.message}>
                <Input type="date" required {...register("startDate")} />
              </Field>
              <Field label="Ngày kết thúc dự kiến" required error={errors.endDate?.message}>
                <Input type="date" required {...register("endDate")} />
              </Field>
              <Controller
                control={control}
                name="teacherId"
                render={({ field, fieldState }) => (
                  <Field label="Giảng viên phụ trách" required className={wide} error={fieldState.error?.message}>
                    <OptionSelect
                      value={field.value}
                      onValueChange={field.onChange}
                      options={(teachers.data ?? []).map((t) => ({ value: t.id, label: t.name }))}
                      placeholder={teachers.isPending ? "Đang tải giảng viên…" : "Chọn giảng viên"}
                    />
                  </Field>
                )}
              />
            </div>
            {formError ? (
              <Alert variant="danger" role="alert">
                {formError}
              </Alert>
            ) : null}
          </DialogBody>
          <DialogFooter>
            <Button variant="outline" disabled={isSubmitting} onClick={() => { onOpenChange(false); }}>
              Hủy
            </Button>
            <Button type="submit" disabled={isSubmitting} aria-busy={isSubmitting}>
              Tạo lớp
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
