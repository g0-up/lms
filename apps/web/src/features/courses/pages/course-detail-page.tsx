import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useState, type ReactNode } from "react";
import { useParams, useSearchParams } from "react-router";
import { toast } from "sonner";
import { draftOf, LockNote, moveItem, selectVersion, stagesQuery, VersionCard } from "@/features/stages";
import { isApiError, NETWORK_ERROR_MESSAGE } from "@/shared/api/errors";
import { useDocumentTitle } from "@/shared/hooks/use-document-title";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Badge, StatusBadge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card, CardHeader, CardTitle } from "@/shared/ui/card";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Crumbs } from "@/shared/ui/crumbs";
import { PageHead } from "@/shared/ui/page-head";
import { Skeleton } from "@/shared/ui/skeleton";
import { AddStageDialog } from "../components/add-stage-dialog";
import { ClassesOfVersionCard } from "../components/classes-of-version-card";
import { CourseActionsBar } from "../components/course-actions-bar";
import { CourseStageList } from "../components/course-stage-list";
import { useCourse } from "../hooks/use-course";
import { coursePath, useCourseVersionActions } from "../hooks/use-course-mutations";
import { useCourseStageMutations } from "../hooks/use-course-stage-mutations";
import { useCourseVersion } from "../hooks/use-course-version";
import {
  addStageOptions,
  coursePublishBlocker,
  publishedVersionId,
  removeAt,
  replaceAt,
  stageVersionStatuses,
  type StageOption,
} from "../model/publish-blocker";
import type { CourseDetail, CourseVersion } from "../model/schemas";

const LEDE =
  "Nhân bản khóa học là nhân bản nông: phiên bản mới trỏ lại đúng các phiên bản chặng cũ cho đến khi bạn đổi.";
const NO_STAGE_LEFT = "Không còn chặng đã phát hành nào để thêm.";

const coursesCrumb = { label: "Khóa học", to: "/admin/courses" };

function DetailSkeleton() {
  return (
    <div className="grid gap-6" aria-busy="true">
      <span className="sr-only">Đang tải khóa học</span>
      <Skeleton className="h-9 w-1/2" />
      <Skeleton className="h-20 w-full" />
      <Skeleton className="h-64 w-full" />
    </div>
  );
}

function RetryAlert({ children, onRetry }: { children: ReactNode; onRetry: () => void }) {
  return (
    <Alert variant="danger" role="alert">
      {children}
      <AlertActions>
        <Button variant="outline" size="sm" onClick={onRetry}>
          Thử lại
        </Button>
      </AlertActions>
    </Alert>
  );
}

/** Message for a failed publish or archive, shown inside its confirm dialog. */
function actionError(error: Error | null): ReactNode {
  if (!error) return null;
  return (
    <Alert variant="danger" role="alert" className="mt-3">
      {isApiError(error) ? error.message : NETWORK_ERROR_MESSAGE}
    </Alert>
  );
}

export function Component() {
  const { courseId = "" } = useParams();
  const [searchParams] = useSearchParams();
  const course = useCourse(courseId);
  const current = course.data ? selectVersion(course.data.versions, searchParams.get("v")) : undefined;
  const version = useCourseVersion(current?.id);
  useDocumentTitle(course.data && current ? `${course.data.name} v${String(current.versionNo)}` : "Khóa học");

  if (course.isPending) return <DetailSkeleton />;
  if (course.isError) {
    return (
      <>
        <Crumbs items={[coursesCrumb]} />
        <RetryAlert onRetry={() => void course.refetch()}>{course.error.message}</RetryAlert>
      </>
    );
  }

  const detail = course.data;
  return (
    <>
      <PageHead
        crumbs={[coursesCrumb, { label: detail.name }]}
        title={detail.name}
        badges={<Badge variant="muted">{detail.code}</Badge>}
        lede={LEDE}
      />
      {current ? (
        <div className="grid gap-6">
          <VersionCard versions={detail.versions} current={current} hrefFor={(vid) => `${coursePath(detail.id)}?v=${vid}`} />
          {version.isPending ? (
            <Skeleton className="h-64 w-full" aria-busy="true" />
          ) : version.isError ? (
            <RetryAlert onRetry={() => void version.refetch()}>Không tải được phiên bản này.</RetryAlert>
          ) : (
            <VersionSection key={version.data.id} detail={detail} version={version.data} />
          )}
        </div>
      ) : (
        <Alert variant="warn">Khóa học này chưa có phiên bản nào.</Alert>
      )}
    </>
  );
}

type Dialog = { kind: "publish" | "archive" | "delete" } | { kind: "add"; options: StageOption[] } | null;

