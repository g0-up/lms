import { http } from "@/shared/api/http";
import type { GradeBody } from "../model/grade-form";
import type { GrantBody } from "../model/grant-form";
import {
  attemptDetailSchema,
  attemptGrantSchema,
  classQuizzesSchema,
  reviewOutcomeSchema,
  type AttemptDetail,
  type AttemptGrant,
  type ClassQuizzes,
  type ReviewOutcome,
} from "../model/quiz-schemas";
import type { ReviewBody } from "../model/review-form";
import { toQueueApiParams, type QueueFilter } from "../model/queue-filter";
import {
  classHomeworkSchema,
  deadlineSchema,
  gradeOutcomeSchema,
  queueSchema,
  recentListSchema,
  submissionDetailSchema,
  type ClassHomework,
  type Deadline,
  type GradeOutcome,
  type Queue,
  type Recent,
  type SubmissionDetail,
} from "../model/schemas";

const submissionPath = (submissionId: string) => `/submissions/${encodeURIComponent(submissionId)}`;
const attemptPath = (attemptId: string) => `/quiz-attempts/${encodeURIComponent(attemptId)}`;
const classPath = (classId: string) => `/classes/${encodeURIComponent(classId)}`;
const deadlinePath = (classId: string, lessonId: string) =>
  `${classPath(classId)}/deadlines/${encodeURIComponent(lessonId)}`;

/** How many recent gradings the queue page lists (the API's default). */
export const RECENT_LIMIT = 6;

export const gradingApi = {
  /** Only `toQueueApiParams` decides what reaches the API; page search params are never forwarded. */
  queue: (filter: QueueFilter, signal?: AbortSignal): Promise<Queue> => {
    const query = toQueueApiParams(filter).toString();
    return http(`/grading/queue${query ? `?${query}` : ""}`, { schema: queueSchema, signal });
  },
  recent: async (signal?: AbortSignal): Promise<Recent[]> =>
    (await http(`/grading/recent?limit=${String(RECENT_LIMIT)}`, { schema: recentListSchema, signal })).items,
  /** 403 outside the grader's classes, 404 when there is no such submission. */
  submission: (submissionId: string, signal?: AbortSignal): Promise<SubmissionDetail> =>
    http(submissionPath(submissionId), { schema: submissionDetailSchema, signal }),
  /** 201; 409 when a later submission superseded this one. */
  grade: (submissionId: string, body: GradeBody): Promise<GradeOutcome> =>
    http(`${submissionPath(submissionId)}/grades`, { method: "POST", body, schema: gradeOutcomeSchema }),
  classHomework: (classId: string, signal?: AbortSignal): Promise<ClassHomework> =>
    http(`${classPath(classId)}/homework`, { schema: classHomeworkSchema, signal }),
  /** `dueAt` is RFC 3339 with an offset; 409 on an ended class. */
  setDeadline: (classId: string, lessonId: string, dueAt: string): Promise<Deadline> =>
    http(deadlinePath(classId, lessonId), { method: "PUT", body: { dueAt }, schema: deadlineSchema }),
  clearDeadline: (classId: string, lessonId: string): Promise<void> =>
    http(deadlinePath(classId, lessonId), { method: "DELETE" }),
  /** A submitted quiz attempt; 403 outside the grader's classes, 404 when there is no such submitted attempt. */
  attempt: (attemptId: string, signal?: AbortSignal): Promise<AttemptDetail> =>
    http(attemptPath(attemptId), { schema: attemptDetailSchema, signal }),
  /** 201; 409 when a newer attempt superseded this one, 400 with the API's message on an invalid form. */
  review: (attemptId: string, body: ReviewBody): Promise<ReviewOutcome> =>
    http(`${attemptPath(attemptId)}/reviews`, { method: "POST", body, schema: reviewOutcomeSchema }),
  classQuizzes: (classId: string, signal?: AbortSignal): Promise<ClassQuizzes> =>
    http(`${classPath(classId)}/quizzes`, { schema: classQuizzesSchema, signal }),
  /** 201; 409 when the class is not running or the member left, 422 on a practice quiz or an invalid form. */
  grant: (classId: string, memberId: string, body: GrantBody): Promise<AttemptGrant> =>
    http(`${classPath(classId)}/members/${encodeURIComponent(memberId)}/grants`, {
      method: "POST",
      body,
      schema: attemptGrantSchema,
    }),
};

export const gradingKeys = {
  /** Prefix of every queue, whatever the filter. */
  queues: ["grading-queue"] as const,
  queue: (filter: QueueFilter) => ["grading-queue", filter] as const,
  recent: ["grading-recent"] as const,
  submission: (submissionId: string) => ["submission", submissionId] as const,
  /** Prefix of the homework tab of every class. */
  classHomeworks: ["class-homework"] as const,
  classHomework: (classId: string) => ["class-homework", classId] as const,
  /** A quiz attempt as its teacher sees it (the student's own view of it has another key). */
  attempt: (attemptId: string) => ["staff-quiz-attempt", attemptId] as const,
  /** Prefix of the quiz tab of every class. */
  classQuizzesAll: ["class-quizzes"] as const,
  classQuizzes: (classId: string) => ["class-quizzes", classId] as const,
};
