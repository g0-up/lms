import { useId, useState, type SubmitEvent } from "react";
import { toast } from "sonner";
import { fmtDateTime } from "@/shared/lib/format";
import { Alert } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Field } from "@/shared/ui/field";
import { Textarea } from "@/shared/ui/textarea";
import type { StageTarget } from "../api/summary-api";
import { useSaveReview } from "../hooks/use-member-stage";
import { editsLabel, reviewErrorField, reviewProblem } from "../model/review-form";
import type { StageReview } from "../model/schemas";

export interface StageReviewFormProps {
  classId: string;
  target: StageTarget;
  /**
   * The current review. The field keeps what the teacher typed while it changes (a save, or someone else's save
   * after a 409); remount the form (`key`) for another member or stage.
   */
  review: StageReview | null;
}

/** The teacher's review of one member's stage: written once, then edited; every save counts as a revision. */
export function StageReviewForm({ classId, target, review }: StageReviewFormProps) {
  const titleId = useId();
  const save = useSaveReview(classId, target);
  const [body, setBody] = useState(review?.body ?? "");
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const edits = review ? editsLabel(review.revision) : null;

  const submit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormError(null);
    const problem = reviewProblem(body);
    setFieldError(problem);
    if (problem) return;
    save.mutate(body, {
      onSuccess: (saved) => {
        setBody(saved.body);
        toast.success(review ? "Đã lưu thay đổi nhận xét." : "Đã lưu nhận xét.");
      },
      onError: (error) => {
        if (reviewErrorField(error) === "body") setFieldError(error.message);
        else setFormError(error.message);
      },
    });
  };

  return (
    <form noValidate aria-labelledby={titleId} onSubmit={submit} className="grid gap-4 border-t border-line p-6 max-[720px]:px-4">
      <h3 id={titleId}>Nhận xét tổng kết chặng</h3>
      {review ? (
        <p className="text-sm text-ink-3">
          Cập nhật {fmtDateTime(review.updatedAt)} bởi {review.authorName}
          {edits ? ` · ${edits}` : ""}
        </p>
      ) : null}
      <Field
        label="Nhận xét"
        required
        help="Học viên thấy nhận xét này trong trang tổng kết của mình."
        error={fieldError ?? undefined}
      >
        <Textarea
          rows={4}
          required
          className="min-h-28 font-sans text-[16px]"
          value={body}
          onChange={(e) => {
            setBody(e.target.value);
          }}
        />
      </Field>
      {formError ? (
        <Alert variant="danger" role="alert">
          {formError}
        </Alert>
      ) : null}
      <div className="flex justify-end">
        <Button type="submit" size="sm" disabled={save.isPending} aria-busy={save.isPending}>
          {review ? "Lưu thay đổi" : "Lưu nhận xét"}
        </Button>
      </div>
    </form>
  );
}
