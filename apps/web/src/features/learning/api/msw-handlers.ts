/**
 * MSW handlers for the student learning API, answering with the golden fixtures of
 * `test/golden.ts`. Test-only: nothing in the app imports this module.
 */
import { http, HttpResponse } from "msw";
import { api } from "@/shared/test/msw";
import type { Attempt, OpenAttempt, QuizOverview, QuizResult } from "../model/quiz-schemas";
import type { LessonPage, MySubmissions, Roadmap, Submission } from "../model/schemas";
import type { SubmitBody } from "../model/submission-form";
import { errors, fixtures, freshExpiry, ids, openAssessment } from "../test/golden";

function withFreshVideoUrl(page: LessonPage): LessonPage {
  return page.content.type === "video" ? { ...page, content: { ...page.content, expiresAt: freshExpiry() } } : page;
}

export function errorResponse(status: number, envelope: { error: { code: string; message: string } }) {
  return HttpResponse.json(envelope, { status });
}

const roadmaps: Partial<Record<string, Roadmap>> = {
  [ids.active]: fixtures.roadmapActive,
  [ids.draft]: fixtures.roadmapDraft,
  [ids.ended]: fixtures.roadmapEnded,
};

const lessons: Partial<Record<string, Partial<Record<string, LessonPage>>>> = {
  [ids.active]: {
    [ids.video]: fixtures.lessonVideo,
    [ids.markdown]: fixtures.lessonMarkdown,
    [ids.homework]: fixtures.lessonHomework,
    [ids.quizPractice]: fixtures.lessonQuizPractice,
    [ids.quizAssessment]: fixtures.lessonQuizAssessment,
  },
  [ids.ended]: { [ids.optional]: fixtures.lessonEnded },
};

/** Presigned storage of the homework uploads; the PUT lands in MSW, never on the network. */
export const STORAGE_URL = "https://storage.mem/put/homework?X-Amz-Signature=mem";
const STORAGE_CORS = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Methods": "PUT",
  "Access-Control-Allow-Headers": "Content-Type",
};

let uploadSeq = 0;

/**
 * The API's answer to a submission on top of `before`: the new submission comes first and a pending
 * one it replaces turns superseded. Uploaded files are named after their media id.
 */
export function submitted(before: MySubmissions, body: SubmitBody, isLate = false): MySubmissions {
  const created: Submission = {
    id: `0199ffff-0000-7000-8000-${String(before.submissions.length + 100).padStart(12, "0")}`,
    no: before.submissions.length + 1,
    submittedAt: new Date().toISOString(),
    isLate,
    repoUrl: body.repoUrl || null,
    commitRef: body.commitRef || null,
    files: body.mediaIds.map((mediaId) => ({ mediaId, fileName: `${mediaId}.md`, sizeBytes: 5 })),
    status: "pending",
    grade: null,
  };
  return {
    ...before,
    status: "pending",
    submissions: [
      created,
      ...before.submissions.map((s): Submission => (s.status === "pending" ? { ...s, status: "superseded" } : s)),
    ],
  };
}

/** `/me/uploads`: reserve, storage PUT, confirm; each reservation gets a new media id. */
export const studentUploadHandlers = [
  http.post(api("/me/uploads"), async ({ request }) => {
    const body = (await request.json()) as { fileName: string };
    uploadSeq += 1;
    return HttpResponse.json(
      {
        mediaId: `0199ffff-0000-7000-8000-${String(uploadSeq).padStart(12, "0")}`,
        uploadUrl: STORAGE_URL,
        expiresAt: freshExpiry(),
        headers: { "Content-Type": body.fileName.endsWith(".pdf") ? "application/pdf" : "text/plain; charset=utf-8" },
      },
      { status: 201 },
    );
  }),
  http.options("https://storage.mem/put/*", () => new HttpResponse(null, { status: 204, headers: STORAGE_CORS })),
  http.put("https://storage.mem/put/*", () => new HttpResponse(null, { status: 200, headers: STORAGE_CORS })),
  http.post(api("/me/uploads/:mediaId/complete"), ({ params }) =>
    HttpResponse.json({
      id: String(params.mediaId),
      kind: "document",
      fileName: "file",
      contentType: "text/plain; charset=utf-8",
      sizeBytes: 5,
      status: "ready",
    }),
  ),
];

