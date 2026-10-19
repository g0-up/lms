import { useEffect, useRef } from "react";
import { toast } from "sonner";
import { isApiError } from "@/shared/api/errors";
import { versionLabel } from "@/shared/lib/format";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { ProgressBar } from "@/shared/ui/progress-bar";
import { Sheet, SheetBody, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/shared/ui/sheet";
import { Skeleton } from "@/shared/ui/skeleton";
import type { StageTarget } from "../api/summary-api";
import { useMemberStage } from "../hooks/use-member-stage";
import { STAGE_NOT_FOUND } from "../model/messages";
import type { MemberStage } from "../model/schemas";
import { StageItems } from "./stage-items";
import { StageReviewForm } from "./stage-review-form";

function DrawerSkeleton() {
  return (
    <div className="grid gap-4 p-6" aria-busy="true">
      <span className="sr-only">Đang tải tổng kết chặng</span>
      <Skeleton className="h-6 w-full" />
      <Skeleton className="h-24 w-full" />
      <Skeleton className="h-24 w-full" />
    </div>
  );
}

function stageState(summary: MemberStage["summary"]): string {
  return summary.complete ? "Đã hoàn thành chặng" : `Còn ${String(summary.missing.length)} học liệu bắt buộc`;
}

export interface StageSummaryDrawerProps {
  classId: string;
  /** The open matrix cell; null keeps the drawer closed. */
  target: StageTarget | null;
  /** Must be stable (e.g. from `useCallback`): it also runs when the member or stage is not part of the class. */
  onClose: () => void;
}

/** Right drawer with one member's stage, lesson by lesson, and the teacher's review of it. */
export function StageSummaryDrawer({ classId, target, onClose }: StageSummaryDrawerProps) {
  const { data, error, isPending, isFetching, refetch } = useMemberStage(classId, target);
  const open = target !== null;
  // Opened without a Radix trigger: remember the cell that had focus to give it back on close.
  const opener = useRef<HTMLElement | null>(null);
  // A cached 404 refetches when the same cell is opened again; wait for that answer.
  const notFound = open && !isFetching && isApiError(error) && error.status === 404;

  useEffect(() => {
    if (!notFound) return;
    toast.error(STAGE_NOT_FOUND, { id: STAGE_NOT_FOUND });
    onClose();
  }, [notFound, onClose]);

  let body;
  if (isPending || notFound || !target) {
    body = <DrawerSkeleton />;
  } else if (error) {
    body = (
      <div className="p-6">
        <Alert variant="danger" role="alert">
          {error.message}
          <AlertActions>
            <Button variant="outline" size="sm" onClick={() => void refetch()}>
              Thử lại
            </Button>
          </AlertActions>
        </Alert>
      </div>
    );
  } else {
    const { summary } = data;
    body = (
      <>
        <div className="border-b border-line p-6 max-[720px]:px-4">
          <ProgressBar size="lg" value={summary.percent} />
          <div className="mt-2 text-sm text-ink-3">
            {summary.done}/{summary.required} học liệu bắt buộc · video và bài đọc do học viên tự xác nhận; quiz và bài
            tập tính theo kết quả
          </div>
        </div>
        <StageItems units={summary.units} />
        <StageReviewForm
          key={`${target.memberId}/${target.stageVersionId}`}
          classId={classId}
          target={target}
          review={summary.review}
        />
      </>
    );
  }

  return (
    <Sheet open={open} onOpenChange={(next) => { if (!next) onClose(); }}>
      <SheetContent
        aria-label="Tổng kết chặng"
        aria-labelledby={undefined}
        onOpenAutoFocus={() => {
          opener.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
        }}
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          opener.current?.focus();
        }}
        {...(data ? {} : { "aria-describedby": undefined })}
      >
        <SheetHeader>
          <div className="min-w-0">
            <SheetTitle>{data ? `${data.member.name} · ${data.summary.name}` : "Tổng kết chặng"}</SheetTitle>
            {data ? (
              <SheetDescription className="text-sm">
                {data.summary.code} · {versionLabel({ no: data.summary.versionNo })} · {stageState(data.summary)}
              </SheetDescription>
            ) : null}
          </div>
        </SheetHeader>
        <SheetBody role="region" aria-label="Học liệu và nhận xét của chặng">
          {body}
        </SheetBody>
      </SheetContent>
    </Sheet>
  );
}
