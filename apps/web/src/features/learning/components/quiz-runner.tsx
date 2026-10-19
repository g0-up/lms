import { Check, ChevronLeft, ChevronRight, Timer } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { useMediaQuery } from "@/shared/hooks/use-media-query";
import { cn } from "@/shared/lib/cn";
import { fmtDateTime } from "@/shared/lib/format";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { CardBody } from "@/shared/ui/card";
import { Skeleton } from "@/shared/ui/skeleton";
import { useCountdown } from "../hooks/use-countdown";
import {
  TIME_UP_MESSAGE,
  useAnswerSaver,
  useClosedAttemptCheck,
  useQuizAttempt,
  useSettleSubmitted,
  useSubmitAttempt,
} from "../hooks/use-quiz-attempt";
import type { OpenAttempt } from "../model/quiz-schemas";
import {
  fmtCountdown,
  readPosition,
  resumeIndex,
  unansweredCount,
  warningsDue,
  writePosition,
  type WarningMinute,
} from "../model/quiz-session";
import { QuizMap } from "./quiz-map";
import { QuizQuestion } from "./quiz-question";
import { QuizSubmitDialog } from "./quiz-submit-dialog";

const WARN_MS = 5 * 60_000;
const DANGER_MS = 60_000;

/** Focus has nowhere to be (the control that held it was removed): the new view may take it. */
const focusLost = () => document.activeElement === null || document.activeElement === document.body;

function TimerPill({ ms }: { ms: number | null }) {
  if (ms === null) return <span className="text-sm text-ink-3">Không giới hạn thời gian</span>;
  return (
    <span
      role="timer"
      aria-label="Thời gian còn lại"
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-sm font-semibold tabular-nums",
        ms <= DANGER_MS
          ? "border-danger-line bg-danger-bg text-danger"
          : ms <= WARN_MS
            ? "border-warn-line bg-warn-bg text-warn"
            : "border-line bg-white text-ink",
      )}
    >
      <Timer aria-hidden="true" className="size-4" />
      {fmtCountdown(ms)}
    </span>
  );
}

interface QuizTakingProps {
  classId: string;
  lessonId: string;
  attempt: OpenAttempt;
  /** Client time the attempt arrived at: the countdown reads the server clock from there. */
  receivedAt: number;
  onResult: () => void;
}

/**
 * Focus mode: one question on screen with the status bar (clock, position, answered, save state), the question map
 * and the step buttons. Answers save as they are picked; the clock submits once when it runs out.
 */
