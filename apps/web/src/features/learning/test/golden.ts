/**
 * Fixtures for the learning tests, straight from the API's golden JSON
 * (`apps/api/internal/features/{learning,media,homework}/testdata`). Every fixture goes through the same Zod
 * schema the app uses, so a contract change on the API turns these tests red. Plain data, no
 * network: `model/` tests may use it. Test-only: nothing in the app imports this module.
 */
import completionJson from "../../../../../api/internal/features/learning/testdata/completion.json";
import completionUntickedJson from "../../../../../api/internal/features/learning/testdata/completion-unticked.json";
import classNotActiveJson from "../../../../../api/internal/features/learning/testdata/error-class-not-active.json";
import classNotStartedJson from "../../../../../api/internal/features/learning/testdata/error-class-not-started.json";
import lessonNotInCourseJson from "../../../../../api/internal/features/learning/testdata/error-lesson-not-in-course.json";
import notMemberJson from "../../../../../api/internal/features/learning/testdata/error-not-member.json";
import notOpenedJson from "../../../../../api/internal/features/learning/testdata/error-not-opened.json";
import lessonEndedJson from "../../../../../api/internal/features/learning/testdata/lesson-ended.json";
import lessonMarkdownJson from "../../../../../api/internal/features/learning/testdata/lesson-markdown.json";
import lessonVideoJson from "../../../../../api/internal/features/learning/testdata/lesson-video.json";
import myClassActiveJson from "../../../../../api/internal/features/learning/testdata/my-class-basic01.json";
import myClassDraftJson from "../../../../../api/internal/features/learning/testdata/my-class-draft.json";
import myClassesJson from "../../../../../api/internal/features/learning/testdata/my-classes.json";
import signedUrlJson from "../../../../../api/internal/features/media/testdata/signed_url.json";
import mySubmissionsJson from "../../../../../api/internal/features/homework/testdata/my-submissions.json";
import submissionDetailJson from "../../../../../api/internal/features/homework/testdata/submission-detail.json";
import lessonQuizJson from "../../../../../api/internal/features/learning/testdata/lesson-quiz-attempt.json";
import attemptOpenJson from "../../../../../api/internal/features/quizzes/testdata/attempt-open.json";
import attemptsStartJson from "../../../../../api/internal/features/quizzes/testdata/attempts-start.json";
import resultAssessmentJson from "../../../../../api/internal/features/quizzes/testdata/attempt-result-assessment.json";
import resultAssessmentReviewedJson from "../../../../../api/internal/features/quizzes/testdata/attempt-result-assessment-reviewed.json";
import resultPracticeJson from "../../../../../api/internal/features/quizzes/testdata/attempt-result-practice.json";
import reviewAssessmentJson from "../../../../../api/internal/features/quizzes/testdata/review-assessment.json";
import reviewPracticeJson from "../../../../../api/internal/features/quizzes/testdata/review-practice.json";
import { errorEnvelopeSchema } from "@/shared/api/schemas";
import {
  completionSchema,
  lessonPageSchema,
  myClassesSchema,
  mySubmissionsSchema,
  roadmapSchema,
  signedUrlSchema,
  type LessonPage,
  type LessonStatus,
  type Roadmap,
  type RoadmapLesson,
} from "../model/schemas";
import {
  assessmentResultSchema,
  openAttemptSchema,
  practiceResultSchema,
  quizOverviewSchema,
  startedSchema,
  type OpenAttempt,
  type QuizOverview,
} from "../model/quiz-schemas";

/** Raw golden payloads, for the contract test. */
export const golden = {
  myClasses: myClassesJson,
  myClassActive: myClassActiveJson,
  myClassDraft: myClassDraftJson,
  lessonVideo: lessonVideoJson,
  lessonMarkdown: lessonMarkdownJson,
  lessonEnded: lessonEndedJson,
  completion: completionJson,
  completionUnticked: completionUntickedJson,
  signedUrl: signedUrlJson,
  mySubmissions: mySubmissionsJson,
  lessonQuiz: lessonQuizJson,
  attemptOpen: attemptOpenJson,
  attemptsStart: attemptsStartJson,
  resultPractice: resultPracticeJson,
  resultAssessment: resultAssessmentJson,
  resultAssessmentReviewed: resultAssessmentReviewedJson,
};

