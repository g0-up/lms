import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router";
import { toast } from "sonner";
import { summaryKeys } from "@/features/summary";
import { isApiError } from "@/shared/api/errors";
import { learningKeys } from "../api/learning-api";
import { quizApi } from "../api/quiz-api";
import type { Attempt, QuizOverview, QuizResult } from "../model/quiz-schemas";
import { clearPosition } from "../model/quiz-session";

export const RESUMED_MESSAGE = "Bạn đang có một lượt chưa nộp: tiếp tục lượt đó.";
export const SUBMITTED_MESSAGE = "Đã nộp bài.";
export const TIME_UP_MESSAGE = "Hết giờ: hệ thống đã tự nộp bài với các câu đã lưu.";
/** `?result=<attemptId>` names the submitted attempt on screen. */
export const RESULT_PARAM = "result";

const isConflict = (error: unknown) => isApiError(error) && error.status === 409;
/** The app's session handling already reacts to these; a toast would only repeat it. */
const isSessionError = (error: unknown) => isApiError(error) && error.status === 401;

/** The quiz's start screen: settings, attempts used, best result and whether a new attempt may start. */
export function useQuizOverview(classId: string, lessonId: string, enabled = true) {
  return useQuery({
    queryKey: learningKeys.quizOverview(classId, lessonId),
    queryFn: ({ signal }) => quizApi.overview(classId, lessonId, signal),
    enabled,
  });
}

/** Focus mode: the quiz has an open attempt and no result is on screen; the lesson page then hides its side list. */
export function useQuizFocus(classId: string, lessonId: string, isQuiz: boolean): boolean {
  const [params] = useSearchParams();
  const overview = useQuizOverview(classId, lessonId, isQuiz);
  return isQuiz && Boolean(overview.data?.openAttemptId) && !params.get(RESULT_PARAM);
}

/** One attempt: open (questions and the saved answers) or submitted (its result). */
export function useQuizAttempt(attemptId: string) {
  return useQuery({
    queryKey: learningKeys.quizAttempt(attemptId),
    queryFn: ({ signal }) => quizApi.attempt(attemptId, signal),
  });
}

/** Reads the attempt again; a 409 means its state moved on the server (submitted, time up, class ended). */
function refetchAttempt(queryClient: QueryClient, attemptId: string): Promise<Attempt> {
  return queryClient.query({
    queryKey: learningKeys.quizAttempt(attemptId),
    queryFn: ({ signal }) => quizApi.attempt(attemptId, signal),
    staleTime: 0,
  });
}

/**
 * After a submission: the start screen, the roadmap and the lesson's progress all changed. The overview stops
 * pointing at the attempt right away, so "Về trang quiz" cannot reopen it while the refetch is in flight.
 */
function invalidateAfterSubmit(queryClient: QueryClient, classId: string, lessonId: string, attemptId: string) {
  const overviewKey = learningKeys.quizOverview(classId, lessonId);
  queryClient.setQueryData<QuizOverview>(overviewKey, (o) =>
    o?.openAttemptId === attemptId ? { ...o, openAttemptId: undefined } : o,
  );
  void queryClient.invalidateQueries({ queryKey: overviewKey });
  void queryClient.invalidateQueries({ queryKey: learningKeys.myClass(classId) });
  void queryClient.invalidateQueries({ queryKey: learningKeys.lesson(classId, lessonId) });
  void queryClient.invalidateQueries({ queryKey: learningKeys.myClasses });
  void queryClient.invalidateQueries({ queryKey: summaryKeys.learner(classId) });
}

/**
 * Starts an attempt or resumes the open one. The attempt goes straight into its cache and the overview
 * points at it, so the page switches to focus mode without another round trip.
 */
export function useStartQuiz(classId: string, lessonId: string) {
  const queryClient = useQueryClient();
  const overviewKey = learningKeys.quizOverview(classId, lessonId);
  return useMutation({
    mutationFn: () => quizApi.start(classId, lessonId),
    meta: { silent: true },
    onSuccess: ({ attempt, resumed }) => {
      queryClient.setQueryData(learningKeys.quizAttempt(attempt.id), attempt);
      queryClient.setQueryData<QuizOverview>(overviewKey, (o) => (o ? { ...o, openAttemptId: attempt.id } : o));
      void queryClient.invalidateQueries({ queryKey: overviewKey });
      if (resumed) toast.info(RESUMED_MESSAGE);
    },
    onError: (error) => {
      if (isSessionError(error)) return;
      toast.error(error.message);
      // Attempts used up or the class changed state: show the start screen the server sees.
      if (isConflict(error)) void queryClient.invalidateQueries({ queryKey: overviewKey });
    },
  });
}

/** A save that failed: the picked options stay on screen so the student can retry. */
export interface SaveFailure {
  optionIds: string[];
  message: string;
}

/**
 * Saves answers as the student picks them. Saves run one at a time per question and only the latest pick
 * is sent: a quick change of mind while a save is in flight sends one more request, not one per click.
 * A 409 goes to `onConflict` (the attempt may have been closed on the server).
 */
