/**
 * Fixtures for the grading tests, straight from the API's golden JSON
 * (`apps/api/internal/features/{grading,homework,quizzes}/testdata`). Every fixture goes through the same Zod schema the
 * app uses, so a contract change on the API turns these tests red. Plain data, no network: `model/` tests may
 * use it. Test-only: nothing in the app imports this module.
 */
import queueAdminJson from "../../../../../api/internal/features/grading/testdata/queue-admin.json";
import queueRecentJson from "../../../../../api/internal/features/grading/testdata/queue-recent.json";
import queueTeacherJson from "../../../../../api/internal/features/grading/testdata/queue-teacher.json";
import classHomeworkJson from "../../../../../api/internal/features/homework/testdata/class-homework.json";
import submissionDetailJson from "../../../../../api/internal/features/homework/testdata/submission-detail.json";
import classQuizzesJson from "../../../../../api/internal/features/quizzes/testdata/class-quizzes.json";
import reviewAssessmentJson from "../../../../../api/internal/features/quizzes/testdata/review-assessment.json";
import reviewPracticeJson from "../../../../../api/internal/features/quizzes/testdata/review-practice.json";
import { attemptDetailSchema, classQuizzesSchema, type AttemptDetail } from "../model/quiz-schemas";
import {
  classHomeworkSchema,
  queueSchema,
  recentListSchema,
  submissionDetailSchema,
  type ClassHomework,
  type Queue,
  type SubmissionDetail,
} from "../model/schemas";

/** Raw golden payloads, for the contract test. */
export const golden = {
  queueTeacher: queueTeacherJson,
  queueAdmin: queueAdminJson,
  recent: queueRecentJson,
  submissionDetail: submissionDetailJson,
  classHomework: classHomeworkJson,
  reviewAssessment: reviewAssessmentJson,
  reviewPractice: reviewPracticeJson,
  classQuizzes: classQuizzesJson,
};

const queueTeacher = queueSchema.parse(queueTeacherJson);
const passed = submissionDetailSchema.parse(submissionDetailJson);
const [latest, first] = passed.history;

/** #2 not graded yet: what a grader opens from the queue. */
const ungraded = { ...latest, status: "pending" as const, grade: null, grades: [] };
const pending: SubmissionDetail = { ...passed, submission: ungraded, history: [ungraded, first] };

/** #1 opened after #2 came in: read-only, with a link to #2. */
const older: SubmissionDetail = { ...passed, submission: first, isLatest: false };

const classHomework = classHomeworkSchema.parse(classHomeworkJson);
const [homework] = classHomework.items;
const [memberPassed, memberPending] = homework.members;

/** One member never submitted and the deadline is past: "Quá hạn" and one overdue count. */
const classHomeworkOverdue: ClassHomework = {
  ...classHomework,
  items: [
    {
      ...homework,
      stats: { ...homework.stats, overdueNotSubmitted: 1 },
      members: [
        memberPassed,
        memberPending,
        {
          memberId: "01990000-0000-7000-8000-000000000915",
          name: "Phạm Thu Trang",
          status: "notSubmitted",
          latest: null,
          waitingDays: null,
        },
      ],
    },
  ],
};

const reviewConfirmed = attemptDetailSchema.parse(reviewAssessmentJson);
const [attemptLatest, attemptFirst] = reviewConfirmed.attempts;

/** #2 waiting for its first confirmation: what a grader opens from the queue. */
const reviewPending: AttemptDetail = {
  ...reviewConfirmed,
  reviewStatus: "pending",
  reviews: [],
  attempts: [{ ...attemptLatest, reviewStatus: "pending", review: null }, attemptFirst],
};

/** #1 (3/10, Chưa đạt) opened after #2 came in: read-only, with a link to #2. */
const reviewSuperseded: AttemptDetail = {
  ...reviewConfirmed,
  id: attemptFirst.id,
  no: attemptFirst.no,
  score: attemptFirst.score,
  percent: attemptFirst.percent,
  passed: attemptFirst.passed,
  correctCount: 1,
  reviewStatus: "superseded",
  isLatest: false,
  reviews: [],
  questions: reviewConfirmed.questions.map((q, i) =>
    i === 0
      ? {
          ...q,
          status: "wrong" as const,
          pointsAwarded: 0,
          options: q.options.map((o) => ({ ...o, selected: !o.isCorrect })),
        }
      : q,
  ),
};

const practiceCommented = attemptDetailSchema.parse(reviewPracticeJson);

/** The same practice attempt before anyone commented on it: no review state yet. */
const practiceUnreviewed: AttemptDetail = {
  ...practiceCommented,
  reviewStatus: null,
  reviews: [],
  attempts: practiceCommented.attempts.map((a) => ({ ...a, reviewStatus: null, review: null })),
};

const classQuizzes = classQuizzesSchema.parse(classQuizzesJson);

const emptyQueue: Queue = { ...queueTeacher, items: [], total: 0, quizPending: 0, overdueCount: 0 };

export const fixtures = {
  /** The teacher's queue oldest first: homework and assessment quiz attempts, the first one overdue. */
  queueTeacher,
  queueAdmin: queueSchema.parse(queueAdminJson),
  emptyQueue,
  recent: recentListSchema.parse(queueRecentJson).items,
  /** #2 graded Đạt 85/100 (late, repo, commit and one file); #1 graded Cần làm lại 50/100. */
  submissionPassed: passed,
  /** #2 waiting for its first grading. */
  submissionPending: pending,
  /** #1 while #2 is the latest submission. */
  submissionOlder: older,
  /** basic01's homework: An passed 85/100, Khang waits for grading; deadline 05/10 15:00 (Vietnam time). */
  classHomework,
  classHomeworkOverdue,
  classHomeworkEnded: { ...classHomework, class: { ...classHomework.class, status: "ended" as const } },
  classHomeworkNone: { ...classHomework, items: [] },
  /** An's assessment #2: 7/10 Đạt, confirmed then edited once; #1 (3/10) superseded. */
  reviewConfirmed,
  reviewPending,
  reviewSuperseded,
  /** An's practice attempt 2/6 with one comment and a note on the wrong question. */
  practiceCommented,
  practiceUnreviewed,
  /** basic01's quizzes, practice and assessment, with every member's state. */
  classQuizzes,
  classQuizzesNone: { ...classQuizzes, items: [] },
};

export const ids = {
  classId: classHomework.class.id,
  lessonId: homework.lesson.id,
  latest: latest.id,
  first: first.id,
  /** Submission id of Khang's pending homework in the class tab. */
  memberPendingSubmission: memberPending.latest?.submissionId ?? "",
  /** An's assessment #2 (also the practice attempt's id) and #1. */
  attemptLatest: attemptLatest.id,
  attemptFirst: attemptFirst.id,
};
