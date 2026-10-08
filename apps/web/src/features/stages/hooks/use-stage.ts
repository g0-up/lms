import { queryOptions, useQuery } from "@tanstack/react-query";
import { stageKeys, stagesApi } from "../api/stages-api";

export function stageQuery(stageId: string) {
  return queryOptions({
    queryKey: stageKeys.detail(stageId),
    queryFn: ({ signal }) => stagesApi.detail(stageId, signal),
  });
}

/** One stage: its versions, the course versions using it and the courses still on an older version. */
export function useStage(stageId: string) {
  return useQuery(stageQuery(stageId));
}