const conflict = (message: string) => errorResponse(409, { error: { code: "INVALID_TRANSITION", message } });
const notFound = (message: string) => errorResponse(404, { error: { code: "NOT_FOUND", message } });

/** What the quiz API holds for one test; handlers read and change it, tests seed and inspect it. */
export interface QuizState {
  /** Start screen per lesson id. */
  overviews: Partial<Record<string, QuizOverview>>;
  /** Attempts by id: open, or their result once submitted. */
  attempts: Partial<Record<string, Attempt>>;
  /** The attempt "Bắt đầu làm bài" opens per lesson id. */
  starts: Partial<Record<string, OpenAttempt>>;
  /** The result a submit produces per attempt id. */
  results: Partial<Record<string, QuizResult>>;
  /** Answers saved, in order. */
  saves: { attemptId: string; questionId: string; optionIds: string[] }[];
  /** Attempt ids submitted, in order. */
  submits: string[];
}

/**
 * Both golden quizzes, never attempted: starting opens the practice attempt, or the assessment one with 10 minutes
 * on its clock from now.
 */
export function quizState(overrides: Partial<QuizState> = {}): QuizState {
  return {
    overviews: { [ids.quizPractice]: fixtures.practiceOverview, [ids.quizAssessment]: fixtures.assessmentOverview },
    attempts: {},
    starts: { [ids.quizPractice]: fixtures.openPracticeNew, [ids.quizAssessment]: openAssessment() },
    results: {
      [ids.practiceAttempt]: fixtures.resultPractice,
      [ids.assessmentAttempt]: fixtures.resultAssessmentPending,
    },
    saves: [],
    submits: [],
    ...overrides,
  };
}

/**
 * Closes the attempt `result` belongs to as the server does on submit, or on its first read past the deadline:
 * the attempt reads as its result, and the overview drops it as open and lists it first.
 */
export function closeAttempt(state: QuizState, result: QuizResult) {
  state.attempts[result.id] = result;
  for (const [lessonId, overview] of Object.entries(state.overviews)) {
    if (overview?.openAttemptId !== result.id) continue;
    const { openAttemptId: _closed, ...rest } = overview;
    state.overviews[lessonId] = {
      ...rest,
      attempts: [
        {
          id: result.id,
          no: result.no,
          submittedAt: result.submittedAt,
          autoSubmitted: result.autoSubmitted,
          score: result.score,
          maxScore: result.maxScore,
          percent: result.percent,
          ...(result.mode === "assessment"
            ? { passed: result.passed ?? undefined, reviewStatus: result.reviewStatus ?? undefined }
            : {}),
        },
        ...overview.attempts,
      ],
    };
  }
}

/**
 * The student quiz API over `state`, following the server's rules: starting resumes the open attempt, a save or
 * submit on a submitted attempt answers 409, and a submission closes the attempt in the overview.
 */
