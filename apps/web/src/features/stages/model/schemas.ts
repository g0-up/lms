import { z } from "zod";
import { isoDate } from "@/shared/api/schemas";

const id = z.string().min(1);
const versionNo = z.number().int().positive();
const count = z.number().int().nonnegative();

export const versionStatusSchema = z.enum(["draft", "published", "archived"]);
export const lessonTypeSchema = z.enum(["video", "markdown", "quiz", "homework"]);
export type LessonType = z.infer<typeof lessonTypeSchema>;
export const quizModeSchema = z.enum(["practice", "assessment"]);
export type QuizMode = z.infer<typeof quizModeSchema>;
export const questionTypeSchema = z.enum(["single", "multi"]);
export type QuestionType = z.infer<typeof questionTypeSchema>;

/** `VersionSummaryDTO`: one version on the stage list and detail (the API sorts newest first). */
export const stageVersionSummarySchema = z.object({
  id,
  versionNo,
  status: versionStatusSchema,
  lessonCount: count,
  publishedAt: isoDate.optional(),
  clonedFromVersionNo: versionNo.optional(),
});
export type StageVersionSummary = z.infer<typeof stageVersionSummarySchema>;

/** `GET /stages`. */
export const stageListSchema = z.object({
  items: z.array(
    z.object({
      id,
      code: z.string(),
      name: z.string(),
      description: z.string().optional(),
      latestVersionNo: versionNo,
      latestPublishedNo: versionNo.optional(),
      draftVersionId: id.optional(),
      versions: z.array(stageVersionSummarySchema),
      usedByCourseCount: count,
      outdatedCourseCount: count,
    }),
  ),
});
export type StageList = z.infer<typeof stageListSchema>;
export type StageListItem = StageList["items"][number];

/** A course version referencing a stage version. */
export const usedByRowSchema = z.object({
  courseId: id,
  courseCode: z.string(),
  courseName: z.string(),
  courseVersionId: id,
  versionNo,
  status: versionStatusSchema,
  classCodes: z.array(z.string()),
});
export type UsedByRow = z.infer<typeof usedByRowSchema>;

/** A published course still on an older version of the stage; `canApply` is false while it has a draft. */
export const outdatedCourseSchema = z.object({
  courseId: id,
  courseCode: z.string(),
  courseName: z.string(),
  courseVersionId: id,
  courseVersionNo: versionNo,
  usingVersionNo: versionNo,
  latestVersionId: id,
  latestVersionNo: versionNo,
  canApply: z.boolean(),
  blockedReason: z.string().optional(),
  draftVersionId: id.optional(),
  draftVersionNo: versionNo.optional(),
});
export type OutdatedCourse = z.infer<typeof outdatedCourseSchema>;

/** `GET /stages/{id}` and `POST /stages`. */
export const stageDetailSchema = z.object({
  id,
  code: z.string(),
  name: z.string(),
  description: z.string().optional(),
  versions: z.array(stageVersionSummarySchema),
  usedBy: z.array(usedByRowSchema.extend({ stageVersionNo: versionNo, outdated: z.boolean() })),
  outdatedCourses: z.array(outdatedCourseSchema),
});
export type StageDetail = z.infer<typeof stageDetailSchema>;

/** `UnitDTO`: a bài học groups a version's lessons; `position` starts at 1. */
export const unitSchema = z.object({
  id,
  unitKey: z.string(),
  title: z.string(),
  position: z.number().int(),
  lessonCount: count,
  requiredCount: count,
});
export type Unit = z.infer<typeof unitSchema>;

/** `OptionDTO`: one answer choice. */
export const optionSchema = z.object({ id, body: z.string(), isCorrect: z.boolean() });
export type QuestionOption = z.infer<typeof optionSchema>;

/** `QuestionDTO`: `questionKey` is fixed at creation; the HTML fields only once published. */
export const questionSchema = z.object({
  id,
  questionKey: z.string(),
  type: questionTypeSchema,
  bodySource: z.string(),
  bodyHtml: z.string().optional(),
  explanationSource: z.string(),
  explanationHtml: z.string().optional(),
  points: z.number().int().positive(),
  options: z.array(optionSchema),
});
export type Question = z.infer<typeof questionSchema>;

