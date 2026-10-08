import { skipToken, useQuery } from "@tanstack/react-query";
import { relatedKeys, stagesApi } from "../api/stages-api";

/** Stage count and classes of the course version an apply would clone; idle until a course is chosen. */
export function useCourseVersionSummary(courseVersionId: string | undefined) {
  return useQuery({
    queryKey: relatedKeys.courseVersionSummary(courseVersionId ?? ""),
    queryFn: courseVersionId ? ({ signal }) => stagesApi.courseVersionSummary(courseVersionId, signal) : skipToken,
  });
}
