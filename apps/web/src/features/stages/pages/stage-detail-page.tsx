import { Plus } from "lucide-react";
import { useState, type ReactNode } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { hasCode, isApiError, NETWORK_ERROR_MESSAGE } from "@/shared/api/errors";
import { useDocumentTitle } from "@/shared/hooks/use-document-title";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Badge, StatusBadge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card, CardHeader, CardTitle } from "@/shared/ui/card";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Crumbs } from "@/shared/ui/crumbs";
import { PageHead } from "@/shared/ui/page-head";
import { Skeleton } from "@/shared/ui/skeleton";
import { ApplyDialog } from "../components/apply-dialog";
import { LessonFormDialog } from "../components/lesson-form-dialog";
import { LessonList } from "../components/lesson-list";
import { LockNote } from "../components/lock-note";
import { OutdatedCoursesCard } from "../components/outdated-courses-card";
import { StageActionsBar } from "../components/stage-actions-bar";
import { UsedByCard } from "../components/used-by-card";
import { VersionCard } from "../components/version-card";
import { useLessonMutations } from "../hooks/use-lesson-mutations";
import { useStage } from "../hooks/use-stage";
import { stagePath, useStageVersionActions } from "../hooks/use-stage-mutations";
import { useStageVersion } from "../hooks/use-stage-version";
import { lessonEditorPath } from "../model/lesson-form";
import { moveItem } from "../model/order";
import type { Lesson, OutdatedCourse, StageDetail, StageVersion } from "../model/schemas";
import { draftOf, latestPublished, selectVersion } from "../model/select-version";
import { PUBLISH_NEEDS_LESSON } from "../model/stage-rules";

const LEDE =
  "Phiên bản đã phát hành là bất biến; mọi thay đổi đi qua một bản nháp mới rồi được áp dụng cho khóa học bằng một thao tác.";

const stagesCrumb = { label: "Chặng", to: "/admin/stages" };

