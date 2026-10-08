import { queryOptions, useQuery } from "@tanstack/react-query";
import { learningApi, learningKeys } from "../api/learning-api";
import type { Roadmap, RoadmapLesson } from "../model/schemas";

export function myClassQuery(classId: string) {
  return queryOptions({
    queryKey: learningKeys.myClass(classId),
    queryFn: ({ signal }) => learningApi.myClass(classId, signal),
  });
}

/** Roadmap of one class with the student's progress; 404 means not (or no longer) a member. */
export function useMyClass(classId: string) {
  return useQuery(myClassQuery(classId));
}

/** Roadmap with one lesson replaced; the same object when `update` returns the lesson unchanged. */
export function updateRoadmapLesson(
  roadmap: Roadmap,
  lessonId: string,
  update: (lesson: RoadmapLesson) => RoadmapLesson,
): Roadmap {
  for (const [si, stage] of roadmap.stages.entries()) {
    const li = stage.lessons.findIndex((l) => l.id === lessonId);
    if (li === -1) continue;
    const lesson = stage.lessons[li];
    const next = update(lesson);
    if (next === lesson) return roadmap;
    const stages = roadmap.stages.with(si, { ...stage, lessons: stage.lessons.with(li, next) });
    return { ...roadmap, stages };
  }
  return roadmap;
}
