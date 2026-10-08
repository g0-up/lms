import { queryOptions, useQuery } from "@tanstack/react-query";
import { learningApi, learningKeys } from "../api/learning-api";

export const myClassesQuery = queryOptions({
  queryKey: learningKeys.myClasses,
  queryFn: ({ signal }) => learningApi.myClasses(signal),
});

/** Classes the signed-in student actively belongs to, draft ones included. */
export function useMyClasses() {
  return useQuery(myClassesQuery);
}
