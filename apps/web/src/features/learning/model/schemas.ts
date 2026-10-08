import { z } from "zod";
import { isoDate } from "@/shared/api/schemas";

/** Class start/end dates are calendar days ("2026-10-05"), not timestamps. */
const day = z.iso.date();
const count = z.number().int().nonnegative();

export const classStatusSchema = z.enum(["draft", "active", "ended"]);
export const readOnlyReasonSchema = z.enum(["draft", "ended"]);
export const lessonTypeSchema = z.enum(["markdown", "video"]);

export type ReadOnlyReason = z.infer<typeof readOnlyReasonSchema>;

export const nextLessonSchema = z.object({
  lessonId: z.string().min(1),
  title: z.string(),
  stageName: z.string(),
});

/** `ClassDTO`: the class on a "Lớp của tôi" card and at the top of the roadmap. */
export const classSchema = z.object({
  id: z.string().min(1),
  code: z.string(),
  name: z.string(),
  status: classStatusSchema,
  startDate: day,
  endDate: day,
  teacherName: z.string(),
  courseName: z.string(),
  courseVersionNo: z.number().int().positive(),
});

/** Course-wide progress: computed by the API over required lessons only, never recomputed here. */
const progressTotals = {
  percent: z.number().int().min(0).max(100),
  requiredDone: count,
  requiredTotal: count,
};

/** `GET /me/classes`: draft classes are listed; dropped memberships never are. */
export const myClassesSchema = z.object({
  items: z.array(
    classSchema.extend({
      ...progressTotals,
      nextLesson: nextLessonSchema.optional(),
      readOnlyReason: readOnlyReasonSchema.optional(),
    }),
  ),
});
export type MyClasses = z.infer<typeof myClassesSchema>;
export type MyClassItem = MyClasses["items"][number];

/** Lesson in the roadmap; `firstOpenedAt`/`completedAt` are absent until opened/completed. */
export const roadmapLessonSchema = z.object({
  id: z.string().min(1),
  title: z.string(),
  type: lessonTypeSchema,
  required: z.boolean(),
  position: z.number().int(),
  durationSeconds: count.optional(),
  firstOpenedAt: isoDate.optional(),
  completedAt: isoDate.optional(),
});
export type RoadmapLesson = z.infer<typeof roadmapLessonSchema>;

export const roadmapStageSchema = z.object({
  id: z.string().min(1),
  name: z.string(),
  position: z.number().int(),
  lessons: z.array(roadmapLessonSchema),
});
export type RoadmapStage = z.infer<typeof roadmapStageSchema>;

/** `GET /me/classes/{id}`: a draft class answers 200 with `readOnly` and `readOnlyReason: "draft"`. */
export const roadmapSchema = z.object({
  class: classSchema,
  ...progressTotals,
  readOnly: z.boolean(),
  readOnlyReason: readOnlyReasonSchema.optional(),
  nextLesson: nextLessonSchema.optional(),
  stages: z.array(roadmapStageSchema),
});
export type Roadmap = z.infer<typeof roadmapSchema>;

export const lessonLinkSchema = z.object({ lessonId: z.string().min(1), title: z.string() });
export type LessonLink = z.infer<typeof lessonLinkSchema>;

export const markdownContentSchema = z.object({ type: z.literal("markdown"), html: z.string() });
export const videoContentSchema = z.object({
  type: z.literal("video"),
  mediaId: z.string().min(1),
  url: z.string().min(1),
  expiresAt: isoDate,
});
export type MarkdownContent = z.infer<typeof markdownContentSchema>;
export type VideoContent = z.infer<typeof videoContentSchema>;

/** `GET /me/classes/{id}/lessons/{lid}`: records the first open (FR-31) when not yet recorded. */
export const lessonPageSchema = z.object({
  lesson: z.object({
    id: z.string().min(1),
    title: z.string(),
    type: lessonTypeSchema,
    required: z.boolean(),
    position: z.number().int(),
    durationSeconds: count.optional(),
    stage: z.object({ id: z.string().min(1), name: z.string() }),
  }),
  content: z.discriminatedUnion("type", [markdownContentSchema, videoContentSchema]),
  // firstOpenedAt stays null when the class has ended and the lesson was never opened.
  progress: z.object({ firstOpenedAt: isoDate.nullable(), completedAt: isoDate.optional() }),
  readOnly: z.boolean(),
  prev: lessonLinkSchema.optional(),
  next: lessonLinkSchema.optional(),
});
export type LessonPage = z.infer<typeof lessonPageSchema>;

/** `PUT …/completion`: `completedAt` is null after unticking. */
export const completionSchema = z.object({ completedAt: isoDate.nullable(), ...progressTotals });
export type Completion = z.infer<typeof completionSchema>;

/** `GET /media/{id}/url`. */
export const signedUrlSchema = z.object({ url: z.string().min(1), expiresAt: isoDate });
export type SignedUrl = z.infer<typeof signedUrlSchema>;
