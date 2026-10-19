import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { gradingApi, gradingKeys } from "../api/grading-api";
import type { QueueFilter } from "../model/queue-filter";

/** The grader's queue, oldest first; the previous rows stay on screen while a new filter loads. */
export function useQueue(filter: QueueFilter) {
  return useQuery({
    queryKey: gradingKeys.queue(filter),
    queryFn: ({ signal }) => gradingApi.queue(filter, signal),
    placeholderData: keepPreviousData,
  });
}

/** The grader's latest gradings and quiz confirmations, newest first. */
export function useRecent() {
  return useQuery({ queryKey: gradingKeys.recent, queryFn: ({ signal }) => gradingApi.recent(signal) });
}
