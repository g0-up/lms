import { useId, type Ref } from "react";
import { MarkdownContent } from "@/shared/ui/markdown-content";
import type { OpenQuestion } from "../model/quiz-schemas";
import { toggleOption } from "../model/quiz-session";

export interface QuizQuestionProps {
  question: OpenQuestion;
  index: number;
  total: number;
  selected: readonly string[];
  disabled: boolean;
  onChange: (optionIds: string[]) => void;
  /** The question's heading; navigation moves focus there. */
  headingRef: Ref<HTMLDivElement>;
}

/** One question of an open attempt: radios for a single answer, checkboxes for several. */
export function QuizQuestion({ question, index, total, selected, disabled, onChange, headingRef }: QuizQuestionProps) {
  const headingId = useId();
  const multi = question.type === "multi";
  return (
    <fieldset aria-labelledby={headingId} className="grid min-w-0 gap-3">
      {/* A div heading: the question text is sanitized HTML that may hold its own blocks. */}
      <div
        id={headingId}
        ref={headingRef}
        role="heading"
        aria-level={2}
        tabIndex={-1}
        className="flex items-start gap-3 text-md font-semibold outline-none"
      >
        <span
          aria-hidden="true"
          className="grid size-7 shrink-0 place-items-center rounded-full bg-surface-blue-50 text-sm text-navy-700 tabular-nums"
        >
          {index + 1}
        </span>
        <MarkdownContent html={question.bodyHtml} className="min-w-0 flex-1" />
      </div>
      <p className="text-sm text-ink-3">
        Câu {index + 1}/{total} · {multi ? "Chọn tất cả đáp án đúng" : "Chọn một đáp án"} · {question.points} điểm
      </p>
      <div className="grid gap-2">
        {question.options.map((option) => (
          <label
            key={option.id}
            className="flex min-h-11 cursor-pointer items-center gap-3 border border-line px-4 py-2.5 has-checked:border-navy-700 has-checked:bg-surface-blue-50 has-disabled:cursor-not-allowed"
          >
            <input
              type={multi ? "checkbox" : "radio"}
              name={`q-${question.id}`}
              value={option.id}
              checked={selected.includes(option.id)}
              disabled={disabled}
              onChange={() => {
                onChange(toggleOption(question.type, selected, option.id));
              }}
              // design-audit-allow hit-area: the wrapping label provides the 44px target.
              className="size-4 shrink-0 accent-navy-700"
            />
            <span className="min-w-0 break-words">{option.body}</span>
          </label>
        ))}
      </div>
    </fieldset>
  );
}
