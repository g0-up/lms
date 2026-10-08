import type { RoadmapLesson, RoadmapStage } from "./schemas";

export interface FlatLesson {
  lesson: RoadmapLesson;
  stage: Pick<RoadmapStage, "id" | "name">;
}

/** Every lesson of the course version: stage order first, then lesson order within the stage. */
export function flattenLessons(stages: readonly RoadmapStage[]): FlatLesson[] {
  return stages.flatMap((stage) => stage.lessons.map((lesson) => ({ lesson, stage: { id: stage.id, name: stage.name } })));
}

/** Lessons right before and after `lessonId`; both undefined when it is not in the list. */
export function neighbors(
  flat: readonly FlatLesson[],
  lessonId: string,
): { prev: FlatLesson | undefined; next: FlatLesson | undefined } {
  const i = flat.findIndex((f) => f.lesson.id === lessonId);
  if (i < 0) return { prev: undefined, next: undefined };
  return { prev: flat[i - 1], next: flat[i + 1] };
}

/** First lesson not completed yet, required or not: where "Học tiếp" leads. */
export function firstIncomplete(flat: readonly FlatLesson[]): FlatLesson | undefined {
  return flat.find((f) => !f.lesson.completedAt);
}