/** `QuizDTO`: a practice quiz has no limits, so the three limits are null there. */
export const quizSchema = z.object({
  mode: quizModeSchema,
  maxAttempts: z.number().int().positive().nullable(),
  timeLimitMinutes: z.number().int().positive().nullable(),
  shuffle: z.boolean(),
  passPercent: z.number().int().positive().nullable(),
  questions: z.array(questionSchema),
});
export type Quiz = z.infer<typeof quizSchema>;

/** `CriterionDTO`: one grading criterion of a homework rubric. */
export const criterionSchema = z.object({ id, description: z.string(), maxPoints: z.number().int().positive() });
export type Criterion = z.infer<typeof criterionSchema>;

/** `AttachmentDTO`: a document handed out with the homework brief. */
export const attachmentSchema = z.object({ mediaId: id, fileName: z.string(), sizeBytes: count });
export type Attachment = z.infer<typeof attachmentSchema>;

/** `HomeworkDTO`: `maxPoints` is the sum of the criteria; `briefHtml` only once published. */
export const homeworkSchema = z.object({
  briefSource: z.string(),
  briefHtml: z.string().optional(),
  passPercent: z.number().int().positive(),
  maxPoints: count,
  criteria: z.array(criterionSchema),
  attachments: z.array(attachmentSchema),
});
export type Homework = z.infer<typeof homeworkSchema>;

/** `LessonDTO`: type-specific fields are present only on that type; `markdownHtml` only once published. */
export const lessonSchema = z.object({
  id,
  unitId: id,
  lessonKey: z.string(),
  title: z.string(),
  type: lessonTypeSchema,
  required: z.boolean(),
  position: z.number().int(),
  durationSeconds: count.optional(),
  markdownSource: z.string().optional(),
  markdownHtml: z.string().optional(),
  videoMediaId: id.optional(),
  videoFileName: z.string().optional(),
  quiz: quizSchema.optional(),
  homework: homeworkSchema.optional(),
});
export type Lesson = z.infer<typeof lessonSchema>;

/**
 * `GET /stage-versions/{vid}` and every version action (clone, publish, archive, reorder). `lessons` is
 * flat in unit order, so `position` counts across units.
 */
export const stageVersionSchema = z.object({
  id,
  stageId: id,
  stageCode: z.string(),
  stageName: z.string(),
  versionNo,
  status: versionStatusSchema,
  publishedAt: isoDate.optional(),
  clonedFromVersionNo: versionNo.optional(),
  units: z.array(unitSchema),
  lessons: z.array(lessonSchema),
  usedBy: z.array(usedByRowSchema),
});
export type StageVersion = z.infer<typeof stageVersionSchema>;

/** `DELETE /stage-versions/{vid}`: deleting the last version deletes the stage too. */
export const deleteStageVersionSchema = z.object({ stageDeleted: z.boolean() });

/** `POST /stage-versions/{vid}/apply`: one row per course; a row with `error` was not applied (HTTP is still 200). */
export const applyResultsSchema = z.object({
  results: z.array(
    z.object({
      courseId: id,
      courseCode: z.string(),
      newVersionNo: versionNo.optional(),
      error: z.object({ code: z.string(), message: z.string() }).optional(),
    }),
  ),
});
export type ApplyResults = z.infer<typeof applyResultsSchema>;

/** `details` of a DRAFT_EXISTS error. */
export const draftExistsDetailsSchema = z.object({ draftVersionId: id, draftVersionNo: versionNo });

/** `POST /stages/markdown-preview`: the HTML learners would see, from the publish-time renderer. */
export const markdownPreviewSchema = z.object({ html: z.string() });
export type MarkdownPreview = z.infer<typeof markdownPreviewSchema>;

/** The part of `GET /course-versions/{vid}` the apply dialog previews: stage count and attached classes. */
export const courseVersionSummarySchema = z.object({
  id,
  versionNo,
  stages: z.array(z.object({ stageId: id })),
  classes: z.array(z.object({ code: z.string() })),
});
export type CourseVersionSummary = z.infer<typeof courseVersionSummarySchema>;
