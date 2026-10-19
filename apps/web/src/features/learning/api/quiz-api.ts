import { http } from "@/shared/api/http";
import {
  attemptSchema,
  quizOverviewSchema,
  quizResultSchema,
  savedSchema,
  startedSchema,
  type Attempt,
  type QuizOverview,
  type QuizResult,
  type Started,
} from "../model/quiz-schemas";
import { lessonPath } from "./learning-api";

const attemptsPath = (classId: string, lessonId: string) => `${lessonPath(classId, lessonId)}/attempts`;
const attemptPath = (attemptId: string) => `/me/attempts/${encodeURIComponent(attemptId)}`;

export const quizApi = {
  /** The start screen; also closes the student's own attempt whose time ran out. */
  overview: (classId: string, lessonId: string, signal?: AbortSignal): Promise<QuizOverview> =>
    http(attemptsPath(classId, lessonId), { schema: quizOverviewSchema, signal }),
  /** Starts an attempt, or hands back the open one (`resumed`). */
  start: (classId: string, lessonId: string): Promise<Started> =>
    http(attemptsPath(classId, lessonId), { method: "POST", schema: startedSchema }),
  /** The open attempt, or its result once submitted (also after the server closed it at the deadline). */
  attempt: (attemptId: string, signal?: AbortSignal): Promise<Attempt> =>
    http(attemptPath(attemptId), { schema: attemptSchema, signal }),
  /** Replaces the answer of one question; an empty list clears it. */
  saveAnswer: (attemptId: string, questionId: string, optionIds: readonly string[]): Promise<{ savedAt: string }> =>
    http(`${attemptPath(attemptId)}/answers/${encodeURIComponent(questionId)}`, {
      method: "PUT",
      body: { optionIds },
      schema: savedSchema,
    }),
  submit: (attemptId: string): Promise<QuizResult> =>
    http(`${attemptPath(attemptId)}/submit`, { method: "POST", schema: quizResultSchema }),
};
