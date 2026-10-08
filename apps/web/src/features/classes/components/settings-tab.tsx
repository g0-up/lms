import { zodResolver } from "@hookform/resolvers/zod";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useId, useMemo, useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { toast } from "sonner";
import { STATUS_VI } from "@/shared/domain";
import { Alert } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Card, CardBody, CardHeader, CardTitle } from "@/shared/ui/card";
import { Field } from "@/shared/ui/field";
import { Input } from "@/shared/ui/input";
import { useUpdateClass } from "../hooks/use-class-mutations";
import { courseVersionsQuery } from "../hooks/use-published-course-versions";
import { useTeachers } from "../hooks/use-teachers";
import {
  changedFields,
  classFormError,
  settingsFormSchema,
  settingsVersionOptions,
  type SettingsFormInput,
  type SettingsFormValues,
} from "../model/class-form";
import type { ClassDetail } from "../model/schemas";
import { OptionSelect } from "./option-select";

/** Settings tab: name, planned dates, teacher, and the course version while the class is a draft. */
export function SettingsTab({ cls }: { cls: ClassDetail }) {
  const courses = useQuery(courseVersionsQuery);
  const teachers = useTeachers();
  const update = useUpdateClass(cls.id);
  const titleId = useId();
  const [formError, setFormError] = useState<string | null>(null);
  const locked = cls.status !== "draft";
  const version = cls.courseVersion;

  const initial = useMemo<SettingsFormValues>(
    () => ({
      name: cls.name,
      courseVersionId: cls.courseVersion.id,
      teacherId: cls.teacher.id,
      startDate: cls.startDate,
      endDate: cls.endDate,
    }),
    [cls],
  );
  const form = useForm<SettingsFormInput, unknown, SettingsFormValues>({
    resolver: zodResolver(settingsFormSchema),
    defaultValues: initial,
  });
  const { reset, register, control } = form;
  const { errors, isDirty, isSubmitting } = form.formState;

  // A saved or reloaded class becomes the new baseline of "changed".
  useEffect(() => {
    reset(initial);
  }, [initial, reset]);

  const versionOptions = settingsVersionOptions(courses.data, {
    id: version.id,
    courseName: version.courseName,
    versionNo: version.versionNo,
  }).map((o) => ({ value: o.id, label: o.label }));
  // The current teacher stays selectable even once disabled, so the select never shows blank.
  const teacherList = teachers.data ?? [];
  const teacherOptions = [
    ...(teacherList.some((t) => t.id === cls.teacher.id) ? [] : [{ value: cls.teacher.id, label: cls.teacher.name }]),
    ...teacherList.map((t) => ({ value: t.id, label: t.name })),
  ];

  const submit = form.handleSubmit(async (values) => {
    setFormError(null);
    const body = changedFields(initial, values);
    if (Object.keys(body).length === 0) return;
    try {
      await update.mutateAsync(body);
      toast.success("Đã lưu cài đặt lớp.");
    } catch (error) {
      setFormError(classFormError(error).message);
    }
  });

  return (
    <Card>
      <form noValidate aria-labelledby={titleId} onSubmit={(e) => void submit(e)}>
        <CardHeader>
          <CardTitle id={titleId}>Cài đặt lớp</CardTitle>
        </CardHeader>
        <CardBody className="grid gap-4">
          {locked ? (
            <Alert variant="info">
              Lớp {STATUS_VI[cls.status].toLowerCase()} không đổi được phiên bản khóa học. Lớp chạy trọn đời trên{" "}
              {version.courseName} v{version.versionNo}.
            </Alert>
          ) : null}
          <div className="grid grid-cols-2 gap-4 max-[720px]:grid-cols-1">
            <Field label="Mã lớp">
              <Input value={cls.code} disabled readOnly />
            </Field>
            <Field label="Tên lớp" required error={errors.name?.message}>
              <Input required autoComplete="off" {...register("name")} />
            </Field>
            <Controller
              control={control}
              name="courseVersionId"
              render={({ field, fieldState }) => (
                <Field
                  label="Phiên bản khóa học"
                  required
                  help={locked ? undefined : "Chỉ liệt kê phiên bản đã phát hành."}
                  error={fieldState.error?.message}
                >
                  <OptionSelect
                    value={field.value}
                    onValueChange={field.onChange}
                    options={versionOptions}
                    disabled={locked}
                  />
                </Field>
              )}
            />
            <Controller
              control={control}
              name="teacherId"
              render={({ field, fieldState }) => (
                <Field label="Giảng viên phụ trách" required error={fieldState.error?.message}>
                  <OptionSelect
                    value={field.value}
                    onValueChange={field.onChange}
                    options={teacherOptions}
                    placeholder="Chọn giảng viên"
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
          </div>
          {formError ? (
            <Alert variant="danger" role="alert">
              {formError}
            </Alert>
          ) : null}
          <div className="flex justify-end">
            <Button type="submit" disabled={!isDirty || isSubmitting} aria-busy={isSubmitting}>
              Lưu thay đổi
            </Button>
          </div>
        </CardBody>
      </form>
    </Card>
  );
}
