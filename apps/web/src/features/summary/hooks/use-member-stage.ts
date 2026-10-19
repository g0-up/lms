import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { isApiError } from "@/shared/api/errors";
import { summaryApi, summaryKeys, type StageTarget } from "../api/summary-api";

const NO_TARGET: StageTarget = { memberId: "", stageVersionId: "" };

/** One member's stage with its review; idle while no matrix cell is open. */
export function useMemberStage(classId: string, target: StageTarget | null) {
  return useQuery({
    queryKey: summaryKeys.memberStage(classId, target ?? NO_TARGET),
    queryFn: ({ signal }) => summaryApi.memberStage(classId, target ?? NO_TARGET, signal),
    enabled: target !== null,
  });
}

/**
 * Writes or replaces the review of an open cell. The drawer and the matrix (its "no review" flag) change with it;
 * a 409 (someone saved in between) reloads the drawer. Errors are left to the form.
 */
export function useSaveReview(classId: string, target: StageTarget) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: string) => summaryApi.saveReview(classId, target, body),
    meta: { silent: true },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: summaryKeys.memberStage(classId, target) }),
        queryClient.invalidateQueries({ queryKey: summaryKeys.matrix(classId) }),
      ]);
    },
    onError: (error) => {
      if (isApiError(error) && error.status === 409) {
        void queryClient.invalidateQueries({ queryKey: summaryKeys.memberStage(classId, target) });
      }
    },
  });
}
