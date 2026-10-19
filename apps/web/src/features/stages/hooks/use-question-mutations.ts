import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { hasCode } from "@/shared/api/errors";
import { stagesApi } from "../api/stages-api";
import type { QuestionBody } from "../model/question-form";
import { orderByIds } from "../model/order";
import type { Lesson } from "../model/schemas";
import { useContentRefresh, useDeleteFailed, useLessonItemsReorder, useLessonResult } from "./use-lesson-mutations";

export interface SaveQuestionInput {
  /** Absent when adding. */
  questionId?: string;
  body: QuestionBody;
}

/** `lesson` with its questions in the order of `ids`. */
function arrangeQuestions(lesson: Lesson, ids: string[]): Lesson {
  if (!lesson.quiz) return lesson;
  return { ...lesson, quiz: { ...lesson.quiz, questions: orderByIds(lesson.quiz.questions, ids) } };
}

/** Add, edit, delete and reorder the questions of one quiz in a draft version. */
export function useQuestionMutations(stageId: string, vid: string, lessonId: string) {
  const refresh = useContentRefresh(stageId, vid);
  const store = useLessonResult(stageId, vid);
  const deleteFailed = useDeleteFailed(stageId, vid);

  // The question dialog shows save errors itself.
  const save = useMutation({
    mutationFn: ({ questionId, body }: SaveQuestionInput) =>
      questionId
        ? stagesApi.updateQuestion(vid, lessonId, questionId, body)
        : stagesApi.addQuestion(vid, lessonId, body),
    meta: { silent: true },
    onSuccess: async (lesson, { questionId }) => {
      await store(lesson);
      toast.success(questionId ? "Đã lưu câu hỏi." : "Đã thêm câu hỏi.");
    },
    onError: (error) => {
      if (hasCode(error, "VERSION_IMMUTABLE")) void refresh();
    },
  });

  const remove = useMutation({
    mutationFn: (questionId: string) => stagesApi.removeQuestion(vid, lessonId, questionId),
    meta: { silent: true },
    onSuccess: async () => {
      await refresh();
      toast.success("Đã xóa câu hỏi.");
    },
    onError: deleteFailed,
  });

  // Takes every question id of the quiz in the new order.
  const reorder = useLessonItemsReorder(
    stageId,
    vid,
    lessonId,
    (ids) => stagesApi.reorderQuestions(vid, lessonId, ids),
    arrangeQuestions,
  );

  return { save, remove, reorder };
}
