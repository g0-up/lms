import { useEffect } from "react";
import { useLocation, useNavigate, useParams } from "react-router";
import { toast } from "sonner";
import { isApiError } from "@/shared/api/errors";
import { useDocumentTitle } from "@/shared/hooks/use-document-title";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { StatusBadge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card } from "@/shared/ui/card";
import { Crumbs } from "@/shared/ui/crumbs";
import { EmptyState } from "@/shared/ui/empty-state";
import { PageHead } from "@/shared/ui/page-head";
import { ProgressBar } from "@/shared/ui/progress-bar";
import { Skeleton } from "@/shared/ui/skeleton";
import { Roadmap } from "../components/roadmap";
import { useMyClass } from "../hooks/use-my-class";
import { useToggleCompletion } from "../hooks/use-toggle-completion";
import { NOT_MEMBER_MESSAGE } from "../model/messages";

const READ_ONLY_COPY = {
  ended: "Lớp đã kết thúc: bạn vẫn xem được học liệu nhưng không tích hoàn thành được nữa.",
  draft: "Lớp chưa bắt đầu.",
} as const;

const myClassesCrumb = { label: "Lớp của tôi", to: "/learn" };

function RoadmapSkeleton() {
  return (
    <div className="grid gap-6" aria-busy="true">
      <span className="sr-only">Đang tải lộ trình</span>
      <Skeleton className="h-9 w-1/2" />
      <Skeleton className="h-24 w-full" />
      <Skeleton className="h-64 w-full" />
    </div>
  );
}

export function Component() {
  const { classId = "" } = useParams();
  const navigate = useNavigate();
  const { hash } = useLocation();
  const { data, error, isPending, refetch } = useMyClass(classId);
  const toggle = useToggleCompletion(classId);
  useDocumentTitle(data?.class.code ?? "Lộ trình lớp");

  const notMember = isApiError(error) && error.status === 404;
  useEffect(() => {
    if (!notMember) return;
    toast.error(NOT_MEMBER_MESSAGE, { id: NOT_MEMBER_MESSAGE });
    void navigate("/learn", { replace: true });
  }, [notMember, navigate]);

  // The lesson page's stage crumb links to `#stage-{id}`; scroll there once the roadmap is shown.
  const loaded = data !== undefined;
  useEffect(() => {
    if (loaded && hash) document.getElementById(decodeURIComponent(hash.slice(1)))?.scrollIntoView({ block: "start" });
  }, [loaded, hash]);

  if (notMember) return null;
  if (isPending) return <RoadmapSkeleton />;
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
  const hasLessons = data.stages.some((s) => s.lessons.length > 0);
  const onToggle = (lessonId: string, completed: boolean) => {
    toggle.mutate({ lessonId, completed });
  };

  return (
    <>
      <PageHead
        crumbs={[myClassesCrumb, { label: cls.code }]}
        title={cls.name}
        badges={<StatusBadge status={cls.status} />}
        lede={`${cls.courseName} v${String(cls.courseVersionNo)} · Giảng viên ${cls.teacherName}`}
      />
      <div className="grid gap-6">
        <Card className="p-6 max-[720px]:px-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div className="text-xs font-medium tracking-[1px] text-ink-3 uppercase">Tiến độ toàn khóa</div>
              <div className="text-sm text-ink-3">
                {data.requiredDone}/{data.requiredTotal} học liệu bắt buộc. Học liệu không bắt buộc không tính vào %.
              </div>
            </div>
            <ProgressBar size="lg" value={data.percent} className="min-w-[260px] max-[720px]:w-full max-[720px]:min-w-0" />
          </div>
          {data.readOnly && data.readOnlyReason ? (
            <Alert variant="info" className="mt-4">
              {READ_ONLY_COPY[data.readOnlyReason]}
            </Alert>
          ) : null}
        </Card>
        {hasLessons ? (
          <Roadmap roadmap={data} onToggle={onToggle} />
        ) : (
          <Card>
            <EmptyState title="Chưa có học liệu" text="Giảng viên chưa thêm học liệu cho lớp này." />
          </Card>
        )}
      </div>
    </>
  );
}
