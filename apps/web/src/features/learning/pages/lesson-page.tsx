import { useEffect } from "react";
import { Link, useNavigate, useParams } from "react-router";
import { toast } from "sonner";
import { hasCode, isApiError } from "@/shared/api/errors";
import { useDocumentTitle } from "@/shared/hooks/use-document-title";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card } from "@/shared/ui/card";
import { Crumbs, type Crumb } from "@/shared/ui/crumbs";
import { PageHead } from "@/shared/ui/page-head";
import { Skeleton } from "@/shared/ui/skeleton";
import { CompleteBar } from "../components/complete-bar";
import { LessonSideList } from "../components/lesson-side-list";
import { LessonViewer } from "../components/lesson-viewer";
import { useLesson } from "../hooks/use-lesson";
import { useMyClass } from "../hooks/use-my-class";
import { useToggleCompletion } from "../hooks/use-toggle-completion";
import { LESSON_NOT_IN_COURSE_MESSAGE, NOT_MEMBER_MESSAGE } from "../model/messages";
import { flattenLessons } from "../model/neighbors";
import { tickBlocker } from "../model/tick-rule";

const is404 = (error: Error | null) => isApiError(error) && error.status === 404;

function ViewerSkeleton() {
  return (
    <Card aria-busy="true">
      <span className="sr-only">Đang tải học liệu</span>
      <Skeleton className="aspect-video w-full" />
      <div className="flex justify-between gap-3 border-t border-line px-6 py-4">
        <Skeleton className="h-9 w-48" />
        <Skeleton className="h-11 w-44 rounded-full" />
      </div>
    </Card>
  );
}

function RetryAlert({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <Alert variant="danger" role="alert">
      {message}
      <AlertActions>
        <Button variant="outline" size="sm" onClick={onRetry}>
          Thử lại
        </Button>
      </AlertActions>
    </Alert>
  );
}

export function Component() {
  const { classId = "", lessonId = "" } = useParams();
  const navigate = useNavigate();
  const roadmapQuery = useMyClass(classId);
  const lessonQuery = useLesson(classId, lessonId);
  const toggle = useToggleCompletion(classId);

  const roadmap = roadmapQuery.data;
  const page = lessonQuery.data;
  // The roadmap is usually cached already: head and side list show while the lesson loads, so
  // focus lands on the new h1 right after "Bài tiếp".
  const entry = roadmap ? flattenLessons(roadmap.stages).find((f) => f.lesson.id === lessonId) : undefined;
  const title = page?.lesson.title ?? entry?.lesson.title;
  useDocumentTitle(title ?? "Học liệu");

  const classGone = is404(roadmapQuery.error);
  const lessonGone = !classGone && roadmapQuery.isSuccess && is404(lessonQuery.error);
  useEffect(() => {
    if (classGone) {
      toast.error(NOT_MEMBER_MESSAGE, { id: NOT_MEMBER_MESSAGE });
      void navigate("/learn", { replace: true });
    } else if (lessonGone) {
      toast.error(LESSON_NOT_IN_COURSE_MESSAGE, { id: LESSON_NOT_IN_COURSE_MESSAGE });
      void navigate(`/learn/classes/${classId}`, { replace: true });
    }
  }, [classGone, lessonGone, classId, navigate]);

  if (classGone || lessonGone) return null;
  if (roadmapQuery.isPending) {
    return (
      <div className="grid gap-6">
        <Skeleton className="h-9 w-1/2" />
        <ViewerSkeleton />
      </div>
    );
  }
  if (roadmapQuery.isError) {
    return (
      <>
        <Crumbs items={[{ label: "Lớp của tôi", to: "/learn" }]} />
        <RetryAlert message={roadmapQuery.error.message} onRetry={() => void roadmapQuery.refetch()} />
      </>
    );
  }

  const classHref = `/learn/classes/${classId}`;
  const stage = page?.lesson.stage ?? entry?.stage;
  const crumbs: Crumb[] = [
    { label: "Lớp của tôi", to: "/learn" },
    { label: roadmapQuery.data.class.code, to: classHref },
    ...(stage ? [{ label: stage.name, to: `${classHref}#stage-${stage.id}` }] : []),
    { label: title ?? "Học liệu" },
  ];
  const required = page?.lesson.required ?? entry?.lesson.required ?? true;
  const head = (
    <PageHead
      crumbs={crumbs}
      title={title ?? "Học liệu"}
      badges={required ? null : <Badge variant="subtle">Không bắt buộc</Badge>}
    />
  );

  let viewer;
  if (lessonQuery.isPending) {
    viewer = <ViewerSkeleton />;
  } else if (lessonQuery.isError) {
    const error = lessonQuery.error;
    // A draft class answers 409 here: its students see the roadmap but no lesson content yet.
    viewer = hasCode(error, "INVALID_TRANSITION") ? (
      <Alert variant="info" role="status">
        {error.message}
        <AlertActions>
          <Button asChild variant="outline" size="sm">
            <Link to={classHref}>Về lộ trình</Link>
          </Button>
        </AlertActions>
      </Alert>
    ) : (
      <RetryAlert message={error.message} onRetry={() => void lessonQuery.refetch()} />
    );
  } else {
    const lesson = lessonQuery.data;
    // Only active and ended classes reach this point (a draft answers 409 above), so a read-only
    // lesson the cached roadmap does not explain yet means the class has just ended.
    const readOnlyReason = roadmapQuery.data.readOnlyReason ?? (lesson.readOnly ? "ended" : undefined);
    viewer = (
      <Card className="min-w-0">
        <LessonViewer key={lesson.lesson.id} page={lesson} />
        <CompleteBar
          classId={classId}
          prev={lesson.prev}
          next={lesson.next}
          completedAt={lesson.progress.completedAt}
          // Opening this page recorded the first open, so only the class state can block here.
          blocker={tickBlocker({ readOnlyReason, opened: true })}
          onToggle={(completed) => {
            toggle.mutate({ lessonId, completed });
          }}
        />
      </Card>
    );
  }

  const sideStage = stage && roadmapQuery.data.stages.find((s) => s.id === stage.id);
  return (
    <>
      {head}
      <div className="grid grid-cols-[minmax(0,1fr)_320px] items-start gap-6 max-[960px]:grid-cols-1">
        {viewer}
        {sideStage && !lessonQuery.isError ? (
          <LessonSideList classId={classId} stage={sideStage} currentLessonId={lessonId} />
        ) : null}
      </div>
    </>
  );
}
