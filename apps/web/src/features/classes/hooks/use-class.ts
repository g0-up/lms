import { queryOptions, useQuery } from "@tanstack/react-query";
import { classesApi, classesKeys } from "../api/classes-api";

export const classQuery = (classId: string) =>
  queryOptions({
    queryKey: classesKeys.detail(classId),
    queryFn: ({ signal }) => classesApi.get(classId, signal),
  });

export function useClass(classId: string) {
  return useQuery(classQuery(classId));
}
