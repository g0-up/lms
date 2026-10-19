import type { ClassMatrix, MatrixRow } from "./schemas";

/**
 * Filter of the class summary matrix, kept in the page URL: `stage` keeps the members who have not completed that
 * stage, `noReview` those without a review for that stage (both are stage version ids); `rework` and `overdue` keep
 * the members with a homework to redo or past its deadline in any stage.
 */
export interface MatrixFilter {
  stage: string | null;
  noReview: string | null;
  rework: boolean;
  overdue: boolean;
}

export const NO_FILTER: MatrixFilter = { stage: null, noReview: null, rework: false, overdue: false };

const URL_KEYS = ["stage", "noReview", "rework", "overdue"] as const;

/** Value of the stage selects for "every stage" (no condition). */
export const ALL_STAGES = "all";

/** A stage the matrix knows, or null: an unknown id from the URL is dropped, never matched. */
function knownStage(value: string | null, stageIds: readonly string[]): string | null {
  return value !== null && stageIds.includes(value) ? value : null;
}

/** Reads the filter from the page URL against the stages of the matrix. */
export function parseMatrixFilter(params: URLSearchParams, stageIds: readonly string[]): MatrixFilter {
  return {
    stage: knownStage(params.get("stage"), stageIds),
    noReview: knownStage(params.get("noReview"), stageIds),
    rework: params.get("rework") === "1",
    overdue: params.get("overdue") === "1",
  };
}

/** URL parameters for `filter`, unset conditions left out. */
export function toMatrixQuery(filter: MatrixFilter): URLSearchParams {
  const params = new URLSearchParams();
  if (filter.stage) params.set("stage", filter.stage);
  if (filter.noReview) params.set("noReview", filter.noReview);
  if (filter.rework) params.set("rework", "1");
  if (filter.overdue) params.set("overdue", "1");
  return params;
}

/** `params` with its filter replaced by `filter`; other parameters (e.g. `tab`) are kept. */
export function withMatrixFilter(params: URLSearchParams, filter: MatrixFilter): URLSearchParams {
  const next = new URLSearchParams(params);
  for (const key of URL_KEYS) next.delete(key);
  for (const [key, value] of toMatrixQuery(filter)) next.set(key, value);
  return next;
}

export function hasMatrixFilter(filter: MatrixFilter): boolean {
  return filter.stage !== null || filter.noReview !== null || filter.rework || filter.overdue;
}

/** Filter for the form fields; `ALL_STAGES` in a select means no condition. */
export function matrixFilterFromForm(form: {
  stage: string;
  noReview: string;
  rework: boolean;
  overdue: boolean;
}): MatrixFilter {
  const stage = (value: string) => (value === ALL_STAGES || value === "" ? null : value);
  return { stage: stage(form.stage), noReview: stage(form.noReview), rework: form.rework, overdue: form.overdue };
}

/** The rows of `matrix` that match every condition of `filter`, in the API's order. */
export function filterRows(matrix: ClassMatrix, filter: MatrixFilter): MatrixRow[] {
  const cellOf = (row: MatrixRow, stageVersionId: string) => row.cells.find((c) => c.stageVersionId === stageVersionId);
  return matrix.rows.filter(
    (row) =>
      (filter.stage === null || cellOf(row, filter.stage)?.complete !== true) &&
      (filter.noReview === null || cellOf(row, filter.noReview)?.hasReview !== true) &&
      (!filter.rework || row.cells.some((c) => c.rework)) &&
      (!filter.overdue || row.cells.some((c) => c.overdue)),
  );
}

/** "3/7 học viên": members shown out of the class's active members. */
export function memberCount(shown: number, total: number): string {
  return `${String(shown)}/${String(total)} học viên`;
}
