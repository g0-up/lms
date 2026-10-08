import { percentOf, type Percent } from "@/shared/domain";
import type { RoadmapLesson, RoadmapStage } from "./schemas";

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

/** Seconds as mm:ss (1104 → "18:24"); minutes keep counting past an hour. */
export function formatDuration(seconds: number): string {
  const whole = Math.max(0, Math.round(seconds));
  return `${String(Math.floor(whole / 60)).padStart(2, "0")}:${String(whole % 60).padStart(2, "0")}`;
}

/** Second line of a roadmap row: "Video · 18:24" or "Bài đọc", then optional and opened markers. */
export function lessonSub(
  lesson: Pick<RoadmapLesson, "type" | "required" | "durationSeconds" | "firstOpenedAt" | "completedAt">,
): string {
  const parts: string[] = [];
  if (lesson.type === "video") {
    parts.push(lesson.durationSeconds === undefined ? "Video" : `Video · ${formatDuration(lesson.durationSeconds)}`);
  } else {
    parts.push("Bài đọc");
  }
  if (!lesson.required) parts.push("không bắt buộc");
  if (lesson.firstOpenedAt && !lesson.completedAt) parts.push("đã mở");
  return parts.join(" · ");
}
