import { Link } from "react-router";
import { cn } from "@/shared/lib/cn";
import { TypeIcon } from "@/shared/ui/type-icon";
import { lessonSub } from "../model/progress";
import type { ReadOnlyReason, RoadmapLesson } from "../model/schemas";
import { tickBlocker } from "../model/tick-rule";
import { TickCheckbox } from "./tick-checkbox";

export interface LessonRowProps {
  classId: string;
  lesson: RoadmapLesson;
  readOnlyReason: ReadOnlyReason | undefined;
  onToggle: (lessonId: string, completed: boolean) => void;
}

export function LessonRow({ classId, lesson, readOnlyReason, onToggle }: LessonRowProps) {
  const done = Boolean(lesson.completedAt);
  const body = (
    <>
      <TypeIcon type={lesson.type} />
      <span className="min-w-0">
        <span className={cn("font-medium group-hover:underline", done ? "text-ink-2" : "text-ink")}>{lesson.title}</span>
        <br />
        <span className="text-xs text-ink-3">{lessonSub(lesson)}</span>
      </span>
    </>
  );
  return (
    <div className="flex min-h-14 items-center gap-3 border-b border-line px-6 py-2 last:border-b-0 hover:bg-surface-50 max-[720px]:px-4">
      {/* A draft class cannot open lessons yet (the API answers 409), so its rows are not links. */}
      {readOnlyReason === "draft" ? (
        <div className="flex min-h-11 min-w-0 flex-1 items-center gap-3">{body}</div>
      ) : (
        <Link
          to={`/learn/classes/${classId}/lessons/${lesson.id}`}
          className="group flex min-h-11 min-w-0 flex-1 items-center gap-3 text-ink no-underline"
        >
          {body}
        </Link>
      )}
      <TickCheckbox
        lessonTitle={lesson.title}
        checked={done}
        blocker={tickBlocker({ readOnlyReason, opened: Boolean(lesson.firstOpenedAt) })}
        onCheckedChange={(completed) => {
          onToggle(lesson.id, completed);
        }}
      />
    </div>
  );
}
