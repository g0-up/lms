import { Circle, CircleCheck, CircleX, Clock, Lock, MessageSquare } from "lucide-react";
import { useEffect, useRef, type ReactNode } from "react";
import { Link, useLocation } from "react-router";
import { QUIZ_KIND_VI } from "@/shared/domain";
import { cn } from "@/shared/lib/cn";
import { fmtDateTime } from "@/shared/lib/format";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { CardBody } from "@/shared/ui/card";
import { MarkdownContent } from "@/shared/ui/markdown-content";
import { Skeleton } from "@/shared/ui/skeleton";
import { useQuizAttempt } from "../hooks/use-quiz-attempt";
import type { AssessmentResult, PracticeResult, QuizOverview, QuizResult } from "../model/quiz-schemas";
import { StartButton } from "./quiz-start";

function ReviewBox({ title, review, children }: { title: string; review: NonNullable<QuizResult["review"]>; children?: ReactNode }) {
  return (
    <section className="grid gap-2 border-l-4 border-navy-700 bg-surface-blue-50 px-4 py-3">
      <h2 className="text-xs font-semibold tracking-[.5px] text-ink-3 uppercase">
        {title} · {review.reviewerName}
      </h2>
      <p className="whitespace-pre-line">{review.comment}</p>
      {children}
      <p className="text-sm text-ink-3">
        {fmtDateTime(review.reviewedAt)}
        {review.updated ? " · đã cập nhật" : ""}
      </p>
    </section>
  );
}

type PracticeQuestion = PracticeResult["questions"][number];

function questionVerdict(q: PracticeQuestion): string {
  if (q.correct) return `Đúng · ${String(q.pointsAwarded)} điểm`;
  return q.options.some((o) => o.selected) ? "Sai · 0 điểm" : "Chưa trả lời · 0 điểm";
}

