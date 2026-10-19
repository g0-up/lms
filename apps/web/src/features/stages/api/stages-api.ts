import { http } from "@/shared/api/http";
import type { CriterionBody } from "../model/criterion-form";
import type { LessonBody, LessonPatch } from "../model/lesson-form";
import type { QuestionBody } from "../model/question-form";
import type { UnitLessons } from "../model/order";
import {
  applyResultsSchema,
  courseVersionSummarySchema,
  deleteStageVersionSchema,
  lessonSchema,
  markdownPreviewSchema,
  stageDetailSchema,
  stageListSchema,
  stageVersionSchema,
  unitSchema,
  type ApplyResults,
  type CourseVersionSummary,
  type Lesson,
  type MarkdownPreview,
  type StageDetail,
  type StageList,
  type StageVersion,
  type Unit,
} from "../model/schemas";

const enc = encodeURIComponent;
const versionPath = (vid: string) => `/stage-versions/${enc(vid)}`;
const lessonPath = (vid: string, lessonId: string) => `${versionPath(vid)}/lessons/${enc(lessonId)}`;

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
    http(lessonPath(vid, lessonId), { method: "PATCH", body, schema: lessonSchema }),
  removeLesson: (vid: string, lessonId: string): Promise<undefined> =>
    http(lessonPath(vid, lessonId), { method: "DELETE" }),
  /** The full layout: every unit of the version in its current order, each with every one of its lessons. */
  reorderLessons: (vid: string, units: UnitLessons[]): Promise<StageVersion> =>
    http(`${versionPath(vid)}/lessons/order`, { method: "PUT", body: { units }, schema: stageVersionSchema }),
  /** Question and criterion changes answer with the whole lesson. */
  addQuestion: (vid: string, lessonId: string, body: QuestionBody): Promise<Lesson> =>
    http(`${lessonPath(vid, lessonId)}/questions`, { method: "POST", body, schema: lessonSchema }),
  /** A full replace: options left out are deleted, options sent with their id keep it. */
  updateQuestion: (vid: string, lessonId: string, questionId: string, body: QuestionBody): Promise<Lesson> =>
    http(`${lessonPath(vid, lessonId)}/questions/${enc(questionId)}`, { method: "PATCH", body, schema: lessonSchema }),
  removeQuestion: (vid: string, lessonId: string, questionId: string): Promise<undefined> =>
    http(`${lessonPath(vid, lessonId)}/questions/${enc(questionId)}`, { method: "DELETE" }),
  /** Every question id of the quiz, in the new order. */
  reorderQuestions: (vid: string, lessonId: string, questionIds: string[]): Promise<Lesson> =>
    http(`${lessonPath(vid, lessonId)}/questions/order`, {
      method: "PUT",
      body: { questionIds },
      schema: lessonSchema,
    }),
  addCriterion: (vid: string, lessonId: string, body: CriterionBody): Promise<Lesson> =>
    http(`${lessonPath(vid, lessonId)}/criteria`, { method: "POST", body, schema: lessonSchema }),
  updateCriterion: (vid: string, lessonId: string, criterionId: string, body: CriterionBody): Promise<Lesson> =>
    http(`${lessonPath(vid, lessonId)}/criteria/${enc(criterionId)}`, { method: "PATCH", body, schema: lessonSchema }),
  removeCriterion: (vid: string, lessonId: string, criterionId: string): Promise<undefined> =>
    http(`${lessonPath(vid, lessonId)}/criteria/${enc(criterionId)}`, { method: "DELETE" }),
  /** Every criterion id of the homework, in the new order. */
  reorderCriteria: (vid: string, lessonId: string, criterionIds: string[]): Promise<Lesson> =>
    http(`${lessonPath(vid, lessonId)}/criteria/order`, {
      method: "PUT",
      body: { criterionIds },
      schema: lessonSchema,
    }),
  createUnit: (vid: string, title: string): Promise<Unit> =>
    http(`${versionPath(vid)}/units`, { method: "POST", body: { title }, schema: unitSchema }),
  renameUnit: (vid: string, unitId: string, title: string): Promise<Unit> =>
    http(`${versionPath(vid)}/units/${enc(unitId)}`, { method: "PATCH", body: { title }, schema: unitSchema }),
  /** Deletes the unit together with its lessons. */
  removeUnit: (vid: string, unitId: string): Promise<undefined> =>
    http(`${versionPath(vid)}/units/${enc(unitId)}`, { method: "DELETE" }),
  /** Every unit id of the version, in the new order. */
  reorderUnits: (vid: string, unitIds: string[]): Promise<StageVersion> =>
    http(`${versionPath(vid)}/units/order`, { method: "PUT", body: { unitIds }, schema: stageVersionSchema }),
  /** Clones, swaps and publishes each course in one transaction per course. */
  apply: (vid: string, courseIds: string[]): Promise<ApplyResults> =>
    http(`${versionPath(vid)}/apply`, { method: "POST", body: { courseIds }, schema: applyResultsSchema }),
  /** Server-rendered HTML of a markdown source; 422 when it holds an image that was not uploaded. */
  previewMarkdown: (markdownSource: string, signal?: AbortSignal): Promise<MarkdownPreview> =>
    http("/stages/markdown-preview", {
      method: "POST",
      body: { markdownSource },
      schema: markdownPreviewSchema,
      signal,
    }),
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
