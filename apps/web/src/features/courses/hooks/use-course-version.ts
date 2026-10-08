import { skipToken, useQuery } from "@tanstack/react-query";
import { courseKeys, coursesApi } from "../api/courses-api";

/** Stages and classes of one course version; idle until the version is known. */
export function useCourseVersion(vid: string | undefined) {
  return useQuery({
    queryKey: courseKeys.version(vid ?? ""),
    queryFn: vid ? ({ signal }) => coursesApi.version(vid, signal) : skipToken,
  });
}
