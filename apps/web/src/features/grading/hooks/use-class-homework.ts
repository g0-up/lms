import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { gradingApi, gradingKeys } from "../api/grading-api";

/** Every homework of a class with its deadline, counts and members. */
export function useClassHomework(classId: string) {
  return useQuery({
    queryKey: gradingKeys.classHomework(classId),
    queryFn: ({ signal }) => gradingApi.classHomework(classId, signal),
  });
}

/** Sets (`dueAt` RFC 3339) or clears (null) the deadline of one homework; the dialog shows errors. */
export function useDeadline(classId: string, lessonId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (dueAt: string | null) => {
      if (dueAt === null) await gradingApi.clearDeadline(classId, lessonId);
      else await gradingApi.setDeadline(classId, lessonId, dueAt);
    },
    meta: { silent: true },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: gradingKeys.classHomework(classId) }),
  });
}
