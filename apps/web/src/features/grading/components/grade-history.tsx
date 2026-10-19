import { Link } from "react-router";
import { fmtDateTime } from "@/shared/lib/format";
import { Card, CardHeader, CardTitle } from "@/shared/ui/card";
import { gradeHref, type GradingBase } from "../model/links";
import type { Grade, SubmissionDetail } from "../model/schemas";

function GradeEntry({ grade }: { grade: Grade }) {
  return (
    <div className="mt-1 grid gap-0.5 border-l-2 border-line pl-3 text-xs text-ink-3">
      <span>
        {grade.gradedBy}: {grade.totalPoints}/{grade.maxPoints} · {grade.result === "passed" ? "Đạt" : "Cần làm lại"}
        {grade.comment ? ` · "${grade.comment}"` : ""}
      </span>
      <span className="text-ink-3">{fmtDateTime(grade.gradedAt)}</span>
      <ul aria-label="Điểm từng tiêu chí" className="grid gap-0.5">
        {grade.scores.map((s) => (
          <li key={s.criterionId}>
            {s.description}: <span className="tabular-nums">{s.points}/{s.maxPoints}</span>
            {s.comment ? ` · ${s.comment}` : ""}
          </li>
        ))}
      </ul>
    </div>
  );
}

/** The student's submissions of this homework, newest first, each with every grading and its scores. */
export function GradeHistory({ detail, base }: { detail: SubmissionDetail; base: GradingBase }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Lịch sử nộp và chấm</CardTitle>
      </CardHeader>
      <ul>
        {detail.history.map((s) => (
          <li key={s.id} className="border-b border-line px-5 py-3 last:border-b-0 max-[720px]:px-4">
            <div className="font-medium text-ink">
              {s.id === detail.submission.id ? (
                `Lần nộp #${String(s.no)} (đang xem)`
              ) : (
                <Link to={gradeHref(base, s.id)} className="-my-2.5 inline-block py-2.5">
                  Lần nộp #{s.no}
                </Link>
              )}
            </div>
            <div className="flex flex-wrap gap-x-2 text-xs text-ink-3">
              <span>{fmtDateTime(s.submittedAt)}</span>
              {s.isLate ? <span className="text-danger">nộp muộn</span> : null}
              {s.status === "superseded" ? <span>Đã thay bằng lần nộp sau</span> : null}
              {s.status === "pending" ? <span>Chờ chấm</span> : null}
            </div>
            {s.grades.map((g) => (
              <GradeEntry key={g.id} grade={g} />
            ))}
          </li>
        ))}
      </ul>
    </Card>
  );
}
