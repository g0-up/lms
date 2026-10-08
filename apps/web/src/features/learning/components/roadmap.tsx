import type { Roadmap as RoadmapData } from "../model/schemas";
import { StageSection } from "./stage-section";

export interface RoadmapProps {
  roadmap: RoadmapData;
  onToggle: (lessonId: string, completed: boolean) => void;
}

/** Stages of the class's course version in order, each with its lessons. */
export function Roadmap({ roadmap, onToggle }: RoadmapProps) {
  return (
    <div className="grid gap-6">
      {roadmap.stages.map((stage, i) => (
        <StageSection
          key={stage.id}
          classId={roadmap.class.id}
          stage={stage}
          ord={i + 1}
          readOnlyReason={roadmap.readOnlyReason}
          onToggle={onToggle}
        />
      ))}
    </div>
  );
}
