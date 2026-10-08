import { Card } from "@/shared/ui/card";
import { ProgressBar } from "@/shared/ui/progress-bar";
import { stageProgress } from "../model/progress";
import type { ReadOnlyReason, RoadmapStage } from "../model/schemas";
import { LessonRow } from "./lesson-row";

export interface StageSectionProps {
  classId: string;
  stage: RoadmapStage;
  /** 1-based position shown in the square. */
  ord: number;
  readOnlyReason: ReadOnlyReason | undefined;
  onToggle: (lessonId: string, completed: boolean) => void;
}

export function StageSection({ classId, stage, ord, readOnlyReason, onToggle }: StageSectionProps) {
  const progress = stageProgress(stage);
  return (
    // id: target of the lesson page's stage crumb (`#stage-{id}`).
    <Card id={`stage-${stage.id}`} className="scroll-mt-4">
      <div className="flex flex-wrap items-center gap-3 border-b border-line px-6 py-4 max-[720px]:px-4">
        <span className="inline-grid size-7 flex-none place-items-center bg-surface-blue-50 text-xs font-semibold text-navy-700 tabular-nums">
          {ord}
        </span>
        <div className="min-w-0 flex-1">
          <h2 className="text-md">{stage.name}</h2>
          <div className="text-sm text-ink-3">
            {stage.lessons.length} học liệu · {progress.done}/{progress.total} bắt buộc đã xong
          </div>
        </div>
        <ProgressBar value={progress.percent} className="w-[180px] max-[720px]:w-full" />
      </div>
      {stage.lessons.map((lesson) => (
        <LessonRow key={lesson.id} classId={classId} lesson={lesson} readOnlyReason={readOnlyReason} onToggle={onToggle} />
      ))}
    </Card>
  );
}
