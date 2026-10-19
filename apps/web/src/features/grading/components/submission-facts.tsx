import type { ReactNode } from "react";
import { Link as LinkIcon } from "lucide-react";
import { fmtDateTime } from "@/shared/lib/format";
import { Badge, SubmissionBadge } from "@/shared/ui/badge";
import { Card, CardBody, CardHeader, CardTitle } from "@/shared/ui/card";
import { FileChip } from "@/shared/ui/file-chip";
import type { SubmissionDetail } from "../model/schemas";
import { fmtWait, hoursSince } from "../model/wait";

function Fact({ term, children }: { term: string; children: ReactNode }) {
  return (
    <div className="contents">
      <dt className="text-ink-3">{term}</dt>
      <dd className="m-0 min-w-0 text-ink [overflow-wrap:anywhere] max-[720px]:mb-2">{children}</dd>
    </div>
  );
}

/** What was submitted, by whom and when, against the class deadline. */
export function SubmissionFacts({ detail }: { detail: SubmissionDetail }) {
  const { submission, student } = detail;
  return (
    <Card>
      <CardHeader>
        <CardTitle>Bài nộp #{submission.no}</CardTitle>
        {submission.status === "pending" ? (
          <Badge variant="info">Chờ chấm · {fmtWait(hoursSince(submission.submittedAt))}</Badge>
        ) : (
          <SubmissionBadge status={submission.status} />
        )}
      </CardHeader>
      <CardBody>
        <dl className="m-0 grid grid-cols-[max-content_minmax(0,1fr)] gap-x-5 gap-y-2 text-sm max-[720px]:grid-cols-1 max-[720px]:gap-y-0.5">
          <Fact term="Học viên">
            {student.name} <span className="text-xs text-ink-3">{student.email}</span>
          </Fact>
          <Fact term="Lớp">
            {detail.class.code} · {detail.class.name}
          </Fact>
          <Fact term="Nộp lúc">
            {fmtDateTime(submission.submittedAt)}
            {submission.isLate ? (
              <>
                {" "}
                <Badge variant="danger">Nộp muộn</Badge>
              </>
            ) : null}
          </Fact>
          <Fact term="Hạn nộp">{detail.deadline ? fmtDateTime(detail.deadline) : "Chưa đặt"}</Fact>
          {submission.repoUrl ? (
            <Fact term="Repo">
              <a
                href={submission.repoUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 break-all"
              >
                <LinkIcon aria-hidden="true" className="size-4 shrink-0" />
                {submission.repoUrl.replace(/^https:\/\//, "")}
                <span className="sr-only"> (mở tab mới)</span>
              </a>
            </Fact>
          ) : null}
          {submission.commitRef ? (
            <Fact term="Commit/tag">
              <code>{submission.commitRef}</code>
            </Fact>
          ) : null}
          {submission.files.length > 0 ? (
            <Fact term="File">
              <span className="flex flex-wrap gap-2">
                {submission.files.map((f) => (
                  <FileChip key={f.mediaId} mediaId={f.mediaId} fileName={f.fileName} sizeBytes={f.sizeBytes} />
                ))}
              </span>
            </Fact>
          ) : null}
        </dl>
      </CardBody>
    </Card>
  );
}
