import { Alert, AlertActions } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Skeleton } from "@/shared/ui/skeleton";
import { useSubmissions } from "../hooks/use-submissions";
import { latestResult } from "../model/submissions";
import { SubmissionFeedback } from "./submission-feedback";
import { SubmissionForm } from "./submission-form";
import { SubmissionHistory } from "./submission-history";

/**
 * The student's side of a homework, under its brief: latest result, the submit form (or why it is
 * locked), then every submission. The server decides whether a new submission is allowed.
 */
export function HomeworkSubmissions({ classId, lessonId }: { classId: string; lessonId: string }) {
  const query = useSubmissions(classId, lessonId);
  if (query.isPending) {
    return (
      <div aria-busy="true" className="grid gap-6">
        <span className="sr-only">Đang tải bài nộp</span>
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (query.isError) {
    return (
      <Alert variant="danger" role="alert">
        {query.error.message}
        <AlertActions>
          <Button variant="outline" size="sm" onClick={() => void query.refetch()}>
            Thử lại
          </Button>
        </AlertActions>
      </Alert>
    );
  }

  const data = query.data;
  const latest = data.submissions.at(0);
  const result = latestResult(data);
  const passed = data.status === "passed";
  return (
    <>
      {result ? <SubmissionFeedback result={result} /> : null}
      {data.canSubmit ? (
        // A new submission remounts the form so it prefills from it and drops the picked files.
        <SubmissionForm
          key={latest?.id ?? "first"}
          classId={classId}
          lessonId={lessonId}
          deadline={data.deadline}
          maxFiles={data.maxFiles}
          latest={latest}
        />
      ) : (
        <Alert variant={passed ? "ok" : "info"}>
          {passed
            ? "Bài đã được chấm Đạt nên không cần nộp lại."
            : (data.lockedReason ?? "Không nộp bài được lúc này.")}
        </Alert>
      )}
      {data.submissions.length > 0 ? <SubmissionHistory submissions={data.submissions} /> : null}
    </>
  );
}
