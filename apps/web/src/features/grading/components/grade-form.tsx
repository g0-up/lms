import { zodResolver } from "@hookform/resolvers/zod";
import { useId, useMemo, useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { Link, useNavigate } from "react-router";
import { toast } from "sonner";
import { NETWORK_ERROR_MESSAGE, isApiError } from "@/shared/api/errors";
import { passPoints } from "@/shared/domain";
import { Alert } from "@/shared/ui/alert";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card, CardHeader, CardTitle } from "@/shared/ui/card";
import { Field } from "@/shared/ui/field";
import { Input } from "@/shared/ui/input";
import { Textarea } from "@/shared/ui/textarea";
import { useGrade } from "../hooks/use-submission";
import {
  gradeDefaults,
  gradeFieldErrors,
  gradeFormSchema,
  gradeTotal,
  gradedMessage,
  type GradeBody,
  type GradeFormInput,
} from "../model/grade-form";
import { itemHref, queueHref, type GradingBase } from "../model/links";
import type { Criterion, SubmissionDetail } from "../model/schemas";

// Field paths of one criterion row; the template keeps react-hook-form's path typing.
const pointsPath = (i: number) => `scores.${String(i)}.points` as `scores.${number}.points`;
const notePath = (i: number) => `scores.${String(i)}.comment` as `scores.${number}.comment`;

function LiveTotal({ criteria, points, passPercent }: { criteria: Criterion[]; points: string[]; passPercent: number }) {
  const { total, max, pct, missing, passed } = gradeTotal(criteria, points, passPercent);
  return (
    <div
      aria-live="polite"
      className="flex min-h-14 flex-wrap items-center justify-between gap-3 bg-surface-50 px-4 py-3 text-sm"
    >
      <span>
        Tổng{" "}
        <b className="font-display text-xl text-ink tabular-nums">
          {total}/{max}
        </b>{" "}
        ({pct}%)
      </span>
      {missing > 0 ? (
        <span className="text-xs text-ink-3">Còn {missing} tiêu chí chưa nhập</span>
      ) : (
        <Badge variant={passed ? "ok" : "warn"}>{passed ? "Đạt" : "Cần làm lại"}</Badge>
      )}
    </div>
  );
}

export interface GradeFormProps {
  detail: SubmissionDetail;
  base: GradingBase;
}

/**
 * FR-62 grading by criterion. Locked when a newer submission came in. A first grading moves on to the next queue
 * item (or back to the queue when it is empty); an edit stays on the page.
 */
