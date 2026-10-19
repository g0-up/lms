import { z } from "zod";
import { isoDate } from "@/shared/api/schemas";

const id = z.string().min(1);
const count = z.number().int().nonnegative();
const points = z.number().int().nonnegative();
const attemptNo = z.number().int().positive();

export const queueKindSchema = z.enum(["homework", "quiz"]);
export type QueueKind = z.infer<typeof queueKindSchema>;

export const personSchema = z.object({ id, name: z.string() });
export const classRefSchema = z.object({ id, code: z.string(), name: z.string() });
export const lessonRefSchema = z.object({ id, title: z.string() });

/**
 * `QueueItemDTO`: a homework submission waiting for a grade or an assessment quiz attempt waiting for
 * confirmation. `scorePercent` is the quiz attempt's total, null for homework; `isLate` only means something
 * for homework and `autoSubmitted` only for quizzes.
 */
export const queueItemSchema = z.object({
  kind: queueKindSchema,
  itemId: id,
  submittedAt: isoDate,
  no: attemptNo,
  isLate: z.boolean(),
  autoSubmitted: z.boolean(),
  scorePercent: z.number().int().min(0).max(100).nullable(),
  student: personSchema,
  class: classRefSchema,
  lesson: lessonRefSchema,
  stage: z.object({ code: z.string(), name: z.string() }),
  waitedHours: count,
  overdue: z.boolean(),
});
export type QueueItem = z.infer<typeof queueItemSchema>;

/**
 * `GET /grading/queue`. `total`, `quizPending` and `overdueCount` count the whole queue of the viewer, not the
 * filtered `items`. The filter options only list what the viewer may grade.
 */
export const queueSchema = z.object({
  items: z.array(queueItemSchema),
  total: count,
  quizPending: count,
  overdueCount: count,
  filters: z.object({
    classes: z.array(classRefSchema),
    lessons: z.array(z.object({ id, title: z.string(), kind: queueKindSchema })),
  }),
});
export type Queue = z.infer<typeof queueSchema>;

/** `RecentDTO`: a homework grading or quiz confirmation; `points`/`maxPoints` of the submission or attempt. */
export const recentSchema = z.object({
  kind: queueKindSchema,
  itemId: id,
  at: isoDate,
  by: personSchema,
  student: personSchema,
  class: classRefSchema,
  lesson: lessonRefSchema,
  no: attemptNo,
  points,
  maxPoints: points,
  passed: z.boolean(),
});
export type Recent = z.infer<typeof recentSchema>;

export const recentListSchema = z.object({ items: z.array(recentSchema) });

export const submissionStatusSchema = z.enum(["pending", "passed", "rework", "superseded"]);

const fileSchema = z.object({ mediaId: id, fileName: z.string(), sizeBytes: count });

/** One grading; each score keeps its criterion's text and maximum at grading time. */
export const gradeSchema = z.object({
  id,
  totalPoints: points,
  maxPoints: points,
  result: z.enum(["passed", "rework"]),
  comment: z.string(),
  gradedBy: z.string(),
  gradedAt: isoDate,
  scores: z.array(
    z.object({ criterionId: id, description: z.string(), points, maxPoints: points, comment: z.string() }),
  ),
});
export type Grade = z.infer<typeof gradeSchema>;

/** A submission; `grade` is the current grading, `grades` every grading newest first (absent when none). */
export const submissionSchema = z.object({
  id,
  no: attemptNo,
  submittedAt: isoDate,
  isLate: z.boolean(),
  repoUrl: z.string().nullable(),
  commitRef: z.string().nullable(),
  files: z.array(fileSchema),
  status: submissionStatusSchema,
  grade: gradeSchema.nullable(),
  grades: z.array(gradeSchema).default([]),
});
export type Submission = z.infer<typeof submissionSchema>;

export const criterionSchema = z.object({ id, description: z.string(), maxPoints: points });
export type Criterion = z.infer<typeof criterionSchema>;

export const classStatusSchema = z.enum(["draft", "active", "ended"]);

/** `GET /submissions/{sid}`: the submission to grade, its brief and rubric, and the student's history newest first. */
export const submissionDetailSchema = z.object({
  submission: submissionSchema,
  student: z.object({
    memberId: id,
    userId: id,
    name: z.string(),
    email: z.string(),
    memberStatus: z.string(),
  }),
  class: classRefSchema.extend({ status: classStatusSchema }),
  stage: z.object({ id, name: z.string() }),
  lesson: z.object({
    id,
    title: z.string(),
    briefHtml: z.string().nullable(),
    passPercent: z.number().int().min(0).max(100),
    maxPoints: points,
    criteria: z.array(criterionSchema),
  }),
  deadline: isoDate.nullable(),
  isLatest: z.boolean(),
  latestSubmissionId: id,
  history: z.array(submissionSchema),
});
export type SubmissionDetail = z.infer<typeof submissionDetailSchema>;

/**
 * `POST /submissions/{sid}/grades`. `nextQueueItem` is the oldest item left in the grader's queue after a first
 * grading; null after a regrade (`updated`) or when the queue is empty.
 */
export const gradeOutcomeSchema = z.object({
  grade: gradeSchema,
  submissionStatus: z.enum(["passed", "rework"]),
  updated: z.boolean(),
  nextQueueItem: queueItemSchema.nullable(),
});
export type GradeOutcome = z.infer<typeof gradeOutcomeSchema>;

/** Status of a member for one homework: their latest submission's status, or not submitted yet. */
export const memberHomeworkStatusSchema = z.enum(["notSubmitted", "pending", "passed", "rework"]);
export type MemberHomeworkStatus = z.infer<typeof memberHomeworkStatusSchema>;

export const memberHomeworkSchema = z.object({
  memberId: id,
  name: z.string(),
  status: memberHomeworkStatusSchema,
  latest: z
    .object({
      submissionId: id,
      no: attemptNo,
      submittedAt: isoDate,
      isLate: z.boolean(),
      hasRepo: z.boolean(),
      fileCount: count,
      totalPoints: points.nullable(),
      maxPoints: points.nullable(),
    })
    .nullable(),
  waitingDays: count.nullable(),
});
export type MemberHomework = z.infer<typeof memberHomeworkSchema>;

/** `GET /classes/{id}/homework`: every homework of the class with its deadline, counts and active members. */
export const classHomeworkSchema = z.object({
  class: classRefSchema.extend({ status: classStatusSchema }),
  items: z.array(
    z.object({
      lesson: lessonRefSchema,
      stage: z.object({ id, name: z.string() }),
      maxPoints: points,
      passPercent: z.number().int().min(0).max(100),
      deadline: isoDate.nullable(),
      stats: z.object({
        submitted: count,
        pending: count,
        passed: count,
        rework: count,
        overdueNotSubmitted: count,
      }),
      members: z.array(memberHomeworkSchema),
    }),
  ),
});
export type ClassHomework = z.infer<typeof classHomeworkSchema>;
export type ClassHomeworkItem = ClassHomework["items"][number];

/** `PUT /classes/{id}/deadlines/{lid}`. */
export const deadlineSchema = z.object({ lessonId: id, dueAt: isoDate.nullable() });
export type Deadline = z.infer<typeof deadlineSchema>;
