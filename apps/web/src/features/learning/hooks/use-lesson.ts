import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { learningApi, learningKeys } from "../api/learning-api";
import type { Roadmap } from "../model/schemas";
import { myClassQuery, updateRoadmapLesson } from "./use-my-class";

/**
 * One lesson page. Fetching it records the first open on the server (FR-31); the recorded time is
 * copied into the cached roadmap so the tick unlocks there without a reload, whichever of the two
 * queries answers first.
 */
export function useLesson(classId: string, lessonId: string) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryKey: learningKeys.lesson(classId, lessonId),
    queryFn: ({ signal }) => learningApi.lesson(classId, lessonId, signal),
    // A refetch would re-sign the video URL for nothing; the player renews it itself.
    refetchOnWindowFocus: false,
  });
  const unopenedInRoadmap = useQuery({
    ...myClassQuery(classId),
    select: (r) => r.stages.some((s) => s.lessons.some((l) => l.id === lessonId && !l.firstOpenedAt)),
  }).data;

  const openedAt = query.data?.progress.firstOpenedAt;
  useEffect(() => {
    if (!openedAt || !unopenedInRoadmap) return;
    queryClient.setQueryData<Roadmap>(learningKeys.myClass(classId), (r) =>
      r ? updateRoadmapLesson(r, lessonId, (l) => (l.firstOpenedAt ? l : { ...l, firstOpenedAt: openedAt })) : r,
    );
  }, [queryClient, classId, lessonId, openedAt, unopenedInRoadmap]);

  return query;
}