export const errors = {
  /** 404: not (or no longer) an active member, or no such class. */
  notMember: errorEnvelopeSchema.parse(notMemberJson),
  /** 404: the lesson is not part of the class's course version. */
  lessonNotInCourse: errorEnvelopeSchema.parse(lessonNotInCourseJson),
  /** 409 CONFLICT: ticking a lesson never opened. */
  notOpened: errorEnvelopeSchema.parse(notOpenedJson),
  /** 409 INVALID_TRANSITION: progress write on a draft or ended class. */
  classNotActive: errorEnvelopeSchema.parse(classNotActiveJson),
  /** 409 INVALID_TRANSITION: opening a lesson of a draft class. */
  classNotStarted: errorEnvelopeSchema.parse(classNotStartedJson),
};

const myClasses = myClassesSchema.parse(myClassesJson);
const roadmapActive = roadmapSchema.parse(myClassActiveJson);

/** basic00 (ended) shares basic01's course version; the API has no golden roadmap for it. */
const roadmapEnded: Roadmap = {
  ...roadmapActive,
  class: { ...roadmapActive.class, id: "01990000-0000-7000-8000-000000000953", code: "basic00", name: "Lập trình cơ bản – khóa 0", status: "ended" },
  readOnly: true,
  readOnlyReason: "ended",
};

/**
 * basic01 with its Database stage split in two bài học: the required lessons stay in "Nội dung chặng",
 * the optional reading moves to "Nâng cao". Same lessons and progress; only the grouping differs.
 */
function splitDatabaseStage(roadmap: Roadmap): Roadmap {
  const [database, ...rest] = roadmap.stages;
  const [core] = database.units;
  const advanced = { id: "01990000-0000-7000-8000-0000000001a2", title: "Nâng cao", position: 2 };
  const lessons = database.lessons.map((l) => (l.required ? l : { ...l, unitId: advanced.id }));
  const units = [
    { ...core, lessons: lessons.filter((l) => l.required) },
    {
      ...advanced,
      requiredDone: 0,
      requiredTotal: 0,
      complete: false,
      optional: true,
      lessons: lessons.filter((l) => !l.required),
    },
  ];
  return { ...roadmap, stages: [{ ...database, unitCount: 2, units, lessons }, ...rest] };
}

const lessonMarkdown = lessonPageSchema.parse(lessonMarkdownJson);

/**
 * The homework of the grading golden (`submission-detail.json`) as a lesson page of basic01's Database
 * stage. The API has no golden lesson page for homework; brief, threshold and rubric are the golden's,
 * plus one attached file.
 */
const homeworkLesson = submissionDetailJson.lesson;
const lessonHomework: LessonPage = lessonPageSchema.parse({
  ...lessonMarkdown,
  lesson: { ...lessonMarkdown.lesson, id: homeworkLesson.id, title: homeworkLesson.title, type: "homework" },
  content: {
    type: "homework",
    homework: {
      briefHtml: homeworkLesson.briefHtml,
      passPercent: homeworkLesson.passPercent,
      maxPoints: homeworkLesson.maxPoints,
      criteria: homeworkLesson.criteria,
      attachments: [{ mediaId: "0199ffff-0000-7000-8000-0000000000a1", fileName: "du-lieu-mau.sql", sizeBytes: 2048 }],
    },
  },
  progress: { firstOpenedAt: "2026-10-05T08:00:00Z" },
});

/** basic01 with the homework appended to its Database stage, as a required lesson. */
/** `roadmap` with `status` on its homework lessons, everywhere the roadmap lists them. */
function withHomeworkStatus(roadmap: Roadmap, status: LessonStatus): Roadmap {
  const mark = (lesson: RoadmapLesson): RoadmapLesson => (lesson.type === "homework" ? { ...lesson, status } : lesson);
  return {
    ...roadmap,
    stages: roadmap.stages.map((stage) => ({
      ...stage,
      lessons: stage.lessons.map(mark),
      units: stage.units.map((unit) => ({ ...unit, lessons: unit.lessons.map(mark) })),
    })),
  };
}

function withHomework(roadmap: Roadmap): Roadmap {
  const [database, ...rest] = roadmap.stages;
  const [core, ...units] = database.units;
  const homework: RoadmapLesson = {
    id: homeworkLesson.id,
    title: homeworkLesson.title,
    type: "homework",
    required: true,
    position: database.lessons.length + 1,
    unitId: core.id,
  };
  const grow = <T extends { requiredTotal: number }>(x: T): T => ({ ...x, requiredTotal: x.requiredTotal + 1 });
  return grow({
    ...roadmap,
    stages: [
      grow({
        ...database,
        lessonCount: database.lessonCount + 1,
        units: [grow({ ...core, lessons: [...core.lessons, homework] }), ...units],
        lessons: [...database.lessons, homework],
      }),
      ...rest,
    ],
  });
}

