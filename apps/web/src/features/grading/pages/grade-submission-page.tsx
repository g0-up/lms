import { useEffect } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { toast } from "sonner";
import { isApiError } from "@/shared/api/errors";
import { shortTitle } from "@/shared/domain";
import { useDocumentTitle } from "@/shared/hooks/use-document-title";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Card, CardBody } from "@/shared/ui/card";
import { Crumbs } from "@/shared/ui/crumbs";
import { MarkdownContent } from "@/shared/ui/markdown-content";
import { PageHead } from "@/shared/ui/page-head";
import { Skeleton } from "@/shared/ui/skeleton";
import { GradeForm } from "../components/grade-form";
import { GradeHistory } from "../components/grade-history";
import { SubmissionFacts } from "../components/submission-facts";
import { useGradingBase } from "../hooks/use-grading-base";
import { useSubmission } from "../hooks/use-submission";
import { gradeHref, queueHref } from "../model/links";
import type { SubmissionDetail } from "../model/schemas";

function Brief({ title, html }: { title: string; html: string }) {
  return (
    <Card>
      <details className="group">
        <summary className="flex min-h-11 cursor-pointer list-none flex-wrap items-center justify-between gap-3 px-6 py-4 max-[720px]:px-4 [&::-webkit-details-marker]:hidden">
          <h2 className="text-md">Đề bài: {title}</h2>
          <span className="text-sm text-ink-3 group-open:hidden">Mở để xem</span>
        </summary>
        <CardBody className="border-t border-line">
          <MarkdownContent html={html} />
        </CardBody>
      </details>
    </Card>
  );
}

function NotLatest({ detail }: { detail: SubmissionDetail }) {
  const base = useGradingBase();
  const latest = detail.history.find((s) => s.id === detail.latestSubmissionId);
  const latestNo = latest ? `#${String(latest.no)}` : "mới hơn";
  return (
    <Alert variant="warn">
      Đây là lần nộp #{detail.submission.no}; học viên đã nộp lần {latestNo}. Chỉ chấm lần nộp mới nhất.{" "}
      <Link to={gradeHref(base, detail.latestSubmissionId)}>Mở lần nộp {latestNo}</Link>
    </Alert>
  );
}

/** FR-62 grading page: the submission and its history on the left, the rubric form on the right. */
export function Component() {
  const { submissionId = "" } = useParams();
  const base = useGradingBase();
  const navigate = useNavigate();
  const { data, error, isPending, refetch } = useSubmission(submissionId);
  useDocumentTitle(data ? `Chấm bài · ${data.student.name}` : "Chấm bài");

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
        <span className="sr-only">Đang tải bài nộp</span>
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

  const { submission, student, lesson } = data;
  const title = shortTitle(lesson.title);
  return (
    <>
      <PageHead
        crumbs={[queueCrumb, { label: `${student.name} #${String(submission.no)}` }]}
        title={`${student.name} · ${title}`}
        lede={`${data.class.code} · ${data.stage.name} · ${String(lesson.criteria.length)} tiêu chí, tổng ${String(lesson.maxPoints)} điểm`}
      />
      <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,440px)] items-start gap-5 max-[960px]:grid-cols-1">
        <div className="grid min-w-0 gap-5">
          {data.isLatest ? null : <NotLatest detail={data} />}
          <SubmissionFacts detail={data} />
          {lesson.briefHtml ? <Brief title={title} html={lesson.briefHtml} /> : null}
          <GradeHistory detail={data} base={base} />
        </div>
        <GradeForm key={`${submission.id}-${submission.grade?.id ?? "new"}`} detail={data} base={base} />
      </div>
    </>
  );
}