function QuizTaking({ classId, lessonId, attempt, receivedAt, onResult }: QuizTakingProps) {
  const { id: attemptId, questions } = attempt;
  const total = questions.length;
  const [answers, setAnswers] = useState<Readonly<Record<string, string[] | undefined>>>(() =>
    Object.fromEntries(questions.map((q) => [q.id, q.selectedOptionIds])),
  );
  const [index, setIndex] = useState(() =>
    resumeIndex(
      questions,
      Object.fromEntries(questions.map((q) => [q.id, q.selectedOptionIds])),
      readPosition(attemptId),
    ),
  );
  const [announcement, setAnnouncement] = useState("");
  const [confirming, setConfirming] = useState(false);

  const closedCheck = useClosedAttemptCheck({ classId, lessonId, attemptId, onResult });
  const saver = useAnswerSaver(attemptId, attempt.savedAt, closedCheck);
  const submit = useSubmitAttempt({ classId, lessonId, attemptId, idle: saver.idle, onResult });
  const ms = useCountdown({ deadlineAt: attempt.deadlineAt, serverNow: attempt.serverNow, receivedAt });
  const timeUp = ms === 0;

  // Warnings at 5 and 1 minutes; a resume past a mark only gets the latest one. At 0 the client submits once
  // (the server may already have closed the attempt: the submit's 409 then opens its result).
  const fired = useRef(new Set<WarningMinute>());
  const autoSubmitted = useRef(false);
  const { mutate: submitMutate, isPending: submitting } = submit;
  useEffect(() => {
    if (ms === null) return;
    const due = warningsDue(ms, fired.current);
    if (due.length > 0) {
      due.forEach((m) => fired.current.add(m));
      toast.warning(`Còn ${String(Math.ceil(ms / 60_000))} phút. Câu trả lời đã chọn được lưu tự động.`);
    }
    if (ms === 0 && !autoSubmitted.current && !submitting) {
      autoSubmitted.current = true;
      submitMutate({ auto: true });
    }
  }, [ms, submitting, submitMutate]);

  // Moving to another question puts focus on its heading and the top of the card in view; the first render
  // leaves focus where it is unless the start button that held it is gone.
  const reduced = useMediaQuery("(prefers-reduced-motion: reduce)");
  const headingRef = useRef<HTMLDivElement>(null);
  const topRef = useRef<HTMLDivElement>(null);
  const shownIndex = useRef<number | null>(null);
  useEffect(() => {
    const first = shownIndex.current === null;
    if (shownIndex.current === index) return;
    shownIndex.current = index;
    if (first && !focusLost()) return;
    headingRef.current?.focus({ preventScroll: true });
    if (!first) topRef.current?.scrollIntoView({ block: "start", behavior: reduced ? "auto" : "smooth" });
  }, [index, reduced]);

  const go = (next: number) => {
    if (next < 0 || next >= total) return;
    setIndex(next);
    writePosition(attemptId, next);
    setAnnouncement(`Câu ${String(next + 1)} trên ${String(total)}`);
  };

  const pick = (questionId: string, optionIds: string[]) => {
    setAnswers((a) => ({ ...a, [questionId]: optionIds }));
    saver.save(questionId, optionIds);
  };

  const unanswered = unansweredCount(questions, answers);
  const answered = total - unanswered;
  const locked = timeUp || submitting;
  const question = questions.at(index);
  const failed = questions.flatMap((q, i) => {
    const failure = saver.failures[q.id];
    return failure ? [{ question: q, position: i + 1, failure }] : [];
  });
  const saveState = saver.saving
    ? "Đang lưu…"
    : failed.length > 0
      ? "Có câu chưa lưu được"
      : saver.savedAt
        ? `Đã lưu ${fmtDateTime(saver.savedAt).slice(-5)}`
        : "Tự lưu khi chọn";
  // Once the clock has run out there is nothing left to confirm: the button retries the automatic submit.
  const openConfirm = () => {
    if (timeUp) submitMutate({ auto: true });
    else setConfirming(true);
  };

  return (
    <div ref={topRef} className="scroll-mt-4">
      <p className="sr-only" aria-live="polite" aria-atomic="true">
        {announcement}
      </p>
      <div className="flex flex-wrap items-center gap-x-5 gap-y-2 border-b border-line px-6 py-3 max-[720px]:px-4">
        <TimerPill ms={ms} />
        <span className="text-sm">
          Câu <b>{index + 1}</b>/{total}
        </span>
        <span className="text-sm">
          {answered}/{total} đã trả lời
        </span>
        <span className="text-sm text-ink-3" aria-live="polite">
          {saveState}
        </span>
        <Button variant="outline" size="sm" className="ml-auto" disabled={submitting} onClick={openConfirm}>
          Nộp bài
        </Button>
      </div>
      <QuizMap
        questions={questions.map((q) => ({ id: q.id, answered: (answers[q.id]?.length ?? 0) > 0 }))}
        current={index}
        onGo={go}
      />
      <CardBody className="grid gap-5">
        {attempt.mode === "assessment" ? (
          <Alert variant="info">
            Lượt {attempt.no}
            {attempt.limit !== null ? `/${String(attempt.limit)}` : ""}. Hết giờ, hệ thống tự nộp với các câu đã lưu.
            Rời trang không dừng đồng hồ.
          </Alert>
        ) : null}
        {failed.map(({ question: q, position, failure }) => (
          <Alert key={q.id} variant="danger" role="alert">
            Câu {position} chưa lưu được: {failure.message}
            <AlertActions>
              <Button
                variant="outline"
                size="sm"
                disabled={locked}
                onClick={() => {
                  saver.save(q.id, answers[q.id] ?? failure.optionIds);
                }}
              >
                Thử lại
              </Button>
            </AlertActions>
          </Alert>
        ))}
        {question ? (
          <QuizQuestion
            key={question.id}
            question={question}
            index={index}
            total={total}
            selected={answers[question.id] ?? []}
            disabled={locked}
            onChange={(optionIds) => {
              pick(question.id, optionIds);
            }}
            headingRef={headingRef}
          />
        ) : null}
      </CardBody>
      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line px-6 py-4 max-[720px]:px-4">
        <Button
          variant="outline"
          disabled={index === 0}
          onClick={() => {
            go(index - 1);
          }}
        >
          <ChevronLeft aria-hidden="true" /> Câu trước
        </Button>
        {index < total - 1 ? (
          <Button
            onClick={() => {
              go(index + 1);
            }}
          >
            Câu tiếp <ChevronRight aria-hidden="true" />
          </Button>
        ) : (
          <Button variant="gradient" disabled={submitting} onClick={openConfirm}>
            <Check aria-hidden="true" /> Nộp bài
          </Button>
        )}
      </div>
      <QuizSubmitDialog
        open={confirming && !timeUp}
        onOpenChange={setConfirming}
        unanswered={unanswered}
        loading={submitting}
        onConfirm={() => {
          submitMutate(
            { auto: false },
            {
              onError: () => {
                setConfirming(false);
              },
            },
          );
        }}
      />
    </div>
  );
}

