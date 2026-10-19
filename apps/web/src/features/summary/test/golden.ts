/**
 * Fixtures for the summary tests, straight from the API's golden JSON (`apps/api/internal/features/summary/testdata`).
 * Every fixture goes through the same Zod schema the app uses, so a contract change on the API turns these tests red.
 * The API normalizes ids per file, so the matrix, the drawer and the student's summary do not share ids. Plain data,
 * no network: `model/` tests may use it. Test-only: nothing in the app imports this module.
 */
import classSummaryJson from "../../../../../api/internal/features/summary/testdata/class-summary.json";
import learnerSummaryJson from "../../../../../api/internal/features/summary/testdata/learner-summary.json";
import memberStageJson from "../../../../../api/internal/features/summary/testdata/member-stage-summary.json";
import {
  classMatrixSchema,
  learnerSummarySchema,
  memberStageSchema,
  type ClassMatrix,
  type MemberStage,
} from "../model/schemas";

/** Raw golden payloads, for the contract test. */
export const golden = {
  classSummary: classSummaryJson,
  learnerSummary: learnerSummaryJson,
  memberStage: memberStageJson,
};

const matrix = classMatrixSchema.parse(classSummaryJson);
const memberStage = memberStageSchema.parse(memberStageJson);

/** basic01's matrix with Trần Thị Bích's Data structure homework graded Cần làm lại (the golden has no rework). */
function withRework(m: ClassMatrix): ClassMatrix {
  const ds = m.stages[1].stageVersionId;
  return {
    ...m,
    rows: m.rows.map((row) =>
      row.name === "Trần Thị Bích"
        ? { ...row, cells: row.cells.map((c) => (c.stageVersionId === ds ? { ...c, rework: true } : c)) }
        : row,
    ),
  };
}

/** An's Data structure stage before Hương wrote the review. */
const memberStageNoReview: MemberStage = { ...memberStage, summary: { ...memberStage.summary, review: null } };

export const fixtures = {
  /** basic01: seven members by name, five stages; Hà, Dũng and Khang overdue in Data structure; reviews for An and Cường. */
  matrix,
  matrixWithRework: withRework(matrix),
  /** An in basic01: Database and Data structure complete (a review on the latter), three stages in progress. */
  learner: learnerSummarySchema.parse(learnerSummaryJson),
  /** An's Data structure stage, complete 6/6, reviewed once by Lê Thu Hương. */
  memberStage,
  memberStageNoReview,
};
