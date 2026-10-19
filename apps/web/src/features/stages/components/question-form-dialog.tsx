import { useQuery } from "@tanstack/react-query";
import { useId, useState } from "react";
import { Controller, type UseFormReturn } from "react-hook-form";
import { Alert } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { Field } from "@/shared/ui/field";
import { FormDialog } from "@/shared/ui/form-dialog";
import { Input } from "@/shared/ui/input";
import { MarkdownContent } from "@/shared/ui/markdown-content";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui/select";
import { Skeleton } from "@/shared/ui/skeleton";
import { Textarea } from "@/shared/ui/textarea";
import { stagesApi } from "../api/stages-api";
import { lessonErrorMessage } from "../model/lesson-form";
import {
  QUESTION_MIN_OPTIONS,
  QUESTION_OPTION_ROWS,
  QUESTION_TYPE_LABEL,
  questionFormDefaults,
  questionFormSchema,
  toQuestionBody,
  type QuestionBody,
  type QuestionFormOutput,
  type QuestionFormValues,
} from "../model/question-form";
import type { Question } from "../model/schemas";

export interface QuestionFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Absent when adding. */
  question?: Question;
  /** Throw an ApiError to keep the dialog open with the server's message. */
  onSave: (body: QuestionBody) => Promise<unknown>;
}

/** "Thêm câu hỏi"/"Sửa câu hỏi": markdown body and explanation, type, points and up to six options. */
export function QuestionFormDialog({ open, onOpenChange, question, onSave }: QuestionFormDialogProps) {
  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={question ? "Sửa câu hỏi" : "Thêm câu hỏi"}
      schema={questionFormSchema}
      defaultValues={questionFormDefaults(question)}
      submitLabel={question ? "Lưu" : "Thêm câu hỏi"}
      onSubmit={(values) => onSave(toQuestionBody(values))}
    >
      {(form) => <QuestionFields form={form} />}
    </FormDialog>
  );
}

/** Form path of an option row's field; the cast keeps the path typed after `String` formats the index. */
function optionField<K extends "id" | "body" | "correct">(i: number, key: K) {
  return `options.${String(i)}.${key}` as `options.${number}.${K}`;
}

function QuestionFields({ form }: { form: UseFormReturn<QuestionFormValues, unknown, QuestionFormOutput> }) {
  const { register, control, formState, getValues } = form;
  const { errors } = formState;
  const legendId = useId();
  const optionsErrorId = useId();
  const optionsError = errors.options?.root?.message;
  const [preview, setPreview] = useState<{ body: string; explanation: string } | null>(null);

  return (
    <>
      <Field label="Nội dung câu hỏi" required help="Hỗ trợ **đậm** và `mã`." error={errors.bodySource?.message}>
        <Textarea required rows={3} {...register("bodySource")} />
      </Field>
      <div className="grid grid-cols-2 gap-4 max-[560px]:grid-cols-1">
        <Controller
          control={control}
          name="type"
          render={({ field }) => (
            <Select value={field.value} onValueChange={field.onChange}>
              <Field label="Loại câu">
                <SelectTrigger onBlur={field.onBlur}>
                  <SelectValue />
                </SelectTrigger>
              </Field>
              <SelectContent>
                {(["single", "multi"] as const).map((type) => (
                  <SelectItem key={type} value={type}>
                    {QUESTION_TYPE_LABEL[type]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
        <Field label="Điểm" required error={errors.points?.message}>
          <Input type="number" inputMode="numeric" min={1} step={1} required {...register("points")} />
        </Field>
      </div>
      <fieldset
        aria-labelledby={legendId}
        aria-describedby={optionsError ? optionsErrorId : undefined}
        className="m-0 grid gap-2 border-0 p-0"
      >
        <legend id={legendId} className="mb-1 p-0 text-sm font-semibold text-ink">
          Phương án <span className="text-xs font-normal text-ink-3">tối thiểu 2, đánh dấu đáp án đúng</span>
        </legend>
        {Array.from({ length: QUESTION_OPTION_ROWS }, (_, i) => {
          const n = i + 1;
          const bodyError = errors.options?.[i]?.body?.message;
          const bodyErrorId = `${optionsErrorId}-${String(n)}`;
          return (
            <div key={n} className="grid gap-1">
              <div className="flex items-center gap-3">
                <input type="hidden" {...register(optionField(i, "id"))} />
                <Controller
                  control={control}
                  name={optionField(i, "correct")}
                  render={({ field }) => (
                    <label className="flex min-h-11 shrink-0 cursor-pointer items-center gap-2 text-xs text-ink">
                      <Checkbox
                        checked={field.value}
                        aria-label={`Phương án ${String(n)} là đáp án đúng`}
                        onCheckedChange={(checked) => {
                          field.onChange(checked === true);
                        }}
                        onBlur={field.onBlur}
                      />
                      Đúng
                    </label>
                  )}
                />
                <Input
                  aria-label={`Phương án ${String(n)}`}
                  placeholder={`Phương án ${String(n)}${n > QUESTION_MIN_OPTIONS ? " (không bắt buộc)" : ""}`}
                  autoComplete="off"
                  aria-invalid={bodyError ? true : undefined}
                  aria-describedby={bodyError ? bodyErrorId : undefined}
                  {...register(optionField(i, "body"))}
                />
              </div>
              {bodyError ? (
                <div id={bodyErrorId} className="text-xs text-danger">
                  {bodyError}
                </div>
              ) : null}
            </div>
          );
        })}
        {optionsError ? (
          <div id={optionsErrorId} className="text-xs text-danger">
            {optionsError}
          </div>
        ) : null}
      </fieldset>
      <Field
        label="Giải thích"
        help="Hiện cho học viên sau khi nộp quiz luyện tập."
        error={errors.explanationSource?.message}
      >
        <Textarea rows={2} {...register("explanationSource")} />
      </Field>
      <div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            setPreview({ body: getValues("bodySource"), explanation: getValues("explanationSource") });
          }}
        >
          {preview ? "Cập nhật xem trước" : "Xem trước"}
        </Button>
      </div>
      {preview ? (
        <section aria-label="Xem trước câu hỏi" className="grid gap-3 border border-line p-4">
          <PreviewBlock label="Nội dung câu hỏi" source={preview.body} />
          <PreviewBlock label="Giải thích" source={preview.explanation} />
        </section>
      ) : null}
    </>
  );
}

/** Server-rendered markdown, as the learner will see it once the stage is published. */
function PreviewBlock({ label, source }: { label: string; source: string }) {
  const empty = source.trim() === "";
  const query = useQuery({
    queryKey: ["stage-markdown-preview", source],
    queryFn: ({ signal }) => stagesApi.previewMarkdown(source, signal),
    enabled: !empty,
    staleTime: 60_000,
    gcTime: 60_000,
  });
  let content;
  if (empty) content = <p className="text-sm text-ink-3">Chưa có nội dung.</p>;
  else if (query.isError) {
    content = (
      <Alert variant="danger" role="alert">
        {lessonErrorMessage(query.error)}
      </Alert>
    );
  } else if (query.isPending) {
    content = (
      <div aria-busy="true">
        <span className="sr-only">Đang tạo bản xem trước</span>
        <Skeleton className="h-12 w-full" />
      </div>
    );
  } else content = <MarkdownContent html={query.data.html} />;
  return (
    <div className="grid gap-1">
      <div className="text-xs font-semibold text-ink-3">{label}</div>
      {content}
    </div>
  );
}