const mySubmissions = mySubmissionsSchema.parse(mySubmissionsJson);
const [pendingSubmission, reworkSubmission] = mySubmissions.submissions;
const passedHistory = submissionDetailJson.history;

/**
 * The quizzes of the grading goldens as lesson pages of basic01's Database stage: the practice
 * "Luyện tập: khóa chính" asks the questions of `attempt-open.json`, the assessment "Kiểm tra: chỉ mục" those of
 * `attempts-start.json`. The API's golden quiz lesson page lends the shape.
 */
function quizLesson({ id, title }: { id: string; title: string }): LessonPage {
  return lessonPageSchema.parse({
    ...lessonQuizJson,
    lesson: {
      ...lessonQuizJson.lesson,
      id,
      title,
      required: true,
      stage: lessonMarkdown.lesson.stage,
      unit: lessonMarkdown.lesson.unit,
    },
  });
}

const openPractice = openAttemptSchema.parse(attemptOpenJson);
const assessmentAttemptId = "0199ffff-0000-7000-8000-0000000000b1";
const startedAssessment = startedSchema.parse(attemptsStartJson).attempt;

/**
 * The assessment attempt open with `minutesLeft` on its clock, as the server reads it at `now`. The golden's
 * deadline equals its server time, so tests pick the time left.
 */
export function openAssessment(minutesLeft = 10, now = Date.now()): OpenAttempt {
  return {
    ...startedAssessment,
    id: assessmentAttemptId,
    serverNow: new Date(now).toISOString(),
    deadlineAt: new Date(now + minutesLeft * 60_000).toISOString(),
  };
}

const resultPractice = practiceResultSchema.parse(resultPracticeJson);
const resultAssessmentConfirmed = assessmentResultSchema.parse({ ...resultAssessmentJson, id: assessmentAttemptId });
const { review: _review, ...assessmentUnreviewed } = resultAssessmentConfirmed;

const summaryOf = (r: {
  id: string;
  no: number;
  submittedAt: string;
  autoSubmitted: boolean;
  score: number;
  maxScore: number;
  percent: number;
}) => ({
  id: r.id,
  no: r.no,
  submittedAt: r.submittedAt,
  autoSubmitted: r.autoSubmitted,
  score: r.score,
  maxScore: r.maxScore,
  percent: r.percent,
});

const practiceOverview: QuizOverview = quizOverviewSchema.parse({
  mode: "practice",
  questionCount: 3,
  maxScore: 6,
  used: 0,
  granted: 0,
  attempts: [],
  canStart: true,
});

const assessmentOverview: QuizOverview = quizOverviewSchema.parse({
  mode: "assessment",
  questionCount: 3,
  maxScore: 10,
  passPercent: 70,
  timeLimitMinutes: 10,
  limit: 2,
  used: 0,
  granted: 0,
  attempts: [],
  canStart: true,
});

const quizFixtures = {
  lessonQuizPractice: quizLesson(reviewPracticeJson.lesson),
  lessonQuizAssessment: quizLesson(reviewAssessmentJson.lesson),
  /** The practice attempt as the API resumes it: every question answered, saved once. */
  openPractice,
  /** The same attempt fresh from "Bắt đầu làm bài": nothing picked, nothing saved. */
  openPracticeNew: {
    ...openPractice,
    savedAt: null,
    questions: openPractice.questions.map((q) => ({ ...q, selectedOptionIds: [] })),
  } satisfies OpenAttempt,
  /** 5/6: questions 1 and 2 right, 3 wrong with the teacher's note; the teacher commented. */
  resultPractice,
  /** 7/10, Đạt, confirmed with a note on question 2; one attempt left. */
  resultAssessmentConfirmed,
  /** The same score just submitted: waiting for the teacher. */
  resultAssessmentPending: assessmentResultSchema.parse({ ...assessmentUnreviewed, reviewStatus: "pending" }),
  /** #2, 2/2 used, its note on question 3. */
  resultAssessmentReviewed: assessmentResultSchema.parse({ ...resultAssessmentReviewedJson, id: assessmentAttemptId }),
  practiceOverview,
  practiceOverviewOpen: quizOverviewSchema.parse({ ...practiceOverview, used: 1, openAttemptId: openPractice.id }),
  practiceOverviewDone: quizOverviewSchema.parse({
    ...practiceOverview,
    used: 1,
    best: { attemptId: resultPractice.id, percent: resultPractice.percent },
    attempts: [summaryOf(resultPractice)],
  }),
  assessmentOverview,
  assessmentOverviewOpen: quizOverviewSchema.parse({ ...assessmentOverview, used: 1, openAttemptId: assessmentAttemptId }),
  /** #1 passed with 70% and confirmed; one attempt left. */
  assessmentOverviewPassed: quizOverviewSchema.parse({
    ...assessmentOverview,
    used: 1,
    best: { attemptId: assessmentAttemptId, percent: 70, passed: true },
    attempts: [{ ...summaryOf(resultAssessmentConfirmed), passed: true, reviewStatus: "confirmed" }],
  }),
  /** Both attempts failed: blocked with the server's reason. */
  assessmentOverviewOut: quizOverviewSchema.parse({
    ...assessmentOverview,
    used: 2,
    best: { attemptId: assessmentAttemptId, percent: 40, passed: false },
    attempts: [
      { ...summaryOf(resultAssessmentConfirmed), no: 2, score: 4, percent: 40, passed: false, reviewStatus: "pending" },
      {
        ...summaryOf(resultAssessmentConfirmed),
        id: "0199ffff-0000-7000-8000-0000000000b0",
        autoSubmitted: true,
        score: 3,
        percent: 30,
        passed: false,
        reviewStatus: "superseded",
      },
    ],
    canStart: false,
    blockedReason: "Bạn đã dùng hết 2 lượt",
  }),
};

