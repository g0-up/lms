import { CircleCheck } from "lucide-react";
import { useEffect, useRef } from "react";
import { toast } from "sonner";
import { isApiError } from "@/shared/api/errors";
import { fmtDateTime, rel, versionLabel } from "@/shared/lib/format";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { ProgressBar } from "@/shared/ui/progress-bar";
import { Sheet, SheetBody, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/shared/ui/sheet";
import { Skeleton } from "@/shared/ui/skeleton";
import { StatusDot } from "@/shared/ui/status-dot";
import { TypeIcon } from "@/shared/ui/type-icon";
import { useMemberReport } from "../hooks/use-member-report";
import { MEMBER_NOT_FOUND } from "../model/messages";
import type { MemberReport } from "../model/schemas";

type DrawerStage = MemberReport["stages"][number];

function StageSection({ stage, index }: { stage: DrawerStage; index: number }) {
  return (
    <section>
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line bg-surface-50 px-6 py-4 max-[720px]:px-4">
        <h2 className="text-md">
          {index + 1}. {stage.name}{" "}
          <span className="text-sm font-normal text-ink-3">{versionLabel({ no: stage.versionNo })}</span>
        </h2>
        <ProgressBar value={stage.percent} className="w-[140px]" />
      </div>
      <ul>
        {stage.lessons.map((lesson) => (
          <li key={lesson.id} className="flex items-center gap-3 border-b border-line px-5 py-3">
            <TypeIcon type={lesson.type} />
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-2 font-medium text-ink">
                {lesson.title}
                {lesson.required ? null : <Badge variant="subtle">Không bắt buộc</Badge>}
              </div>
              <div className="mt-0.5 flex flex-wrap gap-x-3 text-xs text-ink-3">
                <span>{lesson.firstOpenedAt ? `Mở lần đầu ${fmtDateTime(lesson.firstOpenedAt)}` : "Chưa mở"}</span>
                {lesson.completedAt ? (
                  <StatusDot variant="ok" label={`Tích ${fmtDateTime(lesson.completedAt)}`} />
                ) : null}
              </div>
            </div>
            {lesson.completedAt ? <CircleCheck aria-hidden="true" className="size-5 shrink-0 text-ok" /> : null}
          </li>
        ))}
      </ul>
    </section>
  );
}

function DrawerSkeleton() {
  return (
    <div className="grid gap-4 p-6" aria-busy="true">
      <span className="sr-only">Đang tải chi tiết học viên</span>
      <Skeleton className="h-6 w-full" />
      <Skeleton className="h-24 w-full" />
      <Skeleton className="h-24 w-full" />
    </div>
  );
}

export interface StudentDetailDrawerProps {
  classId: string;
  /** `class_members.id` of the open row; null keeps the drawer closed. */
  memberId: string | null;
  /** Must be stable (e.g. from `useCallback`): it also runs when the member is not in the class. */
  onClose: () => void;
}

/** Right drawer with one member's progress, stage by stage and lesson by lesson. */
export function StudentDetailDrawer({ classId, memberId, onClose }: StudentDetailDrawerProps) {
  const { data, error, isPending, isFetching, refetch } = useMemberReport(classId, memberId);
  const open = memberId !== null;
  // Opened without a Radix trigger: remember the row that had focus to give it back on close.
  const opener = useRef<HTMLElement | null>(null);
  // A cached 404 refetches when the same row is opened again; wait for that answer.
  const notFound = open && !isFetching && isApiError(error) && error.status === 404;

  useEffect(() => {
    if (!notFound) return;
    toast.error(MEMBER_NOT_FOUND, { id: MEMBER_NOT_FOUND });
    onClose();
  }, [notFound, onClose]);

  const member = data?.member;
  let body;
  if (isPending || notFound) {
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
    body = (
      <>
        <div className="border-b border-line p-6 max-[720px]:px-4">
          <ProgressBar size="lg" value={data.member.percent} />
          <div className="mt-2 text-sm text-ink-3">
            {data.member.requiredDone}/{data.member.requiredTotal} học liệu bắt buộc · tiến độ do học viên tự xác
            nhận
          </div>
        </div>
        {data.stages.map((stage, i) => (
          <StageSection key={stage.stageId} stage={stage} index={i} />
        ))}
      </>
    );
  }

  return (
    <Sheet open={open} onOpenChange={(next) => { if (!next) onClose(); }}>
      <SheetContent
        aria-label="Chi tiết học viên"
        aria-labelledby={undefined}
        onOpenAutoFocus={() => {
          opener.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
        }}
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          opener.current?.focus();
        }}
        {...(member ? {} : { "aria-describedby": undefined })}
      >
        <SheetHeader>
          <div className="min-w-0">
            <SheetTitle>{member?.name ?? "Chi tiết học viên"}</SheetTitle>
            {member ? (
              <SheetDescription className="text-sm">
                {member.email} · {data.class.code} · hoạt động cuối {rel(member.lastActivityAt)}
              </SheetDescription>
            ) : null}
          </div>
        </SheetHeader>
        <SheetBody role="region" aria-label="Tiến độ theo chặng">
          {body}
        </SheetBody>
      </SheetContent>
    </Sheet>
  );
}
