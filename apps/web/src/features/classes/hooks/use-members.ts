import { useQuery } from "@tanstack/react-query";
import { useRef } from "react";
import { classesApi, classesKeys } from "../api/classes-api";
import { nextPoll, type PollWindow } from "../model/member-view";

/**
 * Members of the class, those who left included. While an invitation is queued the list polls so
 * its dot turns sent/failed without a reload; polling stops with the tab (no observer left).
 */
export function useMembers(classId: string) {
  const poll = useRef<PollWindow | null>(null);
  return useQuery({
    queryKey: classesKeys.members(classId),
    queryFn: ({ signal }) => classesApi.members(classId, signal),
    refetchInterval: (query) => {
      const next = nextPoll(poll.current, query.state.data, Date.now());
      poll.current = next.window;
      return next.interval;
    },
  });
}