export function GradeForm({ detail, base }: GradeFormProps) {
  const { submission, lesson } = detail;
  const { criteria, passPercent, maxPoints } = lesson;
  const current = submission.grade;
  const locked = !detail.isLatest || submission.status === "superseded";
  const grade = useGrade(submission.id);
  const navigate = useNavigate();
  const ids = useId();
  const titleId = `${ids}-title`;
  const [formError, setFormError] = useState<string | null>(null);
  const schema = useMemo(() => gradeFormSchema(criteria), [criteria]);
  const form = useForm<GradeFormInput, unknown, GradeBody>({
    resolver: zodResolver(schema),
    defaultValues: gradeDefaults(criteria, current),
  });
  const { register, control, setError } = form;
  const { errors, isSubmitting } = form.formState;
  const scores = useWatch({ control, name: "scores" });

  const onSubmit = form.handleSubmit(async (body) => {
    setFormError(null);
    try {
      const outcome = await grade.mutateAsync(body);
      const message = gradedMessage(outcome.grade);
      if (outcome.updated) {
        toast.success(message);
      } else if (outcome.nextQueueItem) {
        const next = outcome.nextQueueItem;
        toast.success(`${message} Chuyển sang bài tiếp theo.`);
        void navigate(itemHref(base, next.kind, next.itemId));
      } else {
        toast.success(`${message} Đã chấm hết hàng chờ.`);
        void navigate(queueHref(base));
      }
    } catch (error) {
      const fields = gradeFieldErrors(error);
      if (fields.comment) setError("comment", { message: fields.comment });
      criteria.forEach((c, i) => {
        const message = fields.scores[c.id];
        if (message) setError(pointsPath(i), { message });
      });
      if (!fields.comment && Object.keys(fields.scores).length === 0) {
        setFormError(isApiError(error) ? error.message : NETWORK_ERROR_MESSAGE);
      }
    }
  });

  return (
    <Card className="min-[961px]:sticky min-[961px]:top-4">
      <form noValidate aria-labelledby={titleId} onSubmit={(e) => void onSubmit(e)}>
        <CardHeader>
          <CardTitle id={titleId}>{current ? "Sửa kết quả chấm" : "Chấm theo tiêu chí"}</CardTitle>
          <span className="text-sm text-ink-3">
            Đạt từ {passPercent}% ({passPoints(maxPoints, passPercent)}/{maxPoints} điểm)
          </span>
        </CardHeader>
        <fieldset disabled={locked} className="m-0 grid min-w-0 gap-4 border-0 p-6 max-[720px]:px-4">
          <legend className="sr-only">Điểm từng tiêu chí</legend>
          {criteria.map((c, i) => {
            const id = `${ids}-score-${String(i)}`;
            const maxId = `${id}-max`;
            const errorId = `${id}-error`;
            const pointsError = errors.scores?.[i]?.points?.message;
            const noteError = errors.scores?.[i]?.comment?.message;
            return (
              <div key={c.id} className="grid gap-2 border-b border-line pb-3">
                <div className="flex items-center justify-between gap-3">
                  <label htmlFor={id} className="flex min-w-0 items-start gap-2 text-sm font-semibold text-ink">
                    <span
                      aria-hidden="true"
                      className="inline-grid size-7 flex-none place-items-center bg-surface-blue-50 text-xs font-semibold text-navy-700 tabular-nums"
                    >
                      {i + 1}
                    </span>
                    {c.description}
                  </label>
                  <span className="inline-flex flex-none items-center gap-1">
                    <Input
                      id={id}
                      type="number"
                      inputMode="numeric"
                      min={0}
                      max={c.maxPoints}
                      step={1}
                      required
                      aria-describedby={pointsError ? `${maxId} ${errorId}` : maxId}
                      aria-invalid={pointsError ? true : undefined}
                      className="w-18 pr-2 text-right"
                      {...register(pointsPath(i))}
                    />
                    <span id={maxId} className="text-sm text-ink-3">
                      / {c.maxPoints}
                    </span>
                  </span>
                </div>
                {pointsError ? (
                  <div id={errorId} className="text-xs text-danger">
                    {pointsError}
                  </div>
                ) : null}
                <Input
                  placeholder="Ghi chú cho tiêu chí này (không bắt buộc)"
                  aria-label={`Ghi chú: ${c.description}`}
                  aria-invalid={noteError ? true : undefined}
                  aria-describedby={noteError ? `${id}-note-error` : undefined}
                  {...register(notePath(i))}
                />
                {noteError ? (
                  <div id={`${id}-note-error`} className="text-xs text-danger">
                    {noteError}
                  </div>
                ) : null}
              </div>
            );
          })}
          <LiveTotal criteria={criteria} points={scores.map((s) => s.points)} passPercent={passPercent} />
          <Field
            label="Nhận xét chung"
            required
            help="Học viên nhận email kết quả kèm nhận xét này."
            error={errors.comment?.message}
          >
            <Textarea rows={4} required className="min-h-28 font-sans text-[16px]" {...register("comment")} />
          </Field>
          {formError ? (
            <Alert variant="danger" role="alert">
              {formError}
            </Alert>
          ) : null}
          <div className="flex flex-wrap justify-end gap-2">
            <Button asChild variant="ghost">
              <Link to={queueHref(base)}>Về hàng chờ</Link>
            </Button>
            <Button type="submit" disabled={isSubmitting} aria-busy={isSubmitting}>
              {current ? "Lưu kết quả mới" : "Lưu kết quả"}
            </Button>
          </div>
        </fieldset>
      </form>
    </Card>
  );
}
