import { z } from "zod";
import { isoDate } from "@/shared/api/schemas";

const count = z.number().int().nonnegative();
const percent = z.number().int().min(0).max(100);

export const quizModeSchema = z.enum(["practice", "assessment"]);
export const questionTypeSchema = z.enum(["single", "multi"]);
/** Review state of an assessment attempt; a practice attempt gets `confirmed` with its first comment. */
export const reviewStatusSchema = z.enum(["pending", "confirmed", "superseded"]);

export type QuizMode = z.infer<typeof quizModeSchema>;
export type QuestionType = z.infer<typeof questionTypeSchema>;
export type ReviewStatus = z.infer<typeof reviewStatusSchema>;

/** A submitted attempt in the start screen's history; `passed` and `reviewStatus` only for assessments. */
export const attemptSummarySchema = z.object({
  id: z.string().min(1),
  no: z.number().int().positive(),
  submittedAt: isoDate,
  autoSubmitted: z.boolean(),
  score: count,
  maxScore: count,
  percent,
  passed: z.boolean().optional(),
  reviewStatus: reviewStatusSchema.optional(),
});
export type AttemptSummary = z.infer<typeof attemptSummarySchema>;

/**
 * `GET /me/classes/{id}/lessons/{lid}/attempts`: the start screen. `used` counts the open attempt too; `limit`,
 * `timeLimitMinutes` and `passPercent` are absent when they do not apply. `blockedReason` says why `canStart` is
 * false. Submitted attempts come newest first.
 */
export const quizOverviewSchema = z.object({
  mode: quizModeSchema,
  questionCount: count,
  maxScore: count,
  passPercent: percent.optional(),
  timeLimitMinutes: z.number().int().positive().optional(),
  limit: z.number().int().positive().optional(),
  used: count,
  granted: count,
  openAttemptId: z.string().min(1).optional(),
  best: z.object({ attemptId: z.string().min(1), percent, passed: z.boolean().optional() }).optional(),
  attempts: z.array(attemptSummarySchema),
  canStart: z.boolean(),
  blockedReason: z.string().optional(),
});
export type QuizOverview = z.infer<typeof quizOverviewSchema>;

/**
 * A question while the attempt is open: text, points, options in the student's order and the saved answer.
 * Deliberately no correctness field: the key only comes with a practice result.
 */
export const openQuestionSchema = z.object({
  id: z.string().min(1),
  type: questionTypeSchema,
  bodyHtml: z.string(),
  points: count,
  options: z.array(z.object({ id: z.string().min(1), body: z.string() })),
  selectedOptionIds: z.array(z.string().min(1)),
});
export type OpenQuestion = z.infer<typeof openQuestionSchema>;

/** An attempt in progress: questions in the student's order, the deadline (null without a time limit) and the server's clock. */
export const openAttemptSchema = z.object({
  id: z.string().min(1),
  status: z.literal("open"),
  mode: quizModeSchema,
  no: z.number().int().positive(),
  limit: z.number().int().positive().nullable(),
  deadlineAt: isoDate.nullable(),
  serverNow: isoDate,
  savedAt: isoDate.nullable(),
  questions: z.array(openQuestionSchema),
});
export type OpenAttempt = z.infer<typeof openAttemptSchema>;

/** `POST …/attempts`: 201 with a new attempt, or 200 with the open one (`resumed`). */
export const startedSchema = z.object({ resumed: z.boolean(), attempt: openAttemptSchema });
export type Started = z.infer<typeof startedSchema>;

/** `PUT /me/attempts/{aid}/answers/{qid}`. */
export const savedSchema = z.object({ savedAt: isoDate });

const submittedBase = {
  id: z.string().min(1),
  status: z.literal("submitted"),
  no: z.number().int().positive(),
  startedAt: isoDate,
  submittedAt: isoDate,
  autoSubmitted: z.boolean(),
  score: count,
  maxScore: count,
  percent,
};

/** A practice result: every question in authoring order with the key, the explanation and the teacher's note. */
export const practiceResultSchema = z.object({
  ...submittedBase,
  mode: z.literal("practice"),
  questions: z.array(
    z.object({
      id: z.string().min(1),
      type: questionTypeSchema,
      bodyHtml: z.string(),
      points: count,
      pointsAwarded: count,
      correct: z.boolean(),
      explanationHtml: z.string().optional(),
      note: z.string().optional(),
      options: z.array(
        z.object({ id: z.string().min(1), body: z.string(), isCorrect: z.boolean(), selected: z.boolean() }),
      ),
    }),
  ),
  review: z
    .object({ comment: z.string(), reviewerName: z.string(), reviewedAt: isoDate, updated: z.boolean() })
    .optional(),
});
export type PracticeResult = z.infer<typeof practiceResultSchema>;

/**
 * An assessment result: score and verdict only, never the key. The review's notes name each question by its
 * position in the quiz and its text.
 */
export const assessmentResultSchema = z.object({
  ...submittedBase,
  mode: z.literal("assessment"),
  passed: z.boolean().nullable(),
  passPercent: percent.nullable(),
  reviewStatus: reviewStatusSchema.nullable(),
  best: z.object({ percent }).optional(),
  attemptsLeft: count.nullable(),
  review: z
    .object({
      comment: z.string(),
      reviewerName: z.string(),
      reviewedAt: isoDate,
      updated: z.boolean(),
      notes: z.array(
        z.object({ questionId: z.string().min(1), position: z.number().int().positive(), bodyHtml: z.string(), note: z.string() }),
      ),
    })
    .optional(),
});
export type AssessmentResult = z.infer<typeof assessmentResultSchema>;

export const quizResultSchema = z.discriminatedUnion("mode", [practiceResultSchema, assessmentResultSchema]);
export type QuizResult = z.infer<typeof quizResultSchema>;

/** `GET /me/attempts/{aid}`: the open attempt, or its result once submitted. */
export const attemptSchema = z.union([openAttemptSchema, quizResultSchema]);
export type Attempt = z.infer<typeof attemptSchema>;