function DetailSkeleton() {
  return (
    <div className="grid gap-6" aria-busy="true">
      <span className="sr-only">Đang tải chặng</span>
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
  let message = isApiError(error) ? error.message : NETWORK_ERROR_MESSAGE;
  if (hasCode(error, "VALIDATION_FAILED")) message = `${PUBLISH_NEEDS_LESSON}.`;
  return (
    <Alert variant="danger" role="alert" className="mt-3">
      {message}
    </Alert>
  );
}

export function Component() {
  const { stageId = "" } = useParams();
  const [searchParams] = useSearchParams();
  const stage = useStage(stageId);
  const current = stage.data ? selectVersion(stage.data.versions, searchParams.get("v")) : undefined;
  const version = useStageVersion(current?.id);
  useDocumentTitle(stage.data && current ? `${stage.data.name} v${String(current.versionNo)}` : "Chặng");

  if (stage.isPending) return <DetailSkeleton />;
  if (stage.isError) {
    return (
      <>
        <Crumbs items={[stagesCrumb]} />
        <RetryAlert onRetry={() => void stage.refetch()}>{stage.error.message}</RetryAlert>
      </>
    );
  }

  const detail = stage.data;
  return (
    <>
      <PageHead
        crumbs={[stagesCrumb, { label: detail.name }]}
        title={detail.name}
        badges={<Badge variant="muted">{detail.code}</Badge>}
        lede={LEDE}
      />
      {current ? (
        <div className="grid gap-6">
          <VersionCard versions={detail.versions} current={current} hrefFor={(vid) => `${stagePath(detail.id)}?v=${vid}`} />
          {version.isPending ? (
            <Skeleton className="h-64 w-full" aria-busy="true" />
          ) : version.isError ? (
            <RetryAlert onRetry={() => void version.refetch()}>Không tải được phiên bản này.</RetryAlert>
          ) : (
            <VersionSection key={version.data.id} detail={detail} version={version.data} />
          )}
        </div>
      ) : (
        <Alert variant="warn">Chặng này chưa có phiên bản nào.</Alert>
      )}
    </>
  );
}

type Dialog =
  | { kind: "publish" | "archive" | "delete" }
  | { kind: "lesson"; lesson?: Lesson }
  | { kind: "delete-lesson"; lesson: Lesson }
  | null;

/** Lessons, actions and references of the version being viewed; remounts when the version changes. */
function VersionSection({ detail, version }: { detail: StageDetail; version: StageVersion }) {
  const actions = useStageVersionActions(detail.id);
  const lessonActions = useLessonMutations(detail.id, version.id);
  const navigate = useNavigate();
  const [dialog, setDialog] = useState<Dialog>(null);
  const [applying, setApplying] = useState<OutdatedCourse | null>(null);
  const editable = version.status === "draft";
  const n = version.versionNo;
  const k = version.lessons.length;
  const draft = draftOf(detail.versions);
  const isLatestPublished = latestPublished(detail.versions)?.id === version.id;
  const anyPublishedUse = detail.usedBy.some((u) => u.status === "published");
  const close = () => {
    setDialog(null);
  };

  const openPublishOrArchive = (kind: "publish" | "archive") => {
    (kind === "publish" ? actions.publish : actions.archive).reset();
    setDialog({ kind });
  };

  return (
    <>
      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center gap-3">
            <CardTitle>Học liệu của v{n}</CardTitle>
            <StatusBadge status={version.status} />
          </div>
          {editable ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                setDialog({ kind: "lesson" });
              }}
            >
              <Plus aria-hidden="true" /> Thêm học liệu
            </Button>
          ) : (
            <LockNote />
          )}
        </CardHeader>
        <LessonList
          lessons={version.lessons}
          editable={editable}
          onMove={(index, delta) => {
            lessonActions.reorder.mutate(moveItem(version.lessons, index, delta).map((l) => l.id));
          }}
          onEdit={(lesson) => {
            if (lesson.type === "markdown") void navigate(lessonEditorPath(detail.id, version.id, lesson.id));
            else setDialog({ kind: "lesson", lesson });
          }}
          onDelete={(lesson) => {
            setDialog({ kind: "delete-lesson", lesson });
          }}
        />
        <StageActionsBar
          version={version}
          draft={draft}
          draftHref={draft ? `${stagePath(detail.id)}?v=${draft.id}` : ""}
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

      {isLatestPublished && detail.outdatedCourses.length > 0 ? (
        <OutdatedCoursesCard stageName={detail.name} courses={detail.outdatedCourses} onApply={setApplying} />
      ) : null}
      {isLatestPublished && detail.outdatedCourses.length === 0 && anyPublishedUse ? (
        <Alert variant="ok">Mọi khóa học đã phát hành đều dùng phiên bản mới nhất của chặng này.</Alert>
      ) : null}
      <UsedByCard versionNo={n} usedBy={version.usedBy} />

      <ConfirmDialog
        open={dialog?.kind === "publish"}
        onOpenChange={close}
        title={`Phát hành ${detail.name} v${String(n)}?`}
        text={
          <>
            Sau khi phát hành, phiên bản này và {k} học liệu của nó không sửa được nữa. Khóa học đang dùng phiên bản cũ sẽ
            không tự cập nhật; bạn áp dụng ở bước tiếp theo.
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
            Phiên bản lưu trữ không gắn mới vào khóa học được, nhưng các khóa học và lớp đang dùng vẫn hoạt động bình
            thường.
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
        text={
          editable
            ? `Bản nháp và ${String(k)} học liệu trong đó sẽ bị xóa. Không hoàn tác được.`
            : "Phiên bản này không được khóa học nào tham chiếu. Xóa sẽ không hoàn tác được."
        }
        confirm="Xóa phiên bản"
        loading={actions.remove.isPending}
        onConfirm={() => {
          actions.remove.mutate(version.id, { onSettled: close });
        }}
      />
      <LessonFormDialog
        open={dialog?.kind === "lesson"}
        onOpenChange={close}
        lesson={dialog?.kind === "lesson" ? dialog.lesson : undefined}
        onSave={(values) =>
          lessonActions.save.mutateAsync({ lessonId: dialog?.kind === "lesson" ? dialog.lesson?.id : undefined, values })
        }
        onContinueMarkdown={(draft) => {
          close();
          void navigate(lessonEditorPath(detail.id, version.id), { state: draft });
        }}
      />
      <ConfirmDialog
        open={dialog?.kind === "delete-lesson"}
        onOpenChange={close}
        danger
        title={dialog?.kind === "delete-lesson" ? `Xóa "${dialog.lesson.title}"?` : ""}
        text="Học liệu bị xóa khỏi bản nháp này. Các phiên bản đã phát hành không bị ảnh hưởng."
        confirm="Xóa học liệu"
        loading={lessonActions.remove.isPending}
        onConfirm={() => {
          if (dialog?.kind === "delete-lesson") lessonActions.remove.mutate(dialog.lesson, { onSettled: close });
        }}
      />
      <ApplyDialog
        stageId={detail.id}
        stageName={detail.name}
        course={applying}
        onOpenChange={(open) => {
          if (!open) setApplying(null);
        }}
      />
    </>
  );
}
