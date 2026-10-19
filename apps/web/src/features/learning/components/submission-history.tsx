import { fmtDateTime } from "@/shared/lib/format";
import { SubmissionBadge } from "@/shared/ui/badge";
import { Card, CardHeader, CardTitle } from "@/shared/ui/card";
import type { Submission } from "../model/schemas";
import { repoLabel } from "../model/submissions";

/** Every submission of the student, newest first, with its status and score. */
export function SubmissionHistory({ submissions }: { submissions: Submission[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Lịch sử nộp</CardTitle>
      </CardHeader>
      <ul>
        {submissions.map((s) => (
          <li
            key={s.id}
            className="flex flex-wrap items-center justify-between gap-3 border-b border-line px-6 py-3 last:border-b-0 max-[720px]:px-4"
          >
            <div className="min-w-0">
              <div className="font-medium">Lần nộp #{s.no}</div>
              <div className="flex flex-wrap gap-x-3 text-xs text-ink-3">
                <span>{fmtDateTime(s.submittedAt)}</span>
                {s.isLate ? <span className="text-danger">nộp muộn</span> : null}
                {s.repoUrl ? <span className="break-all">{repoLabel(s.repoUrl)}</span> : null}
                {s.files.length > 0 ? <span>{s.files.map((f) => f.fileName).join(", ")}</span> : null}
                {s.grade ? (
                  <span>
                    {s.grade.totalPoints}/{s.grade.maxPoints} điểm
                  </span>
                ) : null}
              </div>
            </div>
            <SubmissionBadge status={s.status} />
          </li>
        ))}
      </ul>
    </Card>
  );
}
