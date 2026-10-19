import { CircleAlert } from "lucide-react";
import { useEffect } from "react";
import { useNavigate, useParams } from "react-router";
import { toast } from "sonner";
import { isApiError } from "@/shared/api/errors";
import { useDocumentTitle } from "@/shared/hooks/use-document-title";
import { fmtDateTime } from "@/shared/lib/format";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { ToneBadge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card, CardHeader } from "@/shared/ui/card";
import { Crumbs } from "@/shared/ui/crumbs";
import { EmptyState } from "@/shared/ui/empty-state";
import { PageHead } from "@/shared/ui/page-head";
import { ProgressBar } from "@/shared/ui/progress-bar";
import { Skeleton } from "@/shared/ui/skeleton";
import { CourseTable } from "../components/course-table";
import { StageItems } from "../components/stage-items";
import { useLearnerSummary } from "../hooks/use-learner-summary";
import { NOT_MEMBER_MESSAGE } from "../model/messages";
import type { StageSummary } from "../model/schemas";
import { stageBadge } from "../model/stage-status";

const myClassesCrumb = { label: "Lớp của tôi", to: "/learn" };

function SummarySkeleton() {
  return (
    <div className="grid gap-6" aria-busy="true">
      <span className="sr-only">Đang tải tổng kết khóa học</span>
      <Skeleton className="h-9 w-1/2" />
      <Skeleton className="h-24 w-full" />
      <Skeleton className="h-64 w-full" />
    </div>
  );
}

function Missing({ stage }: { stage: StageSummary }) {
  return (
    <div className="border-b border-line px-6 py-4 max-[720px]:px-4">
      <h3 className="flex items-center gap-1.5 text-sm">
        <CircleAlert aria-hidden="true" className="size-4 text-warn" />
        Còn thiếu
      </h3>
      <ul className="mt-2 list-disc pl-5 text-sm text-ink-2">
        {stage.missing.map((m) => (
          <li key={m.lessonId}>
            {m.title} <span className="text-ink-3">· {m.unitTitle}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function Review({ stage }: { stage: StageSummary }) {
  const { review } = stage;
  return (
    <section className="border-t border-line px-6 py-4 max-[720px]:px-4">
      <h3 className="text-sm">Nhận xét của giảng viên</h3>
      {review ? (
        <>
          <p className="mt-2 whitespace-pre-line text-ink">{review.body}</p>
          <p className="mt-1 text-sm text-ink-3">
            {review.authorName} · {fmtDateTime(review.updatedAt)}
          </p>
        </>
      ) : (
        <p className="mt-2 text-sm text-ink-3">Chưa có nhận xét của giảng viên cho chặng này.</p>
      )}
    </section>
  );
}

function StageCard({
  stage,
  index,
  lessonLink,
}: {
  stage: StageSummary;
  index: number;
  lessonLink: ((lessonId: string) => string) | undefined;
}) {
  const badge = stageBadge(stage);
  return (
    <Card>
      <CardHeader>
        <h2>
          {index + 1}. {stage.name}
        </h2>
        <div className="flex flex-wrap items-center gap-3">
          <span className="text-sm text-ink-3">
            {stage.done}/{stage.required} học liệu bắt buộc
          </span>
          <ToneBadge tone={badge.tone}>{badge.label}</ToneBadge>
        </div>
      </CardHeader>
      {stage.complete || stage.missing.length === 0 ? null : <Missing stage={stage} />}
      <StageItems units={stage.units} lessonLink={lessonLink} />
      <Review stage={stage} />
    </Card>
  );
}

export function Component() {
  const { classId = "" } = useParams();
  const navigate = useNavigate();
  const { data, error, isPending, refetch } = useLearnerSummary(classId);
  useDocumentTitle(data ? `Tổng kết ${data.class.code}` : "Tổng kết khóa học");

  // 403 when the student is not (or no longer) learning the class, 404 when there is no such class.
  const notMember = isApiError(error) && (error.status === 403 || error.status === 404);
  useEffect(() => {
    if (!notMember) return;
    toast.error(NOT_MEMBER_MESSAGE, { id: NOT_MEMBER_MESSAGE });
    void navigate("/learn", { replace: true });
  }, [notMember, navigate]);

  if (notMember) return null;
  if (isPending) return <SummarySkeleton />;
  if (error) {
    return (
      <>
        <Crumbs items={[myClassesCrumb]} />
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

  const { class: cls } = data;
  const classPath = `/learn/classes/${cls.id}`;
  // A draft class cannot open lessons yet (the API answers 409), so its lessons are not links.
  const lessonLink = cls.status === "draft" ? undefined : (lessonId: string) => `${classPath}/lessons/${lessonId}`;

  return (
    <>
      <PageHead
        crumbs={[myClassesCrumb, { label: cls.code, to: classPath }, { label: "Tổng kết" }]}
        title="Tổng kết khóa học"
        lede={cls.name}
      />
      <div className="grid gap-6">
        <Card className="p-6 max-[720px]:px-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 className="text-xs font-medium tracking-[1px] text-ink-3 uppercase">Toàn khóa</h2>
              <div className="text-sm text-ink-3">
                {data.requiredDone}/{data.requiredTotal} học liệu bắt buộc. Học liệu không bắt buộc không tính vào %.
              </div>
              <div className="text-sm text-ink">
                <b className="font-semibold tabular-nums">
                  {data.course.stagesComplete}/{data.course.stagesTotal}
                </b>{" "}
                chặng hoàn thành
              </div>
            </div>
            <ProgressBar
              size="lg"
              value={data.coursePercent}
              className="min-w-[260px] max-[720px]:w-full max-[720px]:min-w-0"
            />
          </div>
          {data.course.stages.length > 0 ? (
            <div className="-mx-6 mt-4 -mb-6 border-t border-line max-[720px]:-mx-4">
              <CourseTable course={data.course} stages={data.stages} />
            </div>
          ) : null}
        </Card>
        {data.stages.length > 0 ? (
          data.stages.map((stage, i) => (
            <StageCard key={stage.stageVersionId} stage={stage} index={i} lessonLink={lessonLink} />
          ))
        ) : (
          <Card>
            <EmptyState title="Chưa có học liệu" text="Giảng viên chưa thêm học liệu cho lớp này." />
          </Card>
        )}
      </div>
    </>
  );
}
