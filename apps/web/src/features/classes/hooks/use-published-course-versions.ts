import { queryOptions, useQuery } from "@tanstack/react-query";
import { classesApi, classesKeys } from "../api/classes-api";
import { publishedVersions } from "../model/class-form";

/** The course list behind the version selects; its versions carry their status. */
export const courseVersionsQuery = queryOptions({
  queryKey: classesKeys.publishedVersions,
  queryFn: ({ signal }) => classesApi.courses(signal),
});

/** Published course versions, the only ones a class may run on. */
export function usePublishedCourseVersions() {
  return useQuery({ ...courseVersionsQuery, select: publishedVersions });
}
