import { Check, CircleCheck, Send } from "lucide-react";
import { useId, type ComponentProps } from "react";
import type { FieldErrors, UseFormRegister } from "react-hook-form";
import { Link } from "react-router";
import { QUIZ_KIND_VI } from "@/shared/domain";
import { fmtDateTime } from "@/shared/lib/format";
import { Alert } from "@/shared/ui/alert";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card, CardHeader, CardTitle } from "@/shared/ui/card";
import { Field } from "@/shared/ui/field";
import { Textarea } from "@/shared/ui/textarea";
import type { AttemptDetail } from "../model/quiz-schemas";
import { reviewFormCopy, type ReviewFormInput } from "../model/review-form";
import { fmtWait, hoursSince } from "../model/wait";

export interface ReviewFormProps {
  detail: AttemptDetail;
  /** Id of the form element; the per-question notes next to the answers point at it. */
  formId: string;
  register: UseFormRegister<ReviewFormInput>;
  errors: FieldErrors<ReviewFormInput>;
  submitting: boolean;
  formError: string | null;
  locked: boolean;
  back: { label: string; to: string };
  onSubmit: ComponentProps<"form">["onSubmit"];
}

/**
 * The general comment on an attempt and the way to send it: confirming an assessment attempt, commenting a practice
 * one, or editing the current review. The automatic score is shown, never edited.
 */
export function ReviewForm({
  detail,
  formId,
  register,
  errors,
  submitting,
  formError,
  locked,
  back,
  onSubmit,
}: ReviewFormProps) {
  const ids = useId();
  const titleId = `${ids}-title`;
  const assess = detail.mode === "assessment";
  const current = detail.reviews.at(0) ?? null;
  const edits = detail.reviews.length - 1;
  const copy = reviewFormCopy(detail.mode, current !== null);
  return (
    <Card className="min-[961px]:sticky min-[961px]:top-4">
      <form id={formId} noValidate aria-labelledby={titleId} onSubmit={onSubmit}>
        <CardHeader>
          <CardTitle id={titleId}>{copy.title}</CardTitle>
          {assess && !current && !locked ? (
            <span className="text-sm text-ink-3">Đã chờ {fmtWait(hoursSince(detail.submittedAt))}</span>
          ) : null}
        </CardHeader>
        <fieldset disabled={locked} className="m-0 grid min-w-0 gap-4 border-0 p-6 max-[720px]:px-4">
          <legend className="sr-only">Nhận xét lượt #{detail.no}</legend>
          <div className="flex min-h-14 flex-wrap items-center justify-between gap-3 bg-surface-50 px-4 py-3 text-sm">
            <span>
              Điểm tự động{" "}
              <b className="font-display text-xl text-ink tabular-nums">
                {detail.score}/{detail.maxScore}
              </b>{" "}
              ({detail.percent}%)
            </span>
            {assess && detail.passed !== null ? (
              <Badge variant={detail.passed ? "ok" : "warn"}>{detail.passed ? "Đạt" : "Chưa đạt"}</Badge>
            ) : (
              <Badge variant="subtle">{QUIZ_KIND_VI.practice}</Badge>
            )}
          </div>
          <p className="text-sm text-ink-3">
            Điểm tự động đã tính vào tiến độ; giảng viên không sửa điểm.
            {assess
              ? " Xác nhận để ghi nhận đã xem bài và gửi nhận xét cho học viên."
              : " Nhận xét là tùy chọn với quiz học liệu."}
          </p>
          {current ? (
            <p className="flex flex-wrap items-center gap-1.5 text-sm text-ink">
              <CircleCheck aria-hidden="true" className="size-4 text-ok" />
              {assess ? "Đã xác nhận" : "Đã nhận xét"} bởi {current.reviewer.name} · {fmtDateTime(current.createdAt)}
              {edits > 0 ? ` · đã sửa ${String(edits)} lần` : ""}
            </p>
          ) : null}
          <Field
            label="Nhận xét chung"
            required
            help="Học viên nhận email và thấy nhận xét này cùng ghi chú từng câu trong trang kết quả."
            error={errors.comment?.message}
          >
            <Textarea rows={5} required className="min-h-28 font-sans text-[16px]" {...register("comment")} />
          </Field>
          <p className="text-sm text-ink-3">Ghi chú từng câu nhập ngay dưới mỗi câu hỏi bên cạnh.</p>
          {formError ? (
            <Alert variant="danger" role="alert">
              {formError}
            </Alert>
          ) : null}
          <div className="flex flex-wrap justify-end gap-2">
            <Button asChild variant="ghost">
              <Link to={back.to}>{back.label}</Link>
            </Button>
            <Button type="submit" disabled={submitting} aria-busy={submitting}>
              {assess && !current ? <Check aria-hidden="true" /> : <Send aria-hidden="true" />}
              {copy.submit}
            </Button>
          </div>
        </fieldset>
      </form>
    </Card>
  );
}
