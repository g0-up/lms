import { useQuery } from "@tanstack/react-query";
import { summaryApi, summaryKeys } from "../api/summary-api";

/** The signed-in student's course summary of a class. */
export function useLearnerSummary(classId: string) {
  return useQuery({
    queryKey: summaryKeys.learner(classId),
    queryFn: ({ signal }) => summaryApi.learner(classId, signal),
  });
}
