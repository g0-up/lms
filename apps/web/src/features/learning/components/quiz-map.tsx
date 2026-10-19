import { cn } from "@/shared/lib/cn";

export interface QuizMapProps {
  /** The attempt's questions in order, with whether each has an answer. */
  questions: readonly { id: string; answered: boolean }[];
  current: number;
  onGo: (index: number) => void;
}

/** Jump-to-question buttons; answered questions are filled, the current one is outlined. */
export function QuizMap({ questions, current, onGo }: QuizMapProps) {
  return (
    <nav aria-label="Chuyển nhanh đến câu hỏi" className="border-b border-line px-6 py-3 max-[720px]:px-4">
      <ol className="flex flex-wrap gap-2">
        {questions.map(({ id, answered: done }, i) => (
          <li key={id}>
            <button
              type="button"
              aria-label={`Câu ${String(i + 1)}, ${done ? "đã trả lời" : "chưa trả lời"}`}
              aria-current={i === current ? "step" : undefined}
              onClick={() => {
                onGo(i);
              }}
              className={cn(
                "grid size-11 cursor-pointer place-items-center border text-sm font-semibold tabular-nums",
                "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-navy-700",
                done ? "border-navy-700 bg-navy-700 text-white" : "border-line bg-white text-ink",
                i === current && "outline-2 outline-offset-2 outline-navy-700",
              )}
            >
              {i + 1}
            </button>
          </li>
        ))}
      </ol>
    </nav>
  );
}
