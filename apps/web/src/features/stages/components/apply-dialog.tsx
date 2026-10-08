import { useState } from "react";
import { toast } from "sonner";
import { NETWORK_ERROR_MESSAGE, isApiError } from "@/shared/api/errors";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Dialog, DialogBody, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/shared/ui/dialog";
import { PendingLabel } from "@/shared/ui/pending-label";
import { Skeleton } from "@/shared/ui/skeleton";
import { useCourseVersionSummary } from "../hooks/use-course-version-summary";
import { useApplyStageVersion } from "../hooks/use-stage-mutations";
import type { OutdatedCourse } from "../model/schemas";

export interface ApplyDialogProps {
  stageId: string;
  stageName: string;
  /** The course being moved onto the stage's latest published version; null keeps the dialog closed. */
  course: OutdatedCourse | null;
  onOpenChange: (open: boolean) => void;
}

/**
 * One-step apply: the API clones the course's published version, swaps in the new stage version
 * and publishes the clone, all in one transaction. A per-course error comes back with HTTP 200.
 */
export function ApplyDialog({ stageId, stageName, course, onOpenChange }: ApplyDialogProps) {
  const [busy, setBusy] = useState(false);
  return (
    <Dialog
      open={course !== null}
      onOpenChange={(next) => {
        if (!busy) onOpenChange(next);
      }}
    >
      <DialogContent aria-describedby={undefined}>
        {course ? (
          <ApplyBody
            stageId={stageId}
            stageName={stageName}
            course={course}
            onBusyChange={setBusy}
            onClose={() => {
              onOpenChange(false);
            }}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

interface ApplyBodyProps {
  stageId: string;
  stageName: string;
  course: OutdatedCourse;
  onBusyChange: (busy: boolean) => void;
  onClose: () => void;
}

function ApplyBody({ stageId, stageName, course, onBusyChange, onClose }: ApplyBodyProps) {
  const summary = useCourseVersionSummary(course.courseVersionId);
  const apply = useApplyStageVersion(stageId);
  const [errors, setErrors] = useState<string[]>([]);
  const n = course.latestVersionNo;
  const latest = course.courseVersionNo;
  const next = latest + 1;
  const pending = apply.isPending;

  async function confirm() {
    setErrors([]);
    onBusyChange(true);
    try {
      const { results } = await apply.mutateAsync({ vid: course.latestVersionId, courseId: course.courseId });
      const failed = results.filter((row) => row.error);
      if (failed.length > 0) {
        setErrors(failed.map((row) => `${row.courseCode}: ${row.error?.message ?? ""}`));
        return;
      }
      const applied = results.find((row) => row.courseId === course.courseId);
      toast.success(
        `Đã phát hành ${course.courseName} v${String(applied?.newVersionNo ?? next)} dùng v${String(n)}.`,
      );
      onClose();
    } catch (error) {
      setErrors([isApiError(error) ? error.message : NETWORK_ERROR_MESSAGE]);
    } finally {
      onBusyChange(false);
    }
  }

  let preview;
  if (summary.isPending) {
    preview = (
      <div className="grid gap-2" aria-busy="true">
        <Skeleton className="h-4 w-3/4" />
        <Skeleton className="h-4 w-2/3" />
        <Skeleton className="h-4 w-1/2" />
      </div>
    );
  } else if (summary.isError) {
    preview = (
      <Alert variant="danger" role="alert">
        Không tải được thông tin khóa học.
        <AlertActions>
          <Button variant="outline" size="sm" onClick={() => void summary.refetch()}>
            Thử lại
          </Button>
        </AlertActions>
      </Alert>
    );
  } else {
    const others = Math.max(summary.data.stages.length - 1, 0);
    const codes = summary.data.classes.map((c) => c.code);
    preview = (
      <>
        <p className="m-0">Trong một giao dịch, hệ thống sẽ:</p>
        <ol className="m-0 grid gap-1 pl-5">
          <li>
            Nhân bản{" "}
            <strong>
              {course.courseName} v{latest}
            </strong>{" "}
            thành <strong>v{next}</strong>.
          </li>
          <li>
            Thay{" "}
            <strong>
              {stageName} v{course.usingVersionNo}
            </strong>{" "}
            bằng <strong>v{n}</strong>, giữ nguyên thứ tự và {others} chặng còn lại.
          </li>
          <li>Phát hành v{next}.</li>
        </ol>
        <Alert variant="info">
          Các lớp đang chạy trên v{latest} ({codes.length > 0 ? codes.join(", ") : "chưa có"}) không bị ảnh hưởng. Lớp
          mới hoặc lớp nháp mới gắn được v{next}.
        </Alert>
      </>
    );
  }

  return (
    <>
      <DialogHeader showClose={!pending}>
        <DialogTitle>
          Áp dụng {stageName} v{n} cho {course.courseName}
        </DialogTitle>
      </DialogHeader>
      <DialogBody className="text-sm text-ink-2">
        {preview}
        {errors.length > 0 ? (
          <Alert variant="danger" role="alert">
            {errors.map((line) => (
              <div key={line}>{line}</div>
            ))}
          </Alert>
        ) : null}
      </DialogBody>
      <DialogFooter>
        <Button variant="outline" disabled={pending} onClick={onClose}>
          Hủy
        </Button>
        <Button disabled={pending || !summary.isSuccess} aria-busy={pending} onClick={() => void confirm()}>
          <PendingLabel
            pending={pending}
            idle={`Tạo và phát hành ${course.courseName} v${String(next)}`}
            busy="Đang áp dụng…"
          />
        </Button>
      </DialogFooter>
    </>
  );
}
