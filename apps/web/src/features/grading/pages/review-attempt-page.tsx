import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useId, useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
import { toast } from "sonner";
import { NETWORK_ERROR_MESSAGE, isApiError } from "@/shared/api/errors";
import { QUIZ_KIND_VI, shortTitle } from "@/shared/domain";
import { useDocumentTitle } from "@/shared/hooks/use-document-title";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Crumbs, type Crumb } from "@/shared/ui/crumbs";
import { PageHead } from "@/shared/ui/page-head";
import { Skeleton } from "@/shared/ui/skeleton";
import { AttemptAnswers } from "../components/attempt-answers";
import { AttemptFacts } from "../components/attempt-facts";
import { ReviewForm } from "../components/review-form";
import { ReviewHistory } from "../components/review-history";
import { useGradingBase } from "../hooks/use-grading-base";
import { useQuizAttemptDetail, useReviewAttempt } from "../hooks/use-quiz-review";
import {
  classQuizHref,
  classesHref,
  itemHref,
  parseReviewFrom,
  queueHref,
  quizReviewHref,
  type GradingBase,
  type ReviewFrom,
} from "../model/links";
import type { AttemptDetail } from "../model/quiz-schemas";
import {
  reviewDefaults,
  reviewErrorField,
  reviewFormSchema,
  reviewedMessage,
  type ReviewBody,
  type ReviewFormInput,
} from "../model/review-form";

function Superseded({ detail, base, from }: { detail: AttemptDetail; base: GradingBase; from: ReviewFrom }) {
  const latest = detail.attempts.find((a) => a.id === detail.latestAttemptId);
  const latestNo = latest ? `#${String(latest.no)}` : "mới hơn";
  return (
    <Alert variant="warn">
      Học viên đã nộp lượt {latestNo}; lượt này không cần xác nhận nữa.{" "}
      <Link to={quizReviewHref(base, detail.latestAttemptId, from)}>Mở lượt {latestNo}</Link>
    </Alert>
  );
}

interface WorkspaceProps {
  detail: AttemptDetail;
  base: GradingBase;
  from: ReviewFrom;
  back: { label: string; to: string };
}

/**
 * The attempt on the left with a note field under each question, the general comment on the right; both belong to
 * one form. A first confirmation of an assessment moves on to the next queue item (or back to the queue when it is
 * empty); a practice comment and an edit stay on the page.
 */
function Workspace({ detail, base, from, back }: WorkspaceProps) {
  const locked = detail.reviewStatus === "superseded";
  const assess = detail.mode === "assessment";
  const review = useReviewAttempt(detail.id);
  const navigate = useNavigate();
  const formId = `${useId()}-review`;
  const [formError, setFormError] = useState<string | null>(null);
  const schema = useMemo(() => reviewFormSchema(detail.questions.map((q) => q.id)), [detail.questions]);
  const form = useForm<ReviewFormInput, unknown, ReviewBody>({
    resolver: zodResolver(schema),
    defaultValues: reviewDefaults(detail, detail.reviews.at(0) ?? null),
  });
  const { register, setError } = form;
  const { errors, isSubmitting } = form.formState;

  const onSubmit = form.handleSubmit(async (body) => {
    setFormError(null);
    try {
      const outcome = await review.mutateAsync(body);
      const message = reviewedMessage(detail.mode, outcome);
      if (!assess || outcome.updated) {
        toast.success(message);
      } else if (outcome.nextQueueItem) {
        const next = outcome.nextQueueItem;
        toast.success(`${message} Chuyển sang mục tiếp theo.`);
        void navigate(itemHref(base, next.kind, next.itemId));
      } else {
        toast.success(`${message} Đã xử lý hết hàng chờ.`);
        void navigate(queueHref(base));
      }
    } catch (error) {
      if (reviewErrorField(error) === "comment" && isApiError(error)) {
        setError("comment", { message: error.message });
      } else {
        setFormError(isApiError(error) ? error.message : NETWORK_ERROR_MESSAGE);
      }
    }
  });

  return (
    <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,440px)] items-start gap-5 max-[960px]:grid-cols-1">
      <div className="grid min-w-0 gap-5">
        {locked ? <Superseded detail={detail} base={base} from={from} /> : null}
        <AttemptFacts detail={detail} />
        <AttemptAnswers
          questions={detail.questions}
          formId={formId}
          register={register}
          errors={errors}
          locked={locked}
        />
        <ReviewHistory detail={detail} base={base} from={from} />
      </div>
      <ReviewForm
        detail={detail}
        formId={formId}
        register={register}
        errors={errors}
        submitting={isSubmitting}
        formError={formError}
        locked={locked}
        back={back}
        onSubmit={(e) => void onSubmit(e)}
      />
    </div>
  );
}

/** Quiz attempt review page: what the student answered against the key, the general comment and per-question notes. */
export function Component() {
  const { attemptId = "" } = useParams();
  const [params] = useSearchParams();
  const from = parseReviewFrom(params.get("from"));
  const base = useGradingBase();
  const navigate = useNavigate();
  const { data, error, isPending, refetch } = useQuizAttemptDetail(attemptId);
  useDocumentTitle(data ? `Quiz · ${data.student.name}` : "Chấm bài");

  // The server decides what a grader may open; the page only leaves.
  const denied = isApiError(error) && (error.status === 403 || error.status === 404);
  const deniedMessage = denied ? error.message : null;
  useEffect(() => {
    if (!deniedMessage) return;
    toast.error(deniedMessage, { id: deniedMessage });
    void navigate(queueHref(base), { replace: true });
  }, [deniedMessage, navigate, base]);

  const queueCrumb = { label: "Chấm bài", to: queueHref(base) };
  if (denied) return null;
  if (isPending) {
    return (
      <div className="grid gap-6" aria-busy="true">
        <span className="sr-only">Đang tải lượt làm bài</span>
        <Skeleton className="h-9 w-1/2" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (error) {
    return (
      <>
        <Crumbs items={[queueCrumb]} />
        <Alert variant="danger" role="alert">
          {error.message}
          <AlertActions>
            <Button variant="outline" size="sm" onClick={() => void refetch()}>
              Thử lại
            </Button>
          </AlertActions>
        </Alert>
      </>
    );
  }

  const { student, lesson } = data;
  // Practice is never queued: its page belongs to the class's quiz stats, as does any page opened from there.
  const fromQuizTab = from === "quiz" || data.mode === "practice";
  const quizTab = classQuizHref(base, data.class.id);
  const current: Crumb = { label: `${student.name} lượt #${String(data.no)}` };
  const crumbs: Crumb[] = fromQuizTab
    ? [
        { label: base === "/admin" ? "Lớp học" : "Lớp của tôi", to: classesHref(base) },
        { label: data.class.code, to: quizTab },
        current,
      ]
    : [queueCrumb, current];
  const back = fromQuizTab
    ? { label: "Về thống kê quiz", to: quizTab }
    : { label: "Về hàng chờ", to: queueHref(base) };
  const threshold = data.passPercent === null ? "" : `, đạt từ ${String(data.passPercent)}%`;
  return (
    <>
      <PageHead
        crumbs={crumbs}
        title={`${student.name} · ${shortTitle(lesson.title)}`}
        lede={`${data.class.code} · ${data.stage.name} · ${QUIZ_KIND_VI[data.mode]} · ${String(data.questionCount)} câu${threshold}`}
      />
      <Workspace
        key={`${data.id}-${data.reviews.at(0)?.id ?? "new"}`}
        detail={data}
        base={base}
        from={from}
        back={back}
      />
    </>
  );
}