export function useAnswerSaver(attemptId: string, initialSavedAt: string | null, onConflict: (error: Error) => void) {
  const [savedAt, setSavedAt] = useState(initialSavedAt);
  const [failures, setFailures] = useState<Readonly<Record<string, SaveFailure | undefined>>>({});
  const [saving, setSaving] = useState(0);
  const latest = useRef(new Map<string, string[]>());
  const running = useRef(new Map<string, Promise<void>>());
  const conflict = useRef(onConflict);
  useEffect(() => {
    conflict.current = onConflict;
  }, [onConflict]);

  const pump = useCallback(
    (questionId: string) => {
      // A save in flight picks the queued value up when it finishes.
      if (running.current.has(questionId) || !latest.current.has(questionId)) return;
      const run = async () => {
        setSaving((n) => n + 1);
        try {
          for (let optionIds = latest.current.get(questionId); optionIds; optionIds = latest.current.get(questionId)) {
            latest.current.delete(questionId);
            try {
              const saved = await quizApi.saveAnswer(attemptId, questionId, optionIds);
              setSavedAt(saved.savedAt);
              setFailures((f) => ({ ...f, [questionId]: undefined }));
            } catch (error) {
              // A newer pick is queued: it replaces this one, so only its outcome matters.
              if (latest.current.has(questionId)) continue;
              if (isSessionError(error)) return;
              const message = error instanceof Error ? error.message : String(error);
              setFailures((f) => ({ ...f, [questionId]: { optionIds, message } }));
              if (isConflict(error) && error instanceof Error) conflict.current(error);
            }
          }
        } finally {
          running.current.delete(questionId);
          setSaving((n) => n - 1);
        }
      };
      running.current.set(questionId, run());
    },
    [attemptId],
  );

  const save = useCallback(
    (questionId: string, optionIds: string[]) => {
      latest.current.set(questionId, optionIds);
      pump(questionId);
    },
    [pump],
  );

  /** Resolves once every queued save has finished, successfully or not. */
  const idle = useCallback(async () => {
    while (running.current.size > 0) await Promise.all(running.current.values());
  }, []);

  return { save, idle, savedAt, failures, saving: saving > 0 };
}

export interface SubmitInput {
  /** The clock ran out: the client submits on the student's behalf. */
  auto: boolean;
}

/**
 * Submits the attempt once pending saves are done. On success the result replaces the attempt in the cache
 * and `onResult` shows it. A 409 means the server got there first (closed at the deadline, or submitted in
 * another tab): when the attempt reads back as submitted, its result shows the same way.
 */
export function useSubmitAttempt({
  classId,
  lessonId,
  attemptId,
  idle,
  onResult,
}: {
  classId: string;
  lessonId: string;
  attemptId: string;
  idle: () => Promise<void>;
  onResult: (result: QuizResult) => void;
}) {
  const queryClient = useQueryClient();

  const finish = (result: QuizResult, message: string, auto: boolean) => {
    queryClient.setQueryData(learningKeys.quizAttempt(attemptId), result);
    clearPosition(attemptId);
    invalidateAfterSubmit(queryClient, classId, lessonId, attemptId);
    if (auto) toast.warning(message);
    else toast.success(message);
    onResult(result);
  };

  return useMutation({
    mutationFn: async (_input: SubmitInput) => {
      await idle();
      return quizApi.submit(attemptId);
    },
    meta: { silent: true },
    onSuccess: (result, { auto }) => {
      finish(result, auto ? TIME_UP_MESSAGE : SUBMITTED_MESSAGE, auto);
    },
    onError: async (error, { auto }) => {
      if (isSessionError(error)) return;
      if (isConflict(error)) {
        try {
          const fresh = await refetchAttempt(queryClient, attemptId);
          if (fresh.status === "submitted") {
            finish(fresh, auto ? TIME_UP_MESSAGE : error.message, true);
            return;
          }
        } catch {
          // Fall through to the submit error: it says why the attempt could not be submitted.
        }
      }
      toast.error(error.message);
    },
  });
}

/**
 * Settles an attempt that reads back as submitted when it was expected open (closed by the server at the
 * deadline, or submitted in another tab): forget the stored position and refresh what the submission changed.
 */
export function useSettleSubmitted(classId: string, lessonId: string) {
  const queryClient = useQueryClient();
  return useCallback(
    (attemptId: string) => {
      clearPosition(attemptId);
      invalidateAfterSubmit(queryClient, classId, lessonId, attemptId);
    },
    [queryClient, classId, lessonId],
  );
}

/** Opens the result after a save refused with 409, when the attempt turns out to be closed already. */
export function useClosedAttemptCheck({
  classId,
  lessonId,
  attemptId,
  onResult,
}: {
  classId: string;
  lessonId: string;
  attemptId: string;
  onResult: (result: QuizResult) => void;
}) {
  const queryClient = useQueryClient();
  const checking = useRef(false);
  return useCallback(
    (error: Error) => {
      if (checking.current) return;
      checking.current = true;
      refetchAttempt(queryClient, attemptId)
        .then((fresh) => {
          if (fresh.status !== "submitted") return;
          clearPosition(attemptId);
          invalidateAfterSubmit(queryClient, classId, lessonId, attemptId);
          toast.warning(error.message);
          onResult(fresh);
        })
        .catch(() => {
          // The save's own error stays on screen with its retry.
        })
        .finally(() => {
          checking.current = false;
        });
    },
    [queryClient, classId, lessonId, attemptId, onResult],
  );
}
