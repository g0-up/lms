import { z } from "zod";
import { isoDate } from "@/shared/api/schemas";
import { versionStatusSchema } from "@/features/stages";

const id = z.string().min(1);
const versionNo = z.number().int().positive();
const count = z.number().int().nonnegative();

export const classStatusSchema = z.enum(["draft", "active", "ended"]);

/** `VersionItemDTO`: one version on the course list (the API sorts newest first). */
const courseVersionItemSchema = z.object({
  id,
  versionNo,
  status: versionStatusSchema,
  publishedAt: isoDate.optional(),
  stageCount: count,
  outdatedStageCount: count,
});

/** A class attached to one of the course's versions. */
const classUsingSchema = z.object({ classId: id, code: z.string(), name: z.string(), versionNo });

/** `GET /courses`. */
export const courseListSchema = z.object({
  items: z.array(
    z.object({
      id,
      code: z.string(),
      name: z.string(),
      description: z.string().optional(),
      latestVersionNo: versionNo,
      latestPublishedNo: versionNo.optional(),
      draftVersionId: id.optional(),
      versions: z.array(courseVersionItemSchema),
      classesUsing: z.array(classUsingSchema),
    }),
  ),
});
export type CourseList = z.infer<typeof courseListSchema>;
export type CourseListItem = CourseList["items"][number];

/** `GET /courses/{id}` and `POST /courses`. */
export const courseDetailSchema = z.object({
  id,
  code: z.string(),
  name: z.string(),
  description: z.string().optional(),
  versions: z.array(courseVersionItemSchema.extend({ classCount: count, clonedFromVersionNo: versionNo.optional() })),
  classesUsing: z.array(classUsingSchema),
});
export type CourseDetail = z.infer<typeof courseDetailSchema>;

/** One stage version in a course version; `outdated` when a newer version of the stage is published. */
export const courseStageSchema = z.object({
  position: z.number().int(),
  stageId: id,
  stageCode: z.string(),
  stageName: z.string(),
  stageVersionId: id,
  stageVersionNo: versionNo,
  lessonCount: count,
  requiredCount: count,
  latestPublishedNo: versionNo.optional(),
  outdated: z.boolean(),
});
export type CourseStage = z.infer<typeof courseStageSchema>;

/** A class attached to the course version being viewed. */
export const versionClassSchema = z.object({
  classId: id,
  code: z.string(),
  name: z.string(),
  status: classStatusSchema,
  memberCount: count,
});
export type VersionClass = z.infer<typeof versionClassSchema>;

/** `GET /course-versions/{vid}`, `PUT .../stages` and every version action (clone, publish, archive). */
export const courseVersionSchema = z.object({
  id,
  courseId: id,
  courseCode: z.string(),
  courseName: z.string(),
  versionNo,
  status: versionStatusSchema,
  publishedAt: isoDate.optional(),
  clonedFromVersionNo: versionNo.optional(),
  stages: z.array(courseStageSchema),
  classes: z.array(versionClassSchema),
  newerPublished: z.boolean(),
});
export type CourseVersion = z.infer<typeof courseVersionSchema>;

/** `DELETE /course-versions/{vid}`: deleting the last version deletes the course too. */
export const deleteCourseVersionSchema = z.object({ courseDeleted: z.boolean() });
