import { z } from "zod";
import { isoDate } from "@/shared/api/schemas";

const id = z.string().min(1);
const versionNo = z.number().int().positive();
const count = z.number().int().nonnegative();

export const versionStatusSchema = z.enum(["draft", "published", "archived"]);
export const lessonTypeSchema = z.enum(["video", "markdown"]);

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

/** `LessonDTO`: type-specific fields are present only on that type; `markdownHtml` only once published. */
export const lessonSchema = z.object({
  id,
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
});
export type Lesson = z.infer<typeof lessonSchema>;

/** `GET /stage-versions/{vid}` and every version action (clone, publish, archive, reorder). */
export const stageVersionSchema = z.object({
  id,
  stageId: id,
  stageCode: z.string(),
  stageName: z.string(),
  versionNo,
  status: versionStatusSchema,
  publishedAt: isoDate.optional(),
  clonedFromVersionNo: versionNo.optional(),
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

/** `POST /media/uploads` (201). The upload URL is presigned: never log or display it. */
export const uploadTicketSchema = z.object({ mediaId: id, uploadUrl: z.string().min(1), expiresAt: isoDate });
export type UploadTicket = z.infer<typeof uploadTicketSchema>;

/** `POST /media/uploads/{id}/complete`. */
export const mediaSchema = z.object({
  id,
  kind: z.string(),
  fileName: z.string(),
  contentType: z.string(),
  sizeBytes: count,
  status: z.string(),
});
export type Media = z.infer<typeof mediaSchema>;

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
