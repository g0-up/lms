import { StatusDot } from "@/shared/ui/status-dot";
import { unitProgress } from "../model/progress";
import type { ReadOnlyReason, RoadmapUnit } from "../model/schemas";
import { LessonRow } from "./lesson-row";

export interface UnitGroupProps {
  classId: string;
  unit: RoadmapUnit;
  /** "{stage}.{unit}", 1-based: the "Bài 1.2" of the heading. */
  ord: string;
  readOnlyReason: ReadOnlyReason | undefined;
  onToggle: (lessonId: string, completed: boolean) => void;
}

/** One bài học of a roadmap stage: its heading, required progress, then its lessons. */
export function UnitGroup({ classId, unit, ord, readOnlyReason, onToggle }: UnitGroupProps) {
  const progress = unitProgress(unit);
  const headingId = `unit-${unit.id}-title`;
  return (
    // id: target of the lesson page's bài học crumb (`#unit-{id}`).
    <section id={`unit-${unit.id}`} aria-labelledby={headingId} className="scroll-mt-4 border-b border-line last:border-b-0">
      <div className="flex flex-wrap items-center gap-3 border-b border-line bg-surface-50 px-6 py-3 max-[720px]:px-4">
        <h3 id={headingId} className="min-w-0 flex-1 text-md font-semibold text-navy-900">
          Bài {ord} · {unit.title}
        </h3>
        <span className="text-sm text-ink-3">
          {progress.optional ? "Không bắt buộc" : `${String(progress.done)}/${String(progress.total)} bắt buộc`}
        </span>
        {progress.complete ? <StatusDot variant="ok" label="Hoàn thành" className="text-sm" /> : null}
      </div>
      {unit.lessons.map((lesson) => (
        <LessonRow key={lesson.id} classId={classId} lesson={lesson} readOnlyReason={readOnlyReason} onToggle={onToggle} />
      ))}
    </section>
  );
}
