import { CircleCheck, Layers } from "lucide-react";
import { Link } from "react-router";
import { Button } from "@/shared/ui/button";
import { Card, CardHeader } from "@/shared/ui/card";
import type { RoadmapStage } from "../model/schemas";

export interface LessonSideListProps {
  classId: string;
  stage: RoadmapStage;
  currentLessonId: string;
}

/** Lessons of the current stage, with their completion, next to the viewer. */
export function LessonSideList({ classId, stage, currentLessonId }: LessonSideListProps) {
  return (
    <aside aria-label={`Học liệu chặng ${stage.name}`}>
      <Card>
        <CardHeader>
          <h2>{stage.name}</h2>
        </CardHeader>
        <ul>
          {stage.lessons.map((lesson) => (
            <li key={lesson.id}>
              <Link
                to={`/learn/classes/${classId}/lessons/${lesson.id}`}
                aria-current={lesson.id === currentLessonId ? "page" : undefined}
                className="flex min-h-11 items-center gap-2 border-b border-line px-4 py-2.5 text-sm text-ink-2 no-underline hover:bg-surface-50 aria-[current=page]:bg-surface-blue-50 aria-[current=page]:font-semibold aria-[current=page]:text-navy-900"
              >
                {lesson.completedAt ? (
                  <CircleCheck role="img" aria-label="Đã học xong" className="size-4 flex-none text-ok" />
                ) : (
                  <span aria-hidden="true" className="size-4 flex-none rounded-full border-[1.5px] border-gray-300" />
                )}
                <span>{lesson.title}</span>
              </Link>
            </li>
          ))}
        </ul>
        <div className="p-6 max-[720px]:px-4">
          <Button asChild variant="ghost" size="sm">
            <Link to={`/learn/classes/${classId}`}>
              <Layers aria-hidden="true" /> Toàn bộ lộ trình
            </Link>
          </Button>
        </div>
      </Card>
    </aside>
  );
}
