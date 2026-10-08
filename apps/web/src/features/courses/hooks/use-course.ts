import { useQuery } from "@tanstack/react-query";
import { courseKeys, coursesApi } from "../api/courses-api";

/** One course with its versions and the classes using them. */
export function useCourse(courseId: string) {
  return useQuery({ queryKey: courseKeys.detail(courseId), queryFn: ({ signal }) => coursesApi.detail(courseId, signal) });
}
