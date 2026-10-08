import { queryOptions, useQuery } from "@tanstack/react-query";
import { classesApi, classesKeys } from "../api/classes-api";

export const teachersQuery = queryOptions({
  queryKey: classesKeys.teachers,
  queryFn: ({ signal }) => classesApi.teachers(signal),
});

/** Active teachers, the only ones a class can be assigned to. */
export function useTeachers() {
  return useQuery(teachersQuery);
}
