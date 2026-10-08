import { queryOptions, useQuery } from "@tanstack/react-query";
import { classesApi, classesKeys } from "../api/classes-api";

export const classesQuery = queryOptions({
  queryKey: classesKeys.list,
  queryFn: ({ signal }) => classesApi.list(signal),
});

/** Every class, newest first as the API orders them. */
export function useClasses() {
  return useQuery(classesQuery);
}

/** Average progress per class id; the list shows "—" while it loads or when it fails. */
export function useClassProgress() {
  return useQuery({
    queryKey: classesKeys.progress,
    queryFn: ({ signal }) => classesApi.progress(signal),
    select: (data) => new Map(data.classes.map((c) => [c.id, c.avgPercent])),
  });
}