export function quizHandlers(state: QuizState) {
  const lessonPathOf = "/me/classes/:classId/lessons/:lessonId/attempts";
  return [
    http.get(api(lessonPathOf), ({ params }) => {
      const overview = state.overviews[String(params.lessonId)];
      return overview ? HttpResponse.json(overview) : errorResponse(404, errors.lessonNotInCourse);
    }),
    http.post(api(lessonPathOf), ({ params }) => {
      const lessonId = String(params.lessonId);
      const overview = state.overviews[lessonId];
      if (!overview) return errorResponse(404, errors.lessonNotInCourse);
      const open = overview.openAttemptId ? state.attempts[overview.openAttemptId] : undefined;
      if (open?.status === "open") return HttpResponse.json({ resumed: true, attempt: open });
      if (!overview.canStart) return conflict(`${overview.blockedReason ?? "Không làm quiz được"}.`);
      const attempt = state.starts[lessonId];
      if (!attempt) return errorResponse(404, errors.lessonNotInCourse);
      state.attempts[attempt.id] = attempt;
      state.overviews[lessonId] = { ...overview, used: overview.used + 1, openAttemptId: attempt.id };
      return HttpResponse.json({ resumed: false, attempt }, { status: 201 });
    }),
    http.get(api("/me/attempts/:attemptId"), ({ params }) => {
      const attempt = state.attempts[String(params.attemptId)];
      return attempt ? HttpResponse.json(attempt) : notFound("Không tìm thấy lượt làm bài.");
    }),
    http.put(api("/me/attempts/:attemptId/answers/:questionId"), async ({ params, request }) => {
      const attemptId = String(params.attemptId);
      const questionId = String(params.questionId);
      const attempt = state.attempts[attemptId];
      if (!attempt) return notFound("Không tìm thấy lượt làm bài.");
      if (attempt.status !== "open") return conflict("Lượt làm bài này đã được nộp.");
      const { optionIds } = (await request.json()) as { optionIds: string[] };
      const savedAt = new Date().toISOString();
      state.saves.push({ attemptId, questionId, optionIds });
      state.attempts[attemptId] = {
        ...attempt,
        savedAt,
        questions: attempt.questions.map((q) => (q.id === questionId ? { ...q, selectedOptionIds: optionIds } : q)),
      };
      return HttpResponse.json({ savedAt });
    }),
    http.post(api("/me/attempts/:attemptId/submit"), ({ params }) => {
      const attemptId = String(params.attemptId);
      const attempt = state.attempts[attemptId];
      if (!attempt) return notFound("Không tìm thấy lượt làm bài.");
      if (attempt.status !== "open") return conflict("Lượt làm bài này đã được nộp.");
      const result = state.results[attemptId];
      if (!result) return notFound("Không tìm thấy lượt làm bài.");
      state.submits.push(attemptId);
      closeAttempt(state, result);
      return HttpResponse.json(result);
    }),
  ];
}

/** Happy-path student API: basic01 active, basic03 draft, basic00 ended. */
export const learningHandlers = [
  http.get(api("/me/classes"), () => HttpResponse.json(fixtures.myClasses)),
  http.get(api("/me/classes/:classId"), ({ params }) => {
    const roadmap = roadmaps[String(params.classId)];
    return roadmap ? HttpResponse.json(roadmap) : errorResponse(404, errors.notMember);
  }),
  http.get(api("/me/classes/:classId/lessons/:lessonId"), ({ params }) => {
    const classId = String(params.classId);
    if (classId === ids.draft) return errorResponse(409, errors.classNotStarted);
    if (!(classId in roadmaps)) return errorResponse(404, errors.notMember);
    const page = lessons[classId]?.[String(params.lessonId)];
    return page ? HttpResponse.json(withFreshVideoUrl(page)) : errorResponse(404, errors.lessonNotInCourse);
  }),
  http.put(api("/me/classes/:classId/lessons/:lessonId/completion"), async ({ request }) => {
    const body = (await request.json()) as { completed?: boolean };
    return HttpResponse.json(body.completed ? fixtures.completion : fixtures.completionUnticked);
  }),
  http.get(api("/media/:mediaId/url"), () => HttpResponse.json({ ...fixtures.signedUrl, expiresAt: freshExpiry() })),
  // Homework: never submitted; a submission becomes #1, pending.
  http.get(api("/me/classes/:classId/lessons/:lessonId/submissions"), () =>
    HttpResponse.json(fixtures.mySubmissionsNone),
  ),
  http.post(api("/me/classes/:classId/lessons/:lessonId/submissions"), async ({ request }) =>
    HttpResponse.json(submitted(fixtures.mySubmissionsNone, (await request.json()) as SubmitBody), { status: 201 }),
  ),
  ...studentUploadHandlers,
];
