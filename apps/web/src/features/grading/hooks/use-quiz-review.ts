import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { summaryKeys } from "@/features/summary";
import { isApiError } from "@/shared/api/errors";
import { gradingApi, gradingKeys } from "../api/grading-api";
import type { GrantBody } from "../model/grant-form";
import type { ReviewBody } from "../model/review-form";

/** A submitted quiz attempt with the key, its reviews and the student's other attempts; FORBIDDEN outside the grader's classes. */
export function useQuizAttemptDetail(attemptId: string) {
  return useQuery({
    queryKey: gradingKeys.attempt(attemptId),
    queryFn: ({ signal }) => gradingApi.attempt(attemptId, signal),
  });
}

/**
 * Confirms or comments an attempt. The queue, recent list, class quiz tabs and class summaries all change with it; a 409 (a newer
 * attempt came in) reloads the attempt so the form locks. Errors are left to the form.
 */
export function useReviewAttempt(attemptId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: ReviewBody) => gradingApi.review(attemptId, body),
    meta: { silent: true },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: gradingKeys.attempt(attemptId) }),
        queryClient.invalidateQueries({ queryKey: gradingKeys.queues }),
        queryClient.invalidateQueries({ queryKey: gradingKeys.recent }),
        queryClient.invalidateQueries({ queryKey: gradingKeys.classQuizzesAll }),
        queryClient.invalidateQueries({ queryKey: summaryKeys.matrices }),
        queryClient.invalidateQueries({ queryKey: summaryKeys.memberStages }),
      ]);
    },
    onError: (error) => {
      if (isApiError(error) && error.status === 409) {
        void queryClient.invalidateQueries({ queryKey: gradingKeys.attempt(attemptId) });
      }
    },
  });
}

/** Every quiz of a class with its settings, counts and members. */
export function useClassQuizzes(classId: string) {
  return useQuery({
    queryKey: gradingKeys.classQuizzes(classId),
    queryFn: ({ signal }) => gradingApi.classQuizzes(classId, signal),
  });
}

/** Grants a member extra attempts at an assessment; the quiz tab reloads with the new limit and grant. Errors are left to the dialog. */
export function useGrantAttempts(classId: string, memberId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: GrantBody) => gradingApi.grant(classId, memberId, body),
    meta: { silent: true },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: gradingKeys.classQuizzes(classId) }),
  });
}
