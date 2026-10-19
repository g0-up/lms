import { z } from "zod";
import { isoDate } from "@/shared/api/schemas";

const count = z.number().int().nonnegative();
const percent = z.number().int().min(0).max(100);

/** Tone of a status label, the same set as the badge variants (`optional` reads as the subtle badge). */
export const toneSchema = z.enum(["ok", "info", "warn", "danger", "optional"]);
export type Tone = z.infer<typeof toneSchema>;

export const lessonTypeSchema = z.enum(["markdown", "video", "quiz", "homework"]);

const classSchema = z.object({
  id: z.string().min(1),
  code: z.string(),
  name: z.string(),
  status: z.enum(["draft", "active", "ended"]),
});

/** A stage of the class's course version; `stageVersionId` addresses it in the routes. */
export const stageRefSchema = z.object({
  stageVersionId: z.string().min(1),
  stageId: z.string().min(1),
  code: z.string(),
  name: z.string(),
  versionNo: z.number().int().positive(),
});
export type StageRef = z.infer<typeof stageRefSchema>;

/**
 * The API's verdict on one lesson: `done` counts towards the stage, `label`/`tone` make its badge, `detail` its
 * short sentence; `overdue` and `rework` flag a homework that needs the student.
 */
export const itemStatusSchema = z.object({
  done: z.boolean(),
  label: z.string(),
  tone: toneSchema,
  detail: z.string(),
  overdue: z.boolean(),
  rework: z.boolean(),
});

const submissionSchema = z.object({
  attemptNo: z.number().int().positive(),
  status: z.enum(["pending", "passed", "rework", "superseded"]),
  isLate: z.boolean(),
  submittedAt: isoDate,
  total: count.nullable(),
  max: count.nullable(),
});

/** Homework facts: the deadline, how many submissions and the latest one. */
export const homeworkFactsSchema = z.object({
  dueAt: isoDate.nullable(),
  submissionCount: count,
  latest: submissionSchema.nullable(),
});
export type HomeworkFacts = z.infer<typeof homeworkFactsSchema>;

const scoreSchema = z.object({ score: count, max: count, percent });

/** Quiz facts: the limit (null without one), submitted attempts, the best and the latest score. */
export const quizFactsSchema = z.object({
  mode: z.enum(["practice", "assessment"]),
  passPercent: percent.nullable(),
  limit: z.number().int().positive().nullable(),
  attemptsUsed: count,
  submitted: count,
  open: z.boolean(),
  best: scoreSchema.nullable(),
  latest: scoreSchema.extend({ reviewStatus: z.enum(["pending", "confirmed", "superseded"]).nullable() }).nullable(),
});
export type QuizFacts = z.infer<typeof quizFactsSchema>;

/** A lesson of the stage with its status; `homework` and `quiz` come with lessons of that type only. */
export const itemSchema = z.object({
  lessonId: z.string().min(1),
  title: z.string(),
  type: lessonTypeSchema,
  required: z.boolean(),
  firstOpenedAt: isoDate.nullable(),
  completedAt: isoDate.nullable(),
  status: itemStatusSchema,
  homework: homeworkFactsSchema.optional(),
  quiz: quizFactsSchema.optional(),
});
export type SummaryItem = z.infer<typeof itemSchema>;

export const unitSchema = z.object({
  id: z.string().min(1),
  title: z.string(),
  position: z.number().int(),
  items: z.array(itemSchema),
});
export type SummaryUnit = z.infer<typeof unitSchema>;

/** The teacher's stage review; `revision` counts saves, so `revision - 1` edits. */
export const reviewSchema = z.object({
  body: z.string(),
  authorName: z.string(),
  createdAt: isoDate,
  updatedAt: isoDate,
  revision: z.number().int().positive(),
});
export type StageReview = z.infer<typeof reviewSchema>;

/** One stage for one member: required lessons done, what is still missing and the review. */
export const stageSummarySchema = stageRefSchema.extend({
  units: z.array(unitSchema),
  required: count,
  done: count,
  percent,
  complete: z.boolean(),
  missing: z.array(z.object({ lessonId: z.string().min(1), title: z.string(), unitTitle: z.string() })),
  rework: z.boolean(),
  overdue: z.boolean(),
  review: reviewSchema.nullable(),
});
export type StageSummary = z.infer<typeof stageSummarySchema>;

/** An assessment's best attempt (null before any) and whether the lesson is passed. */
export const assessmentScoreSchema = z.object({
  lessonId: z.string().min(1),
  title: z.string(),
  bestPercent: percent.nullable(),
  passed: z.boolean(),
});
export type AssessmentScore = z.infer<typeof assessmentScoreSchema>;

/** The latest submission of a homework: its points once graded (null before) and its state. */
export const homeworkScoreSchema = z.object({
  lessonId: z.string().min(1),
  title: z.string(),
  total: count.nullable(),
  max: count.nullable(),
  status: z.enum(["not_submitted", "pending", "passed", "rework"]),
});
export type HomeworkScore = z.infer<typeof homeworkScoreSchema>;

/** One stage in the course-wide summary: complete or how many required lessons are missing, and its scores. */
export const courseStageSchema = z.object({
  stageVersionId: z.string().min(1),
  name: z.string(),
  complete: z.boolean(),
  missingCount: count,
  assessments: z.array(assessmentScoreSchema),
  homework: z.array(homeworkScoreSchema),
});
export type CourseStage = z.infer<typeof courseStageSchema>;

/** The course-wide summary: stages complete out of all, then stage by stage. There is no course-wide score. */
export const courseSummarySchema = z.object({
  stagesComplete: count,
  stagesTotal: count,
  stages: z.array(courseStageSchema),
});
export type CourseSummary = z.infer<typeof courseSummarySchema>;

/** `GET /me/classes/{id}/summary`: the signed-in student's course summary, stage by stage. */
export const learnerSummarySchema = z.object({
  class: classSchema,
  stages: z.array(stageSummarySchema),
  coursePercent: percent,
  requiredDone: count,
  requiredTotal: count,
  course: courseSummarySchema,
});
export type LearnerSummary = z.infer<typeof learnerSummarySchema>;

/** One member × stage cell; `missing` is the number of required lessons still to do. */
export const matrixCellSchema = z.object({
  stageVersionId: z.string().min(1),
  complete: z.boolean(),
  done: count,
  required: count,
  missing: count,
  rework: z.boolean(),
  overdue: z.boolean(),
  hasReview: z.boolean(),
});
export type MatrixCell = z.infer<typeof matrixCellSchema>;

export const matrixRowSchema = z.object({
  memberId: z.string().min(1),
  name: z.string(),
  email: z.string(),
  coursePercent: percent,
  cells: z.array(matrixCellSchema),
});
export type MatrixRow = z.infer<typeof matrixRowSchema>;

/** `GET /classes/{id}/summary`: every active member against every stage of the class. */
export const classMatrixSchema = z.object({
  class: classSchema,
  stages: z.array(stageRefSchema),
  rows: z.array(matrixRowSchema),
});
export type ClassMatrix = z.infer<typeof classMatrixSchema>;

/** `GET /classes/{id}/members/{mid}/stages/{svid}/summary`: one member's stage, as the drawer shows it. */
export const memberStageSchema = z.object({
  class: classSchema,
  member: z.object({ id: z.string().min(1), name: z.string(), email: z.string(), status: z.string() }),
  summary: stageSummarySchema,
});
export type MemberStage = z.infer<typeof memberStageSchema>;
