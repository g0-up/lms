import { Link } from "react-router";
import { fmtDateTime } from "@/shared/lib/format";
import { Card, CardHeader, CardTitle } from "@/shared/ui/card";
import { quizReviewHref, type GradingBase, type ReviewFrom } from "../model/links";
import type { AttemptDetail } from "../model/quiz-schemas";
import { ReviewStateBadge } from "./attempt-facts";

export interface ReviewHistoryProps {
  detail: AttemptDetail;
  base: GradingBase;
  /** Kept on the links so the way back stays the one the page was opened with. */
  from: ReviewFrom;
}

/** The student's submitted attempts of this quiz, newest first, each with its score and current comment. */
export function ReviewHistory({ detail, base, from }: ReviewHistoryProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Các lượt đã nộp</CardTitle>
      </CardHeader>
      <ul>
        {detail.attempts.map((a) => (
          <li
            key={a.id}
            className="flex flex-wrap items-start justify-between gap-3 border-b border-line px-5 py-3 last:border-b-0 max-[720px]:px-4"
          >
            <div className="grid min-w-0 flex-1 gap-0.5">
              <div className="font-medium text-ink">
                {a.id === detail.id ? (
                  `Lượt #${String(a.no)} (đang xem)`
                ) : (
                  <Link to={quizReviewHref(base, a.id, from)} className="-my-2.5 inline-block py-2.5">
                    Lượt #{a.no}
                  </Link>
                )}
              </div>
              <div className="flex flex-wrap gap-x-2 text-xs text-ink-3">
                <span>{fmtDateTime(a.submittedAt)}</span>
                <span className="tabular-nums">
                  {a.score}/{a.maxScore} · {a.percent}%
                </span>
                {a.review ? <span className="[overflow-wrap:anywhere]">{`${a.review.reviewer.name}: "${a.review.comment}"`}</span> : null}
              </div>
            </div>
            {a.reviewStatus ? <ReviewStateBadge attempt={a} mode={detail.mode} /> : null}
          </li>
        ))}
      </ul>
    </Card>
  );
}
