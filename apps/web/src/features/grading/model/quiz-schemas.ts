import { z } from "zod";
import { isoDate } from "@/shared/api/schemas";
import { classRefSchema, classStatusSchema, lessonRefSchema, personSchema, queueItemSchema } from "./schemas";

const id = z.string().min(1);
const count = z.number().int().nonnegative();
const points = z.number().int().nonnegative();
const percent = z.number().int().min(0).max(100);
const attemptNo = z.number().int().positive();
const minutes = z.number().int().positive();

export const quizModeSchema = z.enum(["practice", "assessment"]);

/** Practice attempts only ever reach `confirmed` (commented); `superseded` once a newer assessment attempt came in. */
export const quizReviewStatusSchema = z.enum(["pending", "confirmed", "superseded"]);

const stageRefSchema = z.object({ id, name: z.string() });

/** One teacher review of an attempt: the general comment and the notes on some questions. */
export const quizReviewSchema = z.object({
  id,
  reviewer: personSchema,
  createdAt: isoDate,
  comment: z.string(),
  notes: z.array(z.object({ questionId: id, note: z.string() })),
});
export type QuizReview = z.infer<typeof quizReviewSchema>;

/** A question as the student saw it, with the key, what they picked and the points it earned. */
export const reviewQuestionSchema = z.object({
  id,
  type: z.enum(["single", "multi"]),
  bodyHtml: z.string(),
  points,
  pointsAwarded: points,
  status: z.enum(["correct", "wrong", "blank"]),
  options: z.array(z.object({ id, body: z.string(), isCorrect: z.boolean(), selected: z.boolean() })),
});
export type ReviewQuestion = z.infer<typeof reviewQuestionSchema>;

/** A submitted attempt of the same student and quiz, with its current review. */
export const staffAttemptSchema = z.object({
  id,
  no: attemptNo,
  submittedAt: isoDate,
  autoSubmitted: z.boolean(),
  score: points,
  maxScore: points,
  percent,
  passed: z.boolean().nullable(),
  reviewStatus: quizReviewStatusSchema.nullable(),
  review: quizReviewSchema.nullable(),
});
export type StaffAttempt = z.infer<typeof staffAttemptSchema>;

/**
 * `GET /quiz-attempts/{aid}`: a submitted attempt for its teacher. Questions come in the order the student saw;
 * `reviews` and `attempts` are newest first, the first review being the current one. `passed`/`passPercent` are
 * null for practice, `limit` when attempts are unlimited, `reviewStatus` before a practice attempt's first comment.
 */
export const attemptDetailSchema = z.object({
  id,
  student: z.object({ memberId: id, userId: id, name: z.string(), email: z.string(), memberStatus: z.string() }),
  class: classRefSchema.extend({ status: classStatusSchema }),
  stage: stageRefSchema,
  lesson: lessonRefSchema,
  mode: quizModeSchema,
  no: attemptNo,
  limit: z.number().int().positive().nullable(),
  startedAt: isoDate,
  submittedAt: isoDate,
  autoSubmitted: z.boolean(),
  minutes,
  timeLimitMinutes: minutes.nullable(),
  score: points,
  maxScore: points,
  percent,
  correctCount: count,
  questionCount: count,
  passed: z.boolean().nullable(),
  passPercent: percent.nullable(),
  reviewStatus: quizReviewStatusSchema.nullable(),
  isLatest: z.boolean(),
  latestAttemptId: id,
  questions: z.array(reviewQuestionSchema),
  reviews: z.array(quizReviewSchema),
  attempts: z.array(staffAttemptSchema),
});
export type AttemptDetail = z.infer<typeof attemptDetailSchema>;

/**
 * `POST /quiz-attempts/{aid}/reviews`. `nextQueueItem` is the oldest item left in the grader's queue after the
 * first confirmation of an assessment attempt; null for practice, an edit (`updated`) or an empty queue.
 */
export const reviewOutcomeSchema = z.object({
  review: quizReviewSchema,
  reviewStatus: quizReviewStatusSchema,
  updated: z.boolean(),
  nextQueueItem: queueItemSchema.nullable(),
});
export type ReviewOutcome = z.infer<typeof reviewOutcomeSchema>;

/**
 * A member against one quiz: practice is `todo`, `inProgress` or `done`; an assessment `passed` on the best
 * attempt, else `failed` while attempts are left and `outOfAttempts` once they are used up.
 */
export const memberQuizStatusSchema = z.enum(["todo", "inProgress", "done", "passed", "failed", "outOfAttempts"]);
export type MemberQuizStatus = z.infer<typeof memberQuizStatusSchema>;

export const memberQuizSchema = z.object({
  memberId: id,
  name: z.string(),
  used: count,
  limit: z.number().int().positive().nullable(),
  granted: count,
  bestPercent: percent.nullable(),
  status: memberQuizStatusSchema,
  latest: z
    .object({
      attemptId: id,
      no: attemptNo,
      percent,
      submittedAt: isoDate,
      reviewStatus: quizReviewStatusSchema.nullable(),
    })
    .nullable(),
});
export type MemberQuiz = z.infer<typeof memberQuizSchema>;

/** One extra-attempts grant to a member on an assessment; also the answer of `POST …/members/{mid}/grants`. */
export const attemptGrantSchema = z.object({
  id,
  memberId: id,
  lessonId: id,
  studentName: z.string(),
  extraAttempts: z.number().int().min(1).max(3),
  reason: z.string(),
  grantedByName: z.string(),
  grantedAt: isoDate,
});
export type AttemptGrant = z.infer<typeof attemptGrantSchema>;

/**
 * Share of submitted attempts (superseded ones included) of active members that got each question right;
 * `correctPercent` is null while the quiz has no submitted attempt. `bodyText` is plain text, never HTML.
 */
export const questionStatsSchema = z.object({
  attempts: count,
  rows: z.array(
    z.object({
      questionId: id,
      position: z.number().int().positive(),
      bodyText: z.string(),
      correctPercent: percent.nullable(),
    }),
  ),
});
export type QuestionStats = z.infer<typeof questionStatsSchema>;

/**
 * `GET /classes/{id}/quizzes`: every quiz of the class with its settings, counts over active members and each
 * member's state. The settings an assessment alone has are absent on practice; `grants` is newest first.
 */
export const classQuizzesSchema = z.object({
  class: classRefSchema.extend({ status: classStatusSchema }),
  items: z.array(
    z.object({
      lesson: lessonRefSchema,
      stage: stageRefSchema,
      mode: quizModeSchema,
      maxAttempts: z.number().int().positive().optional(),
      timeLimitMinutes: minutes.optional(),
      passPercent: percent.optional(),
      questionCount: count,
      stats: z.object({
        attempted: count,
        members: count,
        passed: count,
        pending: count,
        avgBestPercent: percent.nullable(),
        submittedAttempts: count,
      }),
      members: z.array(memberQuizSchema),
      grants: z.array(attemptGrantSchema),
      questionStats: questionStatsSchema,
    }),
  ),
});
export type ClassQuizzes = z.infer<typeof classQuizzesSchema>;
export type ClassQuizItem = ClassQuizzes["items"][number];