/** Stages, actions and classes of the version being viewed; remounts when the version changes. */
function VersionSection({ detail, version }: { detail: CourseDetail; version: CourseVersion }) {
  const queryClient = useQueryClient();
  const actions = useCourseVersionActions(detail.id);
  const setStages = useCourseStageMutations(detail.id, version.id);
  // Stage version statuses (for the publish blocker) and swap targets come from the stage list.
  const stageList = useQuery(stagesQuery);
  const [dialog, setDialog] = useState<Dialog>(null);
  const editable = version.status === "draft";
  const n = version.versionNo;
  const statusOf = stageVersionStatuses(stageList.data);
  const publishBlocker = coursePublishBlocker(version, statusOf);
  const draft = draftOf(detail.versions);
  const ids = version.stages.map((s) => s.stageVersionId);
  const close = () => {
    setDialog(null);
  };

  const openPublishOrArchive = (kind: "publish" | "archive") => {
    (kind === "publish" ? actions.publish : actions.archive).reset();
    setDialog({ kind });
  };

  async function openAdd() {
    try {
      const options = addStageOptions(await queryClient.query(stagesQuery), version.stages);
      if (options.length === 0) toast.error(NO_STAGE_LEFT);
      else setDialog({ kind: "add", options });
    } catch (error) {
      toast.error(isApiError(error) ? error.message : NETWORK_ERROR_MESSAGE);
    }
  }

  async function swap(index: number) {
    const stage = version.stages[index];
    if (!stage.latestPublishedNo) return;
    try {
      // The cached list may predate the newer stage version; refetch once before giving up.
      let target = publishedVersionId(await queryClient.query(stagesQuery), stage.stageId, stage.latestPublishedNo);
      target ??= publishedVersionId(await queryClient.query({ ...stagesQuery, staleTime: 0 }), stage.stageId, stage.latestPublishedNo);
      if (!target) {
        toast.error("Không tìm thấy phiên bản mới của chặng. Tải lại trang rồi thử lại.");
        return;
      }
      setStages.mutate({ kind: "swap", stageVersionIds: replaceAt(version.stages, index, target) });
    } catch (error) {
      toast.error(isApiError(error) ? error.message : NETWORK_ERROR_MESSAGE);
    }
  }

  return (
    <>
      {editable && publishBlocker ? <Alert variant="warn">Chưa phát hành được: {publishBlocker}</Alert> : null}
      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center gap-3">
            <CardTitle>Chặng trong v{n}</CardTitle>
            <StatusBadge status={version.status} />
          </div>
          {editable ? (
            <Button variant="outline" size="sm" onClick={() => void openAdd()}>
              <Plus aria-hidden="true" /> Thêm chặng
            </Button>
          ) : (
            <LockNote />
          )}
        </CardHeader>
        <CourseStageList
          stages={version.stages}
          editable={editable}
          statusOf={statusOf}
          onSwap={(index) => void swap(index)}
          onMove={(index, delta) => {
            setStages.mutate({ kind: "move", stageVersionIds: moveItem(ids, index, delta) });
          }}
          onRemove={(index) => {
            setStages.mutate({ kind: "remove", stageVersionIds: removeAt(version.stages, index) });
          }}
        />
        <CourseActionsBar
          version={version}
          publishBlocker={publishBlocker}
          draft={draft}
          draftHref={draft ? `${coursePath(detail.id)}?v=${draft.id}` : ""}
          cloning={actions.clone.isPending}
          onPublish={() => {
            openPublishOrArchive("publish");
          }}
          onArchive={() => {
            openPublishOrArchive("archive");
          }}
          onDelete={() => {
            setDialog({ kind: "delete" });
          }}
          onClone={() => {
            actions.clone.mutate(version.id);
          }}
        />
      </Card>
      <ClassesOfVersionCard versionNo={n} classes={version.classes} />

      <ConfirmDialog
        open={dialog?.kind === "publish"}
        onOpenChange={close}
        title={`Phát hành ${detail.name} v${String(n)}?`}
        text={
          <>
            Sau khi phát hành, danh sách và thứ tự {version.stages.length} chặng không sửa được. Lớp mới có thể gắn với
            phiên bản này.
            {actionError(actions.publish.error)}
          </>
        }
        confirm="Phát hành"
        loading={actions.publish.isPending}
        onConfirm={() => {
          actions.publish.mutate(version.id, { onSuccess: close });
        }}
      />
      <ConfirmDialog
        open={dialog?.kind === "archive"}
        onOpenChange={close}
        title={`Lưu trữ v${String(n)}?`}
        text={
          <>
            Lớp mới không gắn được phiên bản lưu trữ; các lớp đang dùng vẫn hoạt động bình thường.
            {actionError(actions.archive.error)}
          </>
        }
        confirm="Lưu trữ"
        loading={actions.archive.isPending}
        onConfirm={() => {
          actions.archive.mutate(version.id, { onSuccess: close });
        }}
      />
      <ConfirmDialog
        open={dialog?.kind === "delete"}
        onOpenChange={close}
        danger
        title={`Xóa ${detail.name} v${String(n)}?`}
        text="Không hoàn tác được. Các phiên bản chặng không bị ảnh hưởng."
        confirm="Xóa phiên bản"
        loading={actions.remove.isPending}
        onConfirm={() => {
          actions.remove.mutate(version.id, { onSettled: close });
        }}
      />
      <AddStageDialog
        open={dialog?.kind === "add"}
        onOpenChange={close}
        options={dialog?.kind === "add" ? dialog.options : []}
        onAdd={(stageVersionId) => setStages.mutateAsync({ kind: "add", stageVersionIds: [...ids, stageVersionId] })}
      />
    </>
  );
}
