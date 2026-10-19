import { cn } from "@/shared/lib/cn";
import { Badge } from "@/shared/ui/badge";
import type { QuestionStats as Stats } from "../model/quiz-schemas";
import { isLowCorrect } from "../model/quiz-tab";

/** Share of the class's submitted attempts that got each question right; a question under 30% is flagged in words. */
export function QuestionStats({ stats }: { stats: Stats }) {
  return (
    <div className="border-b border-line px-6 py-4 max-[720px]:px-4">
      <h3 className="text-xs font-medium tracking-[1px] text-ink-3 uppercase">Tỉ lệ trả lời đúng từng câu</h3>
      {stats.attempts === 0 ? (
        <p className="mt-2 text-sm text-ink-3">Chưa có lượt nộp nào.</p>
      ) : (
        <ul className="mt-2 grid gap-2">
          {stats.rows.map((row) => {
            const pct = row.correctPercent ?? 0;
            const low = isLowCorrect(row.correctPercent);
            return (
              <li
                key={row.questionId}
                className="grid grid-cols-[minmax(0,1fr)_160px_48px] items-center gap-4 text-sm max-[720px]:grid-cols-[minmax(0,1fr)_48px]"
              >
                <span className="min-w-0 text-ink">
                  <b className="font-semibold">Câu {row.position}.</b> {row.bodyText}
                  {low ? (
                    <>
                      {" "}
                      <Badge variant="danger">Dưới 30%: xem lại câu hỏi</Badge>
                    </>
                  ) : null}
                </span>
                <span
                  role="img"
                  aria-label={`${String(pct)}% trả lời đúng`}
                  className={cn(
                    "h-2 overflow-hidden max-[720px]:hidden",
                    low ? "bg-danger-bg shadow-[inset_0_0_0_1px_var(--color-danger-line)]" : "bg-surface-blue-100",
                  )}
                >
                  <i className={cn("block h-full", low ? "bg-danger" : "bg-navy-700")} style={{ width: `${String(pct)}%` }} />
                </span>
                <span className="text-right text-ink tabular-nums">{pct}%</span>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