/** One practice question after submission: the key, what the student picked, the explanation and the teacher's note. */
function PracticeQuestionReview({ question, position }: { question: PracticeQuestion; position: number }) {
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
      <p className={cn("flex items-center gap-1.5 text-sm", question.correct ? "text-ok" : "text-danger")}>
        {question.correct ? <CircleCheck aria-hidden="true" className="size-4" /> : <CircleX aria-hidden="true" className="size-4" />}
        {questionVerdict(question)}
      </p>
      <ul className="grid gap-2">
        {question.options.map((option) => (
          <li
            key={option.id}
            className={cn(
              "flex min-h-11 items-center gap-3 border px-4 py-2.5",
              option.isCorrect
                ? "border-ok-line bg-ok-bg"
                : option.selected
                  ? "border-danger-line bg-danger-bg"
                  : "border-line",
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
              {option.selected ? " (bạn chọn)" : ""}
            </span>
            {option.selected ? (
              <Badge variant="subtle" aria-hidden="true">
                Bạn chọn
              </Badge>
            ) : null}
          </li>
        ))}
      </ul>
      {question.explanationHtml ? (
        <div className="grid gap-1 bg-surface-100 px-4 py-3 text-sm">
          <strong>Giải thích.</strong>
          <MarkdownContent html={question.explanationHtml} />
        </div>
      ) : null}
      {question.note ? (
        <div className="flex items-start gap-2 border-l-4 border-navy-700 bg-surface-blue-50 px-4 py-3 text-sm">
          <MessageSquare aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-navy-700" />
          <p>
            <strong>Ghi chú của giảng viên.</strong> {question.note}
          </p>
        </div>
      ) : null}
    </li>
  );
}

function PracticeBody({ result }: { result: PracticeResult }) {
  return (
    <>
      {result.review ? <ReviewBox title="Nhận xét của giảng viên" review={result.review} /> : null}
      <ol className="grid gap-4">
        {result.questions.map((q, i) => (
          <PracticeQuestionReview key={q.id} question={q} position={i + 1} />
        ))}
      </ol>
    </>
  );
}

/** Assessment: score, verdict and the teacher's confirmation only; the key never reaches this page. */
function AssessmentBody({ result }: { result: AssessmentResult }) {
  const { review } = result;
  return (
    <>
      {review ? (
        <ReviewBox title="Giảng viên đã xác nhận" review={review}>
          {review.notes.length > 0 ? (
            <ul className="grid gap-2">
              {review.notes.map((n) => (
                <li key={n.questionId} className="grid gap-0.5">
                  <div className="flex items-start gap-1 text-sm text-ink-3">
                    <span className="shrink-0">Câu {n.position}.</span>
                    <MarkdownContent html={n.bodyHtml} className="min-w-0 flex-1 [&_p]:mb-0" />
                  </div>
                  <p>{n.note}</p>
                </li>
              ))}
            </ul>
          ) : null}
        </ReviewBox>
      ) : result.reviewStatus === "pending" ? (
        <Alert variant="info" icon={<Clock aria-hidden="true" />}>
          <strong>Chờ giảng viên xác nhận.</strong> Điểm đã tính vào tiến độ. Bạn nhận email khi giảng viên xác nhận và
          nhận xét.
        </Alert>
      ) : null}
      {result.best ? (
        <p className="text-sm text-ink-3">
          Kết quả tính vào tiến độ là lượt tốt nhất: {result.best.percent}%.
          {result.attemptsLeft !== null ? ` Còn ${String(result.attemptsLeft)} lượt.` : ""}
        </p>
      ) : null}
      <p className="flex items-center gap-2 text-sm text-ink-3">
        <Lock aria-hidden="true" className="size-4 shrink-0" />
        Quiz bài tập chỉ hiện điểm và kết quả Đạt, không hiện đáp án.
      </p>
    </>
  );
}

function Verdict({ result }: { result: QuizResult }) {
  if (result.mode === "practice") return <Badge variant="info">{QUIZ_KIND_VI.practice}</Badge>;
  if (result.passed === null) return null;
  return result.passed ? (
    <Badge variant="ok">Đạt</Badge>
  ) : (
    <Badge variant="warn">Chưa đạt · cần {result.passPercent}%</Badge>
  );
}

export interface QuizResultViewProps {
  attemptId: string;
  /** The start screen's data: whether another attempt may start. */
  overview: QuizOverview | undefined;
  starting: boolean;
  onStart: () => void;
  /** Back to the start screen, replacing the entry when the link named an attempt still open. */
  onLeave: (replace: boolean) => void;
}

/** A submitted attempt (`?result=`): practice shows the key per question, assessment only the score and verdict. */
export function QuizResultView({ attemptId, overview, starting, onStart, onLeave }: QuizResultViewProps) {
  const { pathname } = useLocation();
  const query = useQuizAttempt(attemptId);
  const headingRef = useRef<HTMLHeadingElement>(null);
  const result = query.data?.status === "submitted" ? query.data : null;
  const stillOpen = query.data?.status === "open";

  useEffect(() => {
    if (stillOpen) onLeave(true);
  }, [stillOpen, onLeave]);
  // After a submit the button that held focus is gone: start reading at the score.
  const shown = result !== null;
  useEffect(() => {
    const active = document.activeElement;
    if (shown && (active === null || active === document.body)) headingRef.current?.focus();
  }, [shown]);

  const back = (
    <Button asChild variant="ghost" size="sm">
      <Link to={pathname}>Về trang quiz</Link>
    </Button>
  );

  if (query.isPending || stillOpen) {
    return (
      <CardBody aria-busy="true" className="grid gap-4">
        <span className="sr-only">Đang tải kết quả</span>
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-24 w-full" />
      </CardBody>
    );
  }
  if (query.isError || !result) {
    return (
      <CardBody>
        <Alert variant="danger" role="alert">
          {query.error?.message ?? "Không tải được kết quả."}
          <AlertActions>
            <Button variant="outline" size="sm" onClick={() => void query.refetch()}>
              Thử lại
            </Button>
            {back}
          </AlertActions>
        </Alert>
      </CardBody>
    );
  }

  const assess = result.mode === "assessment";
  const passedQuiz = assess && overview?.best?.passed === true;
  return (
    <>
      <CardBody className="grid gap-5">
        <div
          className={cn(
            "flex flex-wrap items-center justify-between gap-3 border-l-4 px-4 py-3",
            result.mode === "assessment" && result.passed === false ? "border-warn-line bg-warn-bg" : "border-ok-line bg-ok-bg",
          )}
        >
          <div>
            <h2 ref={headingRef} tabIndex={-1} className="text-xs font-semibold tracking-[.5px] text-ink-3 uppercase outline-none">
              Lượt {result.no} · nộp {fmtDateTime(result.submittedAt)}
            </h2>
            <p className="text-2xl font-semibold tabular-nums">
              {result.score}/{result.maxScore} <span className="text-md text-ink-3">({result.percent}%)</span>
            </p>
          </div>
          <Verdict result={result} />
        </div>
        {result.autoSubmitted ? (
          <Alert variant="warn" icon={<Clock aria-hidden="true" />}>
            Lượt này được tự nộp khi hết giờ. Chỉ các câu đã lưu trước hạn được chấm.
          </Alert>
        ) : null}
        {result.mode === "practice" ? <PracticeBody result={result} /> : <AssessmentBody result={result} />}
      </CardBody>
      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line px-6 py-4 max-[720px]:px-4">
        {back}
        {overview && !passedQuiz ? (
          <StartButton overview={overview} label="Làm lại" pending={starting} onStart={onStart} />
        ) : null}
      </div>
    </>
  );
}