export interface QuizRunnerProps {
  classId: string;
  lessonId: string;
  attemptId: string;
  /** Shows the attempt's result; `replace` when the attempt turned out to be closed already on arrival. */
  onResult: (attemptId: string, replace: boolean) => void;
}

/** The open attempt: loads it, runs it, and hands over to the result once it is submitted. */
export function QuizRunner({ classId, lessonId, attemptId, onResult }: QuizRunnerProps) {
  const query = useQuizAttempt(attemptId);
  const settle = useSettleSubmitted(classId, lessonId);
  // Submitting fills the cache with the result before the page leaves this view: show the result once.
  const handled = useRef(false);
  const finish = useCallback(
    (replace: boolean) => {
      if (handled.current) return;
      handled.current = true;
      onResult(attemptId, replace);
    },
    [attemptId, onResult],
  );
  const showSubmitted = useCallback(() => {
    finish(false);
  }, [finish]);

  const attempt = query.data;
  // Closed on the server since the overview was read: at the deadline, or submitted in another tab.
  const closedOnArrival = attempt?.status === "submitted" ? attempt : null;
  useEffect(() => {
    if (!closedOnArrival || handled.current) return;
    settle(attemptId);
    if (closedOnArrival.autoSubmitted) toast.warning(TIME_UP_MESSAGE);
    finish(true);
  }, [closedOnArrival, attemptId, settle, finish]);

  if (query.isPending) {
    return (
      <CardBody aria-busy="true" className="grid gap-4">
        <span className="sr-only">Đang tải lượt làm bài</span>
        <Skeleton className="h-9 w-2/3" />
        <Skeleton className="h-11 w-full" />
        <Skeleton className="h-11 w-full" />
      </CardBody>
    );
  }
  if (query.isError) {
    return (
      <CardBody>
        <Alert variant="danger" role="alert">
          {query.error.message}
          <AlertActions>
            <Button variant="outline" size="sm" onClick={() => void query.refetch()}>
              Thử lại
            </Button>
          </AlertActions>
        </Alert>
      </CardBody>
    );
  }
  if (query.data.status !== "open") return null;
  return (
    <QuizTaking
      classId={classId}
      lessonId={lessonId}
      attempt={query.data}
      receivedAt={query.dataUpdatedAt}
      onResult={showSubmitted}
    />
  );
}
