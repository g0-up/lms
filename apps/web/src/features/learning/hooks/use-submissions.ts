import { useQuery } from "@tanstack/react-query";
import { homeworkApi } from "../api/homework-api";
import { learningKeys } from "../api/learning-api";

/** The student's submissions of one homework, newest first, and whether they may submit again. */
export function useSubmissions(classId: string, lessonId: string) {
  return useQuery({
    queryKey: learningKeys.submissions(classId, lessonId),
    queryFn: ({ signal }) => homeworkApi.mySubmissions(classId, lessonId, signal),
  });
}
