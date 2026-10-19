import { Circle, CircleCheck, CircleX } from "lucide-react";
import { useId } from "react";
import type { FieldErrors, UseFormRegister } from "react-hook-form";
import { cn } from "@/shared/lib/cn";
import { Badge } from "@/shared/ui/badge";
import { Card, CardBody, CardHeader, CardTitle } from "@/shared/ui/card";
import { Input } from "@/shared/ui/input";
import { MarkdownContent } from "@/shared/ui/markdown-content";
import type { ReviewQuestion } from "../model/quiz-schemas";
import type { ReviewFormInput } from "../model/review-form";

// Field path of one question's note; the template keeps react-hook-form's path typing.
const notePath = (i: number) => `notes.${String(i)}.text` as `notes.${number}.text`;

function verdict(q: ReviewQuestion): string {
  const max = String(q.points);
  switch (q.status) {
    case "correct":
      return `Đúng · ${String(q.pointsAwarded)}/${max} điểm`;
    case "wrong":
      return `Sai · 0/${max} điểm`;
    case "blank":
      return `Bỏ trống · 0/${max} điểm`;
  }
}

interface AnswerProps {
  question: ReviewQuestion;
  index: number;
  formId: string;
  register: UseFormRegister<ReviewFormInput>;
  error: string | undefined;
}

function Answer({ question, index, formId, register, error }: AnswerProps) {
  const ids = useId();
  const position = index + 1;
  const correct = question.status === "correct";
  return (
    <li className="grid gap-3 border border-line p-4">
      <div className="flex items-start gap-3 font-semibold">
        <span
          aria-hidden="true"
          className="grid size-7 shrink-0 place-items-center rounded-full bg-surface-blue-50 text-sm text-navy-700 tabular-nums"
        >
          {position}
        </span>
        <span className="sr-only">Câu {position}.</span>
        <MarkdownContent html={question.bodyHtml} className="min-w-0 flex-1" />
      </div>
      <p className={cn("flex items-center gap-1.5 text-sm", correct ? "text-ok" : "text-danger")}>
        {correct ? <CircleCheck aria-hidden="true" className="size-4" /> : <CircleX aria-hidden="true" className="size-4" />}
        {verdict(question)}
      </p>
      <ul className="grid gap-2">
        {question.options.map((option) => (
          <li
            key={option.id}
            className={cn(
              "flex min-h-11 items-center gap-3 border px-4 py-2.5",
              option.isCorrect ? "border-ok-line bg-ok-bg" : option.selected ? "border-danger-line bg-danger-bg" : "border-line",
            )}
          >
            {option.isCorrect ? (
              <CircleCheck aria-hidden="true" className="size-4 shrink-0 text-ok" />
            ) : option.selected ? (
              <CircleX aria-hidden="true" className="size-4 shrink-0 text-danger" />
            ) : (
              <Circle aria-hidden="true" className="size-4 shrink-0 text-ink-3" />
            )}
            <span className="min-w-0 flex-1 break-words">{option.body}</span>
            <span className="sr-only">
              {option.isCorrect ? " (đáp án đúng)" : ""}
              {option.selected ? " (học viên chọn)" : ""}
            </span>
            {option.selected ? (
              <Badge variant="subtle" aria-hidden="true">
                Học viên chọn
              </Badge>
            ) : null}
          </li>
        ))}
      </ul>
      <div className="grid gap-1.5">
        <label htmlFor={`${ids}-note`} className="text-sm font-semibold text-ink">
          Ghi chú cho câu {position} <span className="font-normal text-ink-3">(không bắt buộc)</span>
        </label>
        <Input
          id={`${ids}-note`}
          form={formId}
          placeholder={correct ? "vd: Giải thích ngắn vì sao đúng" : "vd: Xem lại phần … trong bài đọc"}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? `${ids}-error` : undefined}
          {...register(notePath(index))}
        />
        {error ? (
          <div id={`${ids}-error`} className="text-xs text-danger">
            {error}
          </div>
        ) : null}
      </div>
    </li>
  );
}

export interface AttemptAnswersProps {
  questions: ReviewQuestion[];
  /** The review form the note fields belong to; they sit next to their question, outside the form element. */
  formId: string;
  register: UseFormRegister<ReviewFormInput>;
  errors: FieldErrors<ReviewFormInput>;
  /** A newer attempt came in: the notes cannot be saved any more. */
  locked: boolean;
}

/** Every question in the order the student saw it: the key, the student's picks, the points and a note field. */
export function AttemptAnswers({ questions, formId, register, errors, locked }: AttemptAnswersProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Bài làm từng câu</CardTitle>
        <span className="text-sm text-ink-3">Theo thứ tự học viên thấy</span>
      </CardHeader>
      <CardBody>
        <fieldset disabled={locked} className="m-0 min-w-0 border-0 p-0">
          <legend className="sr-only">Ghi chú từng câu</legend>
          <ol className="grid gap-4">
            {questions.map((q, i) => (
              <Answer
                key={q.id}
                question={q}
                index={i}
                formId={formId}
                register={register}
                error={errors.notes?.[i]?.text?.message}
              />
            ))}
          </ol>
        </fieldset>
      </CardBody>
    </Card>
  );
}
