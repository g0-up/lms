import { percentOf, type Percent } from "@/shared/domain";
import type { RoadmapLesson, RoadmapStage, RoadmapUnit } from "./schemas";
import { tickable } from "./tick-rule";

export interface StageProgress {
  done: number;
  total: number;
  percent: Percent;
}

/** Required lessons completed in one stage; optional lessons never count. */
export function stageProgress(stage: Pick<RoadmapStage, "lessons">): StageProgress {
  const required = stage.lessons.filter((l) => l.required);
  const done = required.filter((l) => l.completedAt).length;
  return { done, total: required.length, percent: percentOf(done, required.length) };
}

export interface UnitProgress {
  done: number;
  total: number;
  /** No required lesson: the unit never counts as complete. */
  optional: boolean;
  complete: boolean;
}

/**
 * Required lessons completed in one bài học, from its lessons rather than the API's counts so an
 * optimistic tick shows at once. Same rule as the API: complete once every required lesson is done.
 */
export function unitProgress(unit: Pick<RoadmapUnit, "lessons">): UnitProgress {
  const { done, total } = stageProgress(unit);
  return { done, total, optional: total === 0, complete: total > 0 && done === total };
}

/** Seconds as mm:ss (1104 → "18:24"); minutes keep counting past an hour. */
export function formatDuration(seconds: number): string {
  const whole = Math.max(0, Math.round(seconds));
  return `${String(Math.floor(whole / 60)).padStart(2, "0")}:${String(whole % 60).padStart(2, "0")}`;
}

const TYPE_SUB = { markdown: "Bài đọc", quiz: "Quiz", homework: "Bài tập" } as const;

/**
 * Second line of a roadmap row: "Video · 18:24", "Bài đọc", "Quiz" or "Bài tập", then the optional
 * marker and, for lessons the student ticks, the opened marker (a graded lesson completes by its result).
 */
export function lessonSub(
  lesson: Pick<RoadmapLesson, "type" | "required" | "durationSeconds" | "firstOpenedAt" | "completedAt">,
): string {
  const parts: string[] = [];
  if (lesson.type === "video") {
    parts.push(lesson.durationSeconds === undefined ? "Video" : `Video · ${formatDuration(lesson.durationSeconds)}`);
  } else {
    parts.push(TYPE_SUB[lesson.type]);
  }
  if (!lesson.required) parts.push("không bắt buộc");
  if (tickable(lesson.type) && lesson.firstOpenedAt && !lesson.completedAt) parts.push("đã mở");
  return parts.join(" · ");
}
