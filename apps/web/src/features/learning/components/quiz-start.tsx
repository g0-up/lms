import { Info, Play, Timer } from "lucide-react";
import { useId, type ReactNode } from "react";
import { Link } from "react-router";
import { toast } from "sonner";
import { QUIZ_KIND_VI, quizReviewBadge } from "@/shared/domain";
import { fmtDateTime } from "@/shared/lib/format";
import { Alert } from "@/shared/ui/alert";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { CardBody } from "@/shared/ui/card";
import type { AttemptSummary, QuizOverview } from "../model/quiz-schemas";

function Fact({ term, children }: { term: string; children: ReactNode }) {
  return (
    <div className="contents">
      <dt className="text-ink-3">{term}</dt>
      <dd className="m-0 min-w-0 text-ink max-[720px]:mb-2">{children}</dd>
    </div>
  );
}

/** Every attempt the student may have now has been used; the overview says so without its open attempt. */
const usedUp = (o: QuizOverview) => o.limit !== undefined && o.used >= o.limit && !o.openAttemptId;

export interface StartButtonProps {
  overview: QuizOverview;
  label: string;
  pending: boolean;
  onStart: () => void;
}

/**
 * Starts (or resumes) an attempt. Blocked, it stays focusable with `aria-disabled`, says why in its title and on
 * click, and the reason shows next to it.
 */
export function StartButton({ overview, label, pending, onStart }: StartButtonProps) {
  const blocker = overview.canStart ? null : (overview.blockedReason ?? "Chưa làm quiz được lúc này");
  return (
    <div className="flex flex-wrap items-center justify-end gap-3">
      {blocker ? <span className="text-sm text-ink-3">{blocker}</span> : null}
      <Button
        variant="gradient"
        aria-disabled={blocker ? true : undefined}
        title={blocker ?? undefined}
        disabled={pending}
        aria-busy={pending || undefined}
        onClick={() => {
          if (blocker) toast.error(blocker);
          else onStart();
        }}
      >
        <Play aria-hidden="true" /> {label}
      </Button>
    </div>
  );
}

function AttemptRow({ attempt, mode }: { attempt: AttemptSummary; mode: QuizOverview["mode"] }) {
  const review =
    attempt.reviewStatus && attempt.reviewStatus !== "superseded" ? quizReviewBadge(mode, attempt.reviewStatus) : null;
  return (
    <li className="flex flex-wrap items-center gap-3 border-b border-line px-4 py-3 last:border-b-0">
      <div className="min-w-0 flex-1">
        <Link to={`?result=${attempt.id}`} className="font-semibold text-navy-700 underline-offset-2 hover:underline">
          Lượt {attempt.no}
        </Link>
        <div className="flex flex-wrap gap-x-3 text-sm text-ink-3">
          <span>{fmtDateTime(attempt.submittedAt)}</span>
          {attempt.autoSubmitted ? <span>tự nộp khi hết giờ</span> : null}
        </div>
      </div>
      <span className="text-sm tabular-nums">
        {attempt.score}/{attempt.maxScore} · {attempt.percent}%
      </span>
      {attempt.passed !== undefined ? (
        <Badge variant={attempt.passed ? "ok" : "warn"}>{attempt.passed ? "Đạt" : "Chưa đạt"}</Badge>
      ) : null}
      {review ? <Badge variant={review.tone}>{review.label}</Badge> : null}
    </li>
  );
}

export interface QuizStartProps {
  overview: QuizOverview;
  /** Previous / next lesson, shown in the bottom bar next to the start button. */
  nav: ReactNode;
  starting: boolean;
  onStart: () => void;
}

/** The quiz before an attempt: settings, attempts used, verdict so far, how the attempt works and past attempts. */
export function QuizStart({ overview, nav, starting, onStart }: QuizStartProps) {
  const headingId = useId();
  const assess = overview.mode === "assessment";
  const passed = assess && overview.best?.passed === true;
  const out = usedUp(overview);
  const timed = assess && overview.timeLimitMinutes !== undefined;
  const hint = timed
    ? `Đồng hồ ${String(overview.timeLimitMinutes)} phút bắt đầu khi bạn bấm làm bài và vẫn chạy nếu bạn rời trang. Mỗi lần chỉ hiện một câu; câu trả lời được lưu ngay khi chọn; hết giờ, bài tự nộp.`
    : assess
      ? "Mỗi lần chỉ hiện một câu và câu trả lời được lưu ngay khi chọn."
      : "Mỗi lần chỉ hiện một câu và câu trả lời được lưu ngay khi chọn. Nộp xong, bạn xem được đáp án và giải thích từng câu.";
  const attemptsLabel =
    overview.limit === undefined
      ? `${String(overview.used)} đã dùng`
      : `${String(overview.used)}/${String(overview.limit)} đã dùng`;
  return (
    <>
      <CardBody className="grid gap-5">
        <dl className="m-0 grid grid-cols-[max-content_minmax(0,1fr)] gap-x-5 gap-y-2 text-sm max-[720px]:grid-cols-1 max-[720px]:gap-y-0.5">
          <Fact term="Loại quiz">
            {QUIZ_KIND_VI[overview.mode]} ·{" "}
            {assess ? "tính vào tiến độ, giảng viên xác nhận từng lượt" : "luyện tập, làm bao nhiêu lần cũng được"}
          </Fact>
          <Fact term="Số câu">
            {overview.questionCount} câu · {overview.maxScore} điểm
          </Fact>
          {overview.passPercent !== undefined ? <Fact term="Ngưỡng đạt">{overview.passPercent}%</Fact> : null}
          {assess ? (
            <>
              {overview.timeLimitMinutes !== undefined ? (
                <Fact term="Thời gian">{overview.timeLimitMinutes} phút mỗi lượt</Fact>
              ) : null}
              <Fact term="Số lượt">
                {attemptsLabel}
                {overview.granted > 0 ? " · đã được cấp thêm" : ""} · tính lượt tốt nhất
              </Fact>
            </>
          ) : null}
        </dl>
        {passed && overview.best ? (
          <Alert variant="ok">
            Bạn đã đạt quiz bài tập này với {overview.best.percent}%.
            {out ? "" : " Bạn vẫn có thể làm thêm; kết quả tính theo lượt tốt nhất."}
          </Alert>
        ) : null}
        {out && !passed ? (
          <Alert variant="warn">
            {overview.blockedReason ?? `Bạn đã dùng hết ${String(overview.limit)} lượt`} mà chưa đạt. Liên hệ giảng
            viên nếu cần cấp thêm lượt.
          </Alert>
        ) : null}
        {overview.canStart ? (
          <p className="flex items-start gap-2 text-sm text-ink-3">
            {timed ? (
              <Timer aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
            ) : (
              <Info aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
            )}
            <span>{hint}</span>
          </p>
        ) : null}
        {overview.attempts.length > 0 ? (
          <section aria-labelledby={headingId}>
            <h2 id={headingId} className="mb-2 text-xs font-semibold tracking-[.5px] text-ink-3 uppercase">
              Các lượt đã nộp
            </h2>
            <ul className="border border-line">
              {overview.attempts.map((attempt) => (
                <AttemptRow key={attempt.id} attempt={attempt} mode={overview.mode} />
              ))}
            </ul>
          </section>
        ) : null}
      </CardBody>
      <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line px-6 py-4 max-[720px]:px-4">
        {nav}
        <StartButton
          overview={overview}
          label={overview.attempts.length > 0 ? "Làm lượt mới" : "Bắt đầu làm bài"}
          pending={starting}
          onStart={onStart}
        />
      </div>
    </>
  );
}
