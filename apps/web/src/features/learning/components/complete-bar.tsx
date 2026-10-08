import { Check } from "lucide-react";
import { toast } from "sonner";
import { fmtDateTime } from "@/shared/lib/format";
import { Button } from "@/shared/ui/button";
import { StatusDot } from "@/shared/ui/status-dot";
import type { LessonLink } from "../model/schemas";
import { LessonNav } from "./lesson-nav";

export interface CompleteBarProps {
  classId: string;
  prev?: LessonLink;
  next?: LessonLink;
  completedAt: string | undefined;
  /** Why the student cannot tick or untick (see `tickBlocker`), or null. */
  blocker: string | null;
  onToggle: (completed: boolean) => void;
}

/** Bottom bar of the lesson card: navigation on the left, self-reported completion on the right. */
export function CompleteBar({ classId, prev, next, completedAt, blocker, onToggle }: CompleteBarProps) {
  const done = Boolean(completedAt);
  // aria-disabled instead of disabled: the button stays focusable and a click explains why.
  const guard = {
    "aria-disabled": blocker ? true : undefined,
    title: blocker ?? undefined,
    onClick: () => {
      if (blocker) toast.error(blocker);
      else onToggle(!done);
    },
  };
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line px-6 py-4 max-[720px]:px-4">
      <LessonNav classId={classId} prev={prev} next={next} />
      <div className="flex flex-wrap items-center gap-3">
        {done ? (
          <>
            <StatusDot variant="ok" label={`Đã tích ${fmtDateTime(completedAt)}`} className="text-sm" />
            <Button variant="outline" {...guard}>
              Bỏ tích
            </Button>
          </>
        ) : (
          <Button variant="gradient" {...guard}>
            <Check aria-hidden="true" /> Đã học xong
          </Button>
        )}
      </div>
    </div>
  );
}
