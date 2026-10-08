import { http } from "@/shared/api/http";
import type { LessonBody, LessonPatch } from "../model/lesson-form";
import {
  applyResultsSchema,
  courseVersionSummarySchema,
  deleteStageVersionSchema,
  lessonSchema,
  markdownPreviewSchema,
  stageDetailSchema,
  stageListSchema,
  stageVersionSchema,
  type ApplyResults,
  type CourseVersionSummary,
  type Lesson,
  type MarkdownPreview,
  type StageDetail,
  type StageList,
  type StageVersion,
} from "../model/schemas";

const enc = encodeURIComponent;
const versionPath = (vid: string) => `/stage-versions/${enc(vid)}`;

export interface NewStage {
  code: string;
  name: string;
}

export const stagesApi = {
  list: (signal?: AbortSignal): Promise<StageList> => http("/stages", { schema: stageListSchema, signal }),
  /** Creates the stage with an empty draft v1. */
  create: (body: NewStage): Promise<StageDetail> =>
    http("/stages", { method: "POST", body, schema: stageDetailSchema }),
  detail: (stageId: string, signal?: AbortSignal): Promise<StageDetail> =>
    http(`/stages/${enc(stageId)}`, { schema: stageDetailSchema, signal }),
  version: (vid: string, signal?: AbortSignal): Promise<StageVersion> =>
    http(versionPath(vid), { schema: stageVersionSchema, signal }),
  clone: (vid: string): Promise<StageVersion> =>
    http(`${versionPath(vid)}/clone`, { method: "POST", schema: stageVersionSchema }),
  publish: (vid: string): Promise<StageVersion> =>
    http(`${versionPath(vid)}/publish`, { method: "POST", schema: stageVersionSchema }),
  archive: (vid: string): Promise<StageVersion> =>
    http(`${versionPath(vid)}/archive`, { method: "POST", schema: stageVersionSchema }),
  remove: (vid: string): Promise<{ stageDeleted: boolean }> =>
    http(versionPath(vid), { method: "DELETE", schema: deleteStageVersionSchema }),
  addLesson: (vid: string, body: LessonBody): Promise<Lesson> =>
    http(`${versionPath(vid)}/lessons`, { method: "POST", body, schema: lessonSchema }),
  updateLesson: (vid: string, lessonId: string, body: LessonPatch): Promise<Lesson> =>
    http(`${versionPath(vid)}/lessons/${enc(lessonId)}`, { method: "PATCH", body, schema: lessonSchema }),
  removeLesson: (vid: string, lessonId: string): Promise<undefined> =>
    http(`${versionPath(vid)}/lessons/${enc(lessonId)}`, { method: "DELETE" }),
  reorderLessons: (vid: string, lessonIds: string[]): Promise<StageVersion> =>
    http(`${versionPath(vid)}/lessons/order`, { method: "PUT", body: { lessonIds }, schema: stageVersionSchema }),
  /** Clones, swaps and publishes each course in one transaction per course. */
  apply: (vid: string, courseIds: string[]): Promise<ApplyResults> =>
    http(`${versionPath(vid)}/apply`, { method: "POST", body: { courseIds }, schema: applyResultsSchema }),
  /** Server-rendered HTML of a markdown source; 422 when it holds an image that was not uploaded. */
  previewMarkdown: (markdownSource: string, signal?: AbortSignal): Promise<MarkdownPreview> =>
    http("/stages/markdown-preview", { method: "POST", body: { markdownSource }, schema: markdownPreviewSchema, signal }),
  /** Course version read for the apply preview (owned by the courses feature; only the needed fields are kept). */
  courseVersionSummary: (courseVersionId: string, signal?: AbortSignal): Promise<CourseVersionSummary> =>
    http(`/course-versions/${enc(courseVersionId)}`, { schema: courseVersionSummarySchema, signal }),
};

export const stageKeys = {
  list: ["stages"] as const,
  detail: (stageId: string) => ["stage", stageId] as const,
  version: (vid: string) => ["stage-version", vid] as const,
};

/** Keys owned by the courses and dashboard features that stage actions make stale. */
export const relatedKeys = {
  dashboard: ["dashboard"] as const,
  courses: ["courses"] as const,
  course: (courseId: string) => ["course", courseId] as const,
  courseVersionSummary: (courseVersionId: string) => ["course-version", courseVersionId, "summary"] as const,
};
