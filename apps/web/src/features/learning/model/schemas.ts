import { z } from "zod";
import { isoDate } from "@/shared/api/schemas";

/** Class start/end dates are calendar days ("2026-10-05"), not timestamps. */
const day = z.iso.date();
const count = z.number().int().nonnegative();

export const classStatusSchema = z.enum(["draft", "active", "ended"]);
export const readOnlyReasonSchema = z.enum(["draft", "ended"]);
export const lessonTypeSchema = z.enum(["markdown", "video", "quiz", "homework"]);

export type ReadOnlyReason = z.infer<typeof readOnlyReasonSchema>;
export type LessonType = z.infer<typeof lessonTypeSchema>;

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

/** Status label of a quiz or homework ("Chờ chấm", "Quá hạn", "Tốt nhất 80%"…); absent for videos and readings. */
export const lessonStatusSchema = z.object({
  label: z.string().min(1),
  tone: z.enum(["ok", "info", "warn", "danger", "optional"]),
});
export type LessonStatus = z.infer<typeof lessonStatusSchema>;

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
  unitId: z.string().min(1),
  status: lessonStatusSchema.optional(),
});
export type RoadmapLesson = z.infer<typeof roadmapLessonSchema>;

/**
 * A bài học of the roadmap with its lessons. The API counts its required lessons; `optional` means it
 * has none, and `complete` means every required one is done.
 */
export const roadmapUnitSchema = z.object({
  id: z.string().min(1),
  title: z.string(),
  position: z.number().int(),
  requiredDone: count,
  requiredTotal: count,
  complete: z.boolean(),
  optional: z.boolean(),
  lessons: z.array(roadmapLessonSchema),
});
export type RoadmapUnit = z.infer<typeof roadmapUnitSchema>;

/** A stage of the roadmap: its units, and the same lessons flat in unit order. */
export const roadmapStageSchema = z.object({
  id: z.string().min(1),
  name: z.string(),
  position: z.number().int(),
  unitCount: count,
  lessonCount: count,
  requiredDone: count,
  requiredTotal: count,
  units: z.array(roadmapUnitSchema),
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
export const criterionSchema = z.object({
  id: z.string().min(1),
  description: z.string(),
  maxPoints: z.number().int().nonnegative(),
});
export const fileSchema = z.object({
  mediaId: z.string().min(1),
  fileName: z.string(),
  sizeBytes: z.number().int().nonnegative(),
});
/** Homework brief (sanitized HTML), pass threshold, rubric and attached files, in order. */
export const homeworkContentSchema = z.object({
  type: z.literal("homework"),
  homework: z.object({
    briefHtml: z.string(),
    passPercent: z.number().int().min(0).max(100),
    maxPoints: z.number().int().nonnegative(),
    criteria: z.array(criterionSchema),
    attachments: z.array(fileSchema),
  }),
});
/** A quiz lesson carries no content here: its questions come with an attempt. */
export const quizContentSchema = z.object({ type: z.literal("quiz") });
export type MarkdownContent = z.infer<typeof markdownContentSchema>;
export type VideoContent = z.infer<typeof videoContentSchema>;
export type HomeworkContent = z.infer<typeof homeworkContentSchema>;
export type Criterion = z.infer<typeof criterionSchema>;
export type SubmittedFile = z.infer<typeof fileSchema>;

/**
 * `GET /me/classes/{id}/lessons/{lid}`: records the first open (FR-31) when not yet recorded.
 * `crossesUnit` is true when `next` starts another unit, whose title is `nextUnitTitle`.
 */
export const lessonPageSchema = z.object({
  lesson: z.object({
    id: z.string().min(1),
    title: z.string(),
    type: lessonTypeSchema,
    required: z.boolean(),
    position: z.number().int(),
    durationSeconds: count.optional(),
    stage: z.object({ id: z.string().min(1), name: z.string(), position: z.number().int() }),
    unit: z.object({ id: z.string().min(1), title: z.string(), position: z.number().int() }),
    status: lessonStatusSchema.optional(),
  }),
  content: z.discriminatedUnion("type", [
    markdownContentSchema,
    videoContentSchema,
    homeworkContentSchema,
    quizContentSchema,
  ]),
  // firstOpenedAt stays null when the class has ended and the lesson was never opened.
  progress: z.object({ firstOpenedAt: isoDate.nullable(), completedAt: isoDate.optional() }),
  readOnly: z.boolean(),
  prev: lessonLinkSchema.optional(),
  next: lessonLinkSchema.optional(),
  crossesUnit: z.boolean(),
  nextUnitTitle: z.string().optional(),
});
export type LessonPage = z.infer<typeof lessonPageSchema>;

/** `PUT …/completion`: `completedAt` is null after unticking. */
export const completionSchema = z.object({ completedAt: isoDate.nullable(), ...progressTotals });
export type Completion = z.infer<typeof completionSchema>;

/** `GET /media/{id}/url`. */
export const signedUrlSchema = z.object({ url: z.string().min(1), expiresAt: isoDate });
export type SignedUrl = z.infer<typeof signedUrlSchema>;

export const submissionStatusSchema = z.enum(["pending", "passed", "rework", "superseded"]);
export type SubmissionStatus = z.infer<typeof submissionStatusSchema>;

/** One grading of a submission; each score carries its criterion's text and maximum at grading time. */
export const gradeSchema = z.object({
  id: z.string().min(1),
  totalPoints: z.number().int().nonnegative(),
  maxPoints: z.number().int().nonnegative(),
  result: z.enum(["passed", "rework"]),
  comment: z.string(),
  gradedBy: z.string(),
  gradedAt: isoDate,
  scores: z.array(
    z.object({
      criterionId: z.string().min(1),
      description: z.string(),
      points: z.number().int().nonnegative(),
      maxPoints: z.number().int().nonnegative(),
      comment: z.string(),
    }),
  ),
});
export type Grade = z.infer<typeof gradeSchema>;

/** A submission; `grade` is the current grading, null while waiting. */
export const submissionSchema = z.object({
  id: z.string().min(1),
  no: z.number().int().positive(),
  submittedAt: isoDate,
  isLate: z.boolean(),
  repoUrl: z.string().nullable(),
  commitRef: z.string().nullable(),
  files: z.array(fileSchema),
  status: submissionStatusSchema,
  grade: gradeSchema.nullable(),
});
export type Submission = z.infer<typeof submissionSchema>;

/**
 * `GET`/`POST /me/classes/{id}/lessons/{lid}/submissions`: newest submission first. `lockedReason` is the
 * message a new submission would be refused with, null when `canSubmit`.
 */
export const mySubmissionsSchema = z.object({
  lessonId: z.string().min(1),
  deadline: isoDate.nullable(),
  status: z.enum(["notSubmitted", "pending", "passed", "rework"]),
  canSubmit: z.boolean(),
  lockedReason: z.string().nullable(),
  maxFiles: z.number().int().positive(),
  submissions: z.array(submissionSchema),
});
export type MySubmissions = z.infer<typeof mySubmissionsSchema>;
