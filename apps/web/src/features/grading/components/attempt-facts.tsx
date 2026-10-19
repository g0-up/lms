import type { ReactNode } from "react";
import { QUIZ_KIND_VI, quizReviewBadge, shortTitle } from "@/shared/domain";
import { fmtDateTime } from "@/shared/lib/format";
import { Badge } from "@/shared/ui/badge";
import { Card, CardBody, CardHeader, CardTitle } from "@/shared/ui/card";
import type { AttemptDetail } from "../model/quiz-schemas";

function Fact({ term, children }: { term: string; children: ReactNode }) {
  return (
    <div className="contents">
      <dt className="text-ink-3">{term}</dt>
      <dd className="m-0 min-w-0 text-ink [overflow-wrap:anywhere] max-[720px]:mb-2">{children}</dd>
    </div>
  );
}

/** The attempt's review state; a practice attempt nobody commented on yet has none. */
export function ReviewStateBadge({ attempt, mode }: { attempt: Pick<AttemptDetail, "reviewStatus">; mode: AttemptDetail["mode"] }) {
  if (!attempt.reviewStatus) return <Badge variant="subtle">Chưa nhận xét</Badge>;
  const badge = quizReviewBadge(mode, attempt.reviewStatus);
  return <Badge variant={badge.tone}>{badge.label}</Badge>;
}

/** Who took the attempt, when, how long, and the automatic score against the threshold. */
export function AttemptFacts({ detail }: { detail: AttemptDetail }) {
  const { student, lesson, mode } = detail;
  const assess = mode === "assessment";
  return (
    <Card>
      <CardHeader>
        <CardTitle>Lượt #{detail.no}</CardTitle>
        <ReviewStateBadge attempt={detail} mode={mode} />
      </CardHeader>
      <CardBody>
        <dl className="m-0 grid grid-cols-[max-content_minmax(0,1fr)] gap-x-5 gap-y-2 text-sm max-[720px]:grid-cols-1 max-[720px]:gap-y-0.5">
          <Fact term="Học viên">
            {student.name} <span className="text-xs text-ink-3">{student.email}</span>
          </Fact>
          <Fact term="Lớp">
            {detail.class.code} · {detail.class.name}
          </Fact>
          <Fact term="Bài">
            {shortTitle(lesson.title)} <Badge variant={assess ? "info" : "subtle"}>{QUIZ_KIND_VI[mode]}</Badge>
          </Fact>
          <Fact term="Bắt đầu">{fmtDateTime(detail.startedAt)}</Fact>
          <Fact term="Nộp lúc">
            {fmtDateTime(detail.submittedAt)}
            {detail.autoSubmitted ? (
              <>
                {" "}
                <Badge variant="warn">Tự nộp khi hết giờ</Badge>
              </>
            ) : null}
          </Fact>
          <Fact term="Thời gian làm">
            {detail.minutes} phút
            {assess && detail.timeLimitMinutes !== null ? ` / ${String(detail.timeLimitMinutes)} phút` : ""}
          </Fact>
          <Fact term="Điểm tự động">
            <b className="tabular-nums">
              {detail.score}/{detail.maxScore}
            </b>{" "}
            ({detail.percent}%) · {detail.correctCount}/{detail.questionCount} câu đúng
            {assess && detail.passed !== null ? (
              <>
                {" "}
                <Badge variant={detail.passed ? "ok" : "warn"}>
                  {detail.passed ? "Đạt" : `Chưa đạt · cần ${String(detail.passPercent ?? 0)}%`}
                </Badge>
              </>
            ) : null}
          </Fact>
          <Fact term="Lượt">
            #{detail.no}
            {detail.limit === null ? " · không giới hạn lượt" : ` / ${String(detail.limit)} lượt`}
          </Fact>
        </dl>
      </CardBody>
    </Card>
  );
}
