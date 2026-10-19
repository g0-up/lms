/**
 * MSW handlers for the grading, quiz review and class homework/quiz API, answering with the golden fixtures of `test/golden.ts`.
 * Test-only: nothing in the app imports this module.
 */
import { http, HttpResponse } from "msw";
import { api, apiError } from "@/shared/test/msw";
import { isPassing } from "@/shared/domain";
import type { GradeBody } from "../model/grade-form";
import type { GrantBody } from "../model/grant-form";
import type { AttemptDetail, AttemptGrant, ReviewOutcome } from "../model/quiz-schemas";
import type { ReviewBody } from "../model/review-form";
import type { GradeOutcome, Queue, SubmissionDetail } from "../model/schemas";
import { fixtures } from "../test/golden";

/** The queue filtered as the API does; the counts always cover the whole queue. */
export function filterQueue(queue: Queue, params: URLSearchParams): Queue {
  const kind = params.get("kind");
  const classId = params.get("classId");
  const lessonId = params.get("lessonId");
  const overdue = params.get("overdue") === "1";
  const items = queue.items.filter(
    (item) =>
      (!kind || item.kind === kind) &&
      (!classId || item.class.id === classId) &&
      (!lessonId || item.lesson.id === lessonId) &&
      (!overdue || item.overdue),
  );
  return { ...queue, items };
}

/** The API's grading of `detail` with `body`: totals, verdict and the next queue item after a first grading. */
export function gradeOutcome(detail: SubmissionDetail, body: GradeBody, next: Queue["items"][number] | null) {
  const { lesson } = detail;
  const scores = lesson.criteria.map((c) => {
    const score = body.scores.find((s) => s.criterionId === c.id);
    return {
      criterionId: c.id,
      description: c.description,
      points: score?.points ?? 0,
      maxPoints: c.maxPoints,
      comment: score?.comment ?? "",
    };
  });
  const totalPoints = scores.reduce((sum, s) => sum + s.points, 0);
  const result = isPassing(totalPoints, lesson.maxPoints, lesson.passPercent) ? "passed" : "rework";
  const updated = detail.submission.grade !== null;
  return {
    grade: {
      id: "0199ffff-0000-7000-8000-0000000000f1",
      totalPoints,
      maxPoints: lesson.maxPoints,
      result,
      comment: body.comment,
      gradedBy: "Lê Thu Hương",
      gradedAt: new Date().toISOString(),
      scores,
    },
    submissionStatus: result,
    updated,
    nextQueueItem: updated ? null : next,
  } satisfies GradeOutcome;
}

/** Happy-path grading API as the golden teacher sees it; any other submission id is a 404. */
export const gradingHandlers = [
  http.get(api("/grading/queue"), ({ request }) =>
    HttpResponse.json(filterQueue(fixtures.queueTeacher, new URL(request.url).searchParams)),
  ),
  http.get(api("/grading/recent"), () => HttpResponse.json({ items: fixtures.recent })),
  http.get(api("/submissions/:sid"), ({ params }) => {
    if (params.sid === fixtures.submissionPending.submission.id) return HttpResponse.json(fixtures.submissionPending);
    return apiError(404, "NOT_FOUND", "Không tìm thấy bài nộp.");
  }),
  http.post(api("/submissions/:sid/grades"), async ({ request }) => {
    const body = (await request.json()) as GradeBody;
    const next = fixtures.queueTeacher.items.at(0) ?? null;
    return HttpResponse.json(gradeOutcome(fixtures.submissionPending, body, next), { status: 201 });
  }),
];

/** Class homework tab: the golden class, deadlines saved or cleared. */
export const homeworkTabHandlers = [
  http.get(api("/classes/:classId/homework"), () => HttpResponse.json(fixtures.classHomework)),
  http.put(api("/classes/:classId/deadlines/:lessonId"), async ({ request, params }) => {
    const body = (await request.json()) as { dueAt: string };
    return HttpResponse.json({ lessonId: params.lessonId, dueAt: new Date(body.dueAt).toISOString() });
  }),
  http.delete(api("/classes/:classId/deadlines/:lessonId"), () => new HttpResponse(null, { status: 204 })),
];

/** The API's review of `detail` with `body`; only a first confirmation of an assessment moves on to `next`. */
export function reviewOutcome(detail: AttemptDetail, body: ReviewBody, next: Queue["items"][number] | null) {
  const updated = detail.reviews.length > 0;
  return {
    review: {
      id: "0199ffff-0000-7000-8000-0000000000f2",
      reviewer: { id: "01990000-0000-7000-8000-000000000002", name: "Lê Thu Hương" },
      createdAt: new Date().toISOString(),
      comment: body.comment,
      notes: body.notes,
    },
    reviewStatus: "confirmed",
    updated,
    nextQueueItem: detail.mode === "assessment" && !updated ? next : null,
  } satisfies ReviewOutcome;
}

/**
 * Quiz review as the golden teacher sees it: An's pending #2 and superseded #1; any other attempt is a 404. A grant
 * to a member of the golden class quizzes is saved as sent.
 */
export const quizReviewHandlers = [
  http.get(api("/quiz-attempts/:aid"), ({ params }) => {
    if (params.aid === fixtures.reviewPending.id) return HttpResponse.json(fixtures.reviewPending);
    if (params.aid === fixtures.reviewSuperseded.id) return HttpResponse.json(fixtures.reviewSuperseded);
    return apiError(404, "NOT_FOUND", "Không tìm thấy lượt làm bài đã nộp.");
  }),
  http.post(api("/quiz-attempts/:aid/reviews"), async ({ request }) => {
    const body = (await request.json()) as ReviewBody;
    const next = fixtures.queueTeacher.items.at(0) ?? null;
    return HttpResponse.json(reviewOutcome(fixtures.reviewPending, body, next), { status: 201 });
  }),
  http.get(api("/classes/:classId/quizzes"), () => HttpResponse.json(fixtures.classQuizzes)),
  http.post(api("/classes/:classId/members/:memberId/grants"), async ({ request, params }) => {
    const body = (await request.json()) as GrantBody;
    const member = fixtures.classQuizzes.items
      .find((item) => item.lesson.id === body.lessonId)
      ?.members.find((m) => m.memberId === params.memberId);
    if (!member) return apiError(404, "NOT_FOUND", "Không tìm thấy học viên trong lớp.");
    return HttpResponse.json(
      {
        id: "0199ffff-0000-7000-8000-0000000000f3",
        memberId: member.memberId,
        lessonId: body.lessonId,
        studentName: member.name,
        extraAttempts: body.extraAttempts,
        reason: body.reason,
        grantedByName: "Lê Thu Hương",
        grantedAt: new Date().toISOString(),
      } satisfies AttemptGrant,
      { status: 201 },
    );
  }),
];
