import { skipToken, useQuery } from "@tanstack/react-query";
import { stageKeys, stagesApi } from "../api/stages-api";

/** Lessons and references of one stage version; idle until the version is known. */
export function useStageVersion(vid: string | undefined) {
  return useQuery({
    queryKey: stageKeys.version(vid ?? ""),
    queryFn: vid ? ({ signal }) => stagesApi.version(vid, signal) : skipToken,
  });
}
