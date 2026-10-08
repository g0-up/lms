import { useQuery } from "@tanstack/react-query";
import { courseKeys, coursesApi } from "../api/courses-api";

export function useCourses() {
  return useQuery({ queryKey: courseKeys.list, queryFn: ({ signal }) => coursesApi.list(signal) });
}
