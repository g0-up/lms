import { queryOptions, useQuery } from "@tanstack/react-query";
import { stageKeys, stagesApi } from "../api/stages-api";

/** Every stage with its versions; the courses feature reads it to attach published stage versions. */
export const stagesQuery = queryOptions({
  queryKey: stageKeys.list,
  queryFn: ({ signal }) => stagesApi.list(signal),
});

export function useStages() {
  return useQuery(stagesQuery);
}
