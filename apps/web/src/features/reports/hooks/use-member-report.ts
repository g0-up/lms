import { useQuery } from "@tanstack/react-query";
import { reportsApi, reportsKeys } from "../api/reports-api";

/** Lesson-by-lesson progress of one member (`class_members.id`); idle while no member is open. */
export function useMemberReport(classId: string, memberId: string | null) {
  return useQuery({
    queryKey: reportsKeys.memberReport(classId, memberId ?? ""),
    queryFn: ({ signal }) => reportsApi.memberReport(classId, memberId ?? "", signal),
    enabled: memberId !== null,
  });
}
