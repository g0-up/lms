import { queryOptions, useQuery } from "@tanstack/react-query";
import { reportsApi, reportsKeys } from "../api/reports-api";

export const teachClassesQuery = queryOptions({
  queryKey: reportsKeys.teachClasses,
  queryFn: ({ signal }) => reportsApi.teachClasses(signal),
});

/** Classes the signed-in teacher is in charge of. */
export function useTeachClasses() {
  return useQuery(teachClassesQuery);
}

/** Header of a teacher's class; FORBIDDEN or NOT_FOUND when it is not theirs. */
export function useTeachClass(classId: string) {
  return useQuery({
    queryKey: reportsKeys.teachClass(classId),
    queryFn: ({ signal }) => reportsApi.teachClass(classId, signal),
  });
}
