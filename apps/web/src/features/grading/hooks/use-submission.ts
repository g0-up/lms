import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { summaryKeys } from "@/features/summary";
import { isApiError } from "@/shared/api/errors";
import { gradingApi, gradingKeys } from "../api/grading-api";
import type { GradeBody } from "../model/grade-form";

/** A submission to grade with its brief, rubric and the student's history; FORBIDDEN outside the grader's classes. */
export function useSubmission(submissionId: string) {
  return useQuery({
    queryKey: gradingKeys.submission(submissionId),
    queryFn: ({ signal }) => gradingApi.submission(submissionId, signal),
  });
}

/**
 * Saves a grading. The queue, recent list, class homework tabs and class summaries all change with it; a 409 (a newer submission
 * came in) reloads the submission so the form locks. Errors are left to the form.
 */
export function useGrade(submissionId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: GradeBody) => gradingApi.grade(submissionId, body),
    meta: { silent: true },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: gradingKeys.submission(submissionId) }),
        queryClient.invalidateQueries({ queryKey: gradingKeys.queues }),
        queryClient.invalidateQueries({ queryKey: gradingKeys.recent }),
        queryClient.invalidateQueries({ queryKey: gradingKeys.classHomeworks }),
        queryClient.invalidateQueries({ queryKey: summaryKeys.matrices }),
        queryClient.invalidateQueries({ queryKey: summaryKeys.memberStages }),
      ]);
    },
    onError: (error) => {
      if (isApiError(error) && error.status === 409) {
        void queryClient.invalidateQueries({ queryKey: gradingKeys.submission(submissionId) });
      }
    },
  });
}
