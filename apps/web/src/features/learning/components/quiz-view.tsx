import { useCallback } from "react";
import { useSearchParams } from "react-router";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { CardBody } from "@/shared/ui/card";
import { Skeleton } from "@/shared/ui/skeleton";
import { RESULT_PARAM, useQuizOverview, useStartQuiz } from "../hooks/use-quiz-attempt";
import type { LessonPage } from "../model/schemas";
import { LessonNav } from "./lesson-nav";
import { QuizResultView } from "./quiz-result";
import { QuizRunner } from "./quiz-runner";
import { QuizStart } from "./quiz-start";

/**
 * A quiz lesson: the start screen, the open attempt in focus mode, or a result (`?result=`). The overview decides
 * between the first two; the page reads the same overview to hide its side list in focus mode.
 */
export function QuizView({ classId, page }: { classId: string; page: LessonPage }) {
  const lessonId = page.lesson.id;
  const [params, setParams] = useSearchParams();
  const resultId = params.get(RESULT_PARAM);
  const overview = useQuizOverview(classId, lessonId);
  const start = useStartQuiz(classId, lessonId);

  const showResult = useCallback(
    (attemptId: string, replace: boolean) => {
      setParams({ [RESULT_PARAM]: attemptId }, { replace });
    },
    [setParams],
  );
  const leaveResult = useCallback(
    (replace: boolean) => {
      setParams({}, { replace });
    },
    [setParams],
  );
  const onStart = () => {
    start.mutate(undefined, {
      onSuccess: () => {
        if (resultId) leaveResult(false);
      },
    });
  };

  if (resultId) {
    return (
      <QuizResultView
        key={resultId}
        attemptId={resultId}
        overview={overview.data}
        starting={start.isPending}
        onStart={onStart}
        onLeave={leaveResult}
      />
    );
  }
  if (overview.isPending) {
    return (
      <CardBody aria-busy="true" className="grid gap-3">
        <span className="sr-only">Đang tải quiz</span>
        <Skeleton className="h-5 w-2/3" />
        <Skeleton className="h-5 w-1/2" />
        <Skeleton className="h-5 w-3/5" />
      </CardBody>
    );
  }
  if (overview.isError) {
    return (
      <CardBody>
        <Alert variant="danger" role="alert">
          {overview.error.message}
          <AlertActions>
            <Button variant="outline" size="sm" onClick={() => void overview.refetch()}>
              Thử lại
            </Button>
          </AlertActions>
        </Alert>
      </CardBody>
    );
  }
  const { openAttemptId } = overview.data;
  if (openAttemptId) {
    return (
      <QuizRunner
        key={openAttemptId}
        classId={classId}
        lessonId={lessonId}
        attemptId={openAttemptId}
        onResult={showResult}
      />
    );
  }
  return (
    <QuizStart
      overview={overview.data}
      nav={
        <LessonNav
          classId={classId}
          prev={page.prev}
          next={page.next}
          crossesUnit={page.crossesUnit}
          nextUnitTitle={page.nextUnitTitle}
        />
      }
      starting={start.isPending}
      onStart={onStart}
    />
  );
}