export const fixtures = {
  myClasses,
  roadmapActive,
  roadmapDraft: roadmapSchema.parse(myClassDraftJson),
  roadmapEnded,
  lessonVideo: lessonPageSchema.parse(lessonVideoJson),
  lessonMarkdown,
  /** basic01 with two bài học in its Database stage (see `splitDatabaseStage`). */
  roadmapTwoUnits: roadmapSchema.parse(splitDatabaseStage(roadmapActive)),
  /** The markdown lesson of `roadmapTwoUnits`: its next lesson opens the "Nâng cao" bài học. */
  lessonMarkdownCrossing: lessonPageSchema.parse({ ...lessonMarkdown, crossesUnit: true, nextUnitTitle: "Nâng cao" }),
  lessonEnded: lessonPageSchema.parse(lessonEndedJson),
  completion: completionSchema.parse(completionJson),
  completionUnticked: completionSchema.parse(completionUntickedJson),
  signedUrl: signedUrlSchema.parse(signedUrlJson),
  lessonHomework,
  roadmapWithHomework: roadmapSchema.parse(withHomework(roadmapActive)),
  /** `roadmapWithHomework` with its homework graded Cần làm lại, as the API labels the lesson. */
  roadmapHomeworkRework: roadmapSchema.parse(withHomeworkStatus(withHomework(roadmapActive), { label: "Cần làm lại", tone: "warn" })),
  /** `roadmapWithHomework` once a submission waits for grading. */
  roadmapHomeworkPending: roadmapSchema.parse(withHomeworkStatus(withHomework(roadmapActive), { label: "Chờ chấm", tone: "info" })),
  /** #2 waits for grading (late, repo + one file); #1 was graded Cần làm lại. */
  mySubmissions,
  /** Never submitted, no deadline. */
  mySubmissionsNone: mySubmissionsSchema.parse({
    ...mySubmissionsJson,
    deadline: null,
    status: "notSubmitted",
    submissions: [],
  }),
  /** #1 graded Cần làm lại and nothing newer: the student may submit again. */
  mySubmissionsRework: mySubmissionsSchema.parse({
    ...mySubmissionsJson,
    status: "rework",
    submissions: [reworkSubmission],
  }),
  /** #2 graded Đạt: locked with the server's reason. */
  mySubmissionsPassed: mySubmissionsSchema.parse({
    ...mySubmissionsJson,
    status: "passed",
    canSubmit: false,
    lockedReason: "Bài đã Đạt nên không nộp lại được.",
    submissions: passedHistory,
  }),
  pendingSubmission,
  ...quizFixtures,
};

export const ids = {
  active: fixtures.roadmapActive.class.id,
  draft: fixtures.roadmapDraft.class.id,
  ended: roadmapEnded.class.id,
  video: fixtures.lessonVideo.lesson.id,
  markdown: fixtures.lessonMarkdown.lesson.id,
  optional: fixtures.lessonEnded.lesson.id,
  homework: lessonHomework.lesson.id,
  quizPractice: quizFixtures.lessonQuizPractice.lesson.id,
  quizAssessment: quizFixtures.lessonQuizAssessment.lesson.id,
  practiceAttempt: openPractice.id,
  assessmentAttempt: assessmentAttemptId,
};

/** Golden signed URLs carry a fixed past expiry; tests need one that is still valid (API default TTL 2h). */
export function freshExpiry(ms = 2 * 60 * 60 * 1000): string {
  return new Date(Date.now() + ms).toISOString();
}
