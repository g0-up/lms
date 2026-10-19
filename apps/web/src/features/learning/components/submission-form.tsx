import { zodResolver } from "@hookform/resolvers/zod";
import { CalendarClock, Upload } from "lucide-react";
import { useId, useMemo, useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { uploadErrorMessage } from "@/shared/api/upload-media";
import { cn } from "@/shared/lib/cn";
import { dueIn, fmtDateTime, fmtSize } from "@/shared/lib/format";
import { Alert } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Card, CardBody, CardHeader, CardTitle } from "@/shared/ui/card";
import { Field } from "@/shared/ui/field";
import { Input } from "@/shared/ui/input";
import { Progress } from "@/shared/ui/progress";
import { useSubmitHomework } from "../hooks/use-submit-homework";
import type { Submission } from "../model/schemas";
import {
  MAX_FILE_MB,
  SUBMISSION_ACCEPT,
  SUBMISSION_FILE_EXTS,
  submissionFieldErrors,
  submissionFormSchema,
  type SubmissionFormInput,
  type SubmissionFormValues,
} from "../model/submission-form";
import { isPast } from "../model/submissions";

export interface SubmissionFormProps {
  classId: string;
  lessonId: string;
  deadline: string | null;
  maxFiles: number;
  /** Newest submission, if any: its repo and commit prefill the form. */
  latest: Submission | undefined;
}

const isAbort = (error: unknown) => error instanceof DOMException && error.name === "AbortError";

/** Submit or resubmit a homework: a public repo, files, or both. */
export function SubmissionForm({ classId, lessonId, deadline, maxFiles, latest }: SubmissionFormProps) {
  const submit = useSubmitHomework(classId, lessonId);
  const titleId = useId();
  const [formError, setFormError] = useState<string | null>(null);
  const schema = useMemo(() => submissionFormSchema(maxFiles), [maxFiles]);
  const form = useForm<SubmissionFormInput, unknown, SubmissionFormValues>({
    resolver: zodResolver(schema),
    defaultValues: { repoUrl: latest?.repoUrl ?? "", commitRef: latest?.commitRef ?? "", files: [] },
  });
  const { register, control, setError } = form;
  const { errors, isSubmitting } = form.formState;
  const late = deadline !== null && isPast(deadline);
  const { progress } = submit;

  const onSubmit = form.handleSubmit(async (values) => {
    setFormError(null);
    try {
      await submit.mutateAsync(values);
    } catch (error) {
      if (isAbort(error)) return;
      const fields = submissionFieldErrors(error);
      if (fields.repoUrl) setError("repoUrl", { message: fields.repoUrl });
      if (fields.commitRef) setError("commitRef", { message: fields.commitRef });
      if (fields.mediaIds) setError("files", { message: fields.mediaIds });
      if (Object.keys(fields).length === 0) setFormError(uploadErrorMessage(error));
    }
  });

  return (
    <Card>
      <form noValidate aria-labelledby={titleId} onSubmit={(e) => void onSubmit(e)}>
        <CardHeader>
          <CardTitle id={titleId}>{latest ? `Nộp lại (lần #${String(latest.no + 1)})` : "Nộp bài"}</CardTitle>
          {deadline ? (
            <span className={cn("inline-flex items-center gap-1.5 text-sm", late ? "text-danger" : "text-ink-3")}>
              <CalendarClock className="size-4" aria-hidden="true" />
              Hạn {fmtDateTime(deadline)} · {dueIn(deadline)}
            </span>
          ) : null}
        </CardHeader>
        <CardBody className="grid gap-4">
          {latest?.status === "pending" ? (
            <Alert variant="info">
              Lần nộp #{latest.no} đang chờ chấm. Nộp lại sẽ thay lần nộp đó; giảng viên chỉ chấm lần mới nhất.
            </Alert>
          ) : null}
          {late ? (
            <Alert variant="warn">Đã quá hạn nộp. Bạn vẫn nộp được, bài sẽ được đánh dấu nộp muộn.</Alert>
          ) : null}
          <Field
            label="Đường dẫn repo"
            help="GitHub, GitLab hoặc Bitbucket. Repo phải để public để giảng viên mở được."
            error={errors.repoUrl?.message}
          >
            <Input
              type="url"
              inputMode="url"
              autoComplete="url"
              placeholder="https://github.com/ten-ban/ten-repo"
              {...register("repoUrl")}
            />
          </Field>
          <Field
            label="Commit hoặc tag"
            help="Không bắt buộc. Giúp giảng viên chấm đúng phiên bản bạn nộp."
            error={errors.commitRef?.message}
          >
            <Input autoComplete="off" placeholder="vd: a1b2c3d hoặc v1.0" {...register("commitRef")} />
          </Field>
          <Controller
            control={control}
            name="files"
            render={({ field, fieldState }) => (
              <div className="grid gap-2">
                <Field
                  label="File đính kèm"
                  help={`Tối đa ${String(maxFiles)} file, mỗi file ${String(MAX_FILE_MB)} MB: ${SUBMISSION_FILE_EXTS.join(", ")}. Cần repo hoặc ít nhất một file.`}
                  error={fieldState.error?.message}
                >
                  <Input
                    type="file"
                    multiple
                    accept={SUBMISSION_ACCEPT}
                    name={field.name}
                    ref={field.ref}
                    onBlur={field.onBlur}
                    onChange={(e) => {
                      field.onChange(Array.from(e.target.files ?? []));
                    }}
                    className="h-auto py-2 file:mr-3 file:border-0 file:bg-surface-100 file:px-3 file:py-1 file:text-sm file:text-ink"
                  />
                </Field>
                {field.value.length > 0 ? (
                  <ul aria-label="File đã chọn" className="grid gap-1 text-sm">
                    {field.value.map((file, i) => (
                      <li key={`${file.name}-${String(i)}`} className="break-all">
                        {file.name} <span className="text-ink-3">· {fmtSize(file.size)}</span>
                      </li>
                    ))}
                  </ul>
                ) : null}
              </div>
            )}
          />
          {progress ? (
            <div className="grid gap-1.5" role="status">
              <span className="text-sm text-ink-3">
                Đang tải file {progress.index + 1}/{progress.total} · {progress.pct}%
              </span>
              <Progress value={progress.pct} aria-label={`Đã tải lên ${String(progress.pct)}%`} />
            </div>
          ) : null}
          {formError ? (
            <Alert variant="danger" role="alert">
              {formError}
            </Alert>
          ) : null}
          <div className="flex justify-end">
            <Button type="submit" disabled={isSubmitting} aria-busy={isSubmitting}>
              <Upload aria-hidden="true" /> {latest ? "Nộp lại" : "Nộp bài"}
            </Button>
          </div>
        </CardBody>
      </form>
    </Card>
  );
}
