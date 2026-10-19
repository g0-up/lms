import { describe, expect, it } from "vitest";
import { fixtures } from "../test/golden";
import {
  ALL_STAGES,
  NO_FILTER,
  filterRows,
  hasMatrixFilter,
  matrixFilterFromForm,
  memberCount,
  parseMatrixFilter,
  toMatrixQuery,
  withMatrixFilter,
} from "./matrix-filter";

const { matrix, matrixWithRework } = fixtures;
const stageIds = matrix.stages.map((s) => s.stageVersionId);
const [db, ds, go] = stageIds;
const names = (rows: { name: string }[]) => rows.map((r) => r.name);

describe("parseMatrixFilter", () => {
  it("reads every condition from the URL", () => {
    const params = new URLSearchParams({ tab: "summary", stage: go, noReview: ds, rework: "1", overdue: "1" });

    expect(parseMatrixFilter(params, stageIds)).toEqual({ stage: go, noReview: ds, rework: true, overdue: true });
  });

  it("drops a stage the class does not have and flags that are not 1", () => {
    const params = new URLSearchParams({ stage: "nope", noReview: "", rework: "true", overdue: "0" });

    expect(parseMatrixFilter(params, stageIds)).toEqual(NO_FILTER);
  });
});

describe("toMatrixQuery and withMatrixFilter", () => {
  it("writes only the conditions that are set", () => {
    expect(toMatrixQuery(NO_FILTER).toString()).toBe("");
    expect(toMatrixQuery({ stage: db, noReview: null, rework: false, overdue: true }).toString()).toBe(
      `stage=${db}&overdue=1`,
    );
  });

  it("keeps the tab and replaces the previous filter", () => {
    const params = new URLSearchParams({ tab: "summary", stage: db, rework: "1" });
    const next = withMatrixFilter(params, { ...NO_FILTER, overdue: true });

    expect(next.toString()).toBe("tab=summary&overdue=1");
    // The current URL is left as it was.
    expect(params.get("stage")).toBe(db);
  });

  it("round-trips through the URL", () => {
    const filter = { stage: go, noReview: ds, rework: true, overdue: false };

    expect(parseMatrixFilter(toMatrixQuery(filter), stageIds)).toEqual(filter);
  });
});

describe("hasMatrixFilter and matrixFilterFromForm", () => {
  it("counts any condition as a filter", () => {
    expect(hasMatrixFilter(NO_FILTER)).toBe(false);
    expect(hasMatrixFilter({ ...NO_FILTER, stage: db })).toBe(true);
    expect(hasMatrixFilter({ ...NO_FILTER, noReview: db })).toBe(true);
    expect(hasMatrixFilter({ ...NO_FILTER, rework: true })).toBe(true);
    expect(hasMatrixFilter({ ...NO_FILTER, overdue: true })).toBe(true);
  });

  it("reads every stage in a select as no condition", () => {
    expect(matrixFilterFromForm({ stage: ALL_STAGES, noReview: "", rework: false, overdue: false })).toEqual(NO_FILTER);
    expect(matrixFilterFromForm({ stage: go, noReview: ds, rework: true, overdue: true })).toEqual({
      stage: go,
      noReview: ds,
      rework: true,
      overdue: true,
    });
  });
});

describe("filterRows", () => {
  it("keeps every row, in the API's order, without a filter", () => {
    expect(names(filterRows(matrix, NO_FILTER))).toEqual([
      "Hoàng Thu Hà",
      "Lê Văn Cường",
      "Nguyễn Hoàng An",
      "Phạm Minh Dũng",
      "Trần Thị Bích",
      "Vũ Đức Khang",
      "Đặng Hải Phong",
    ]);
  });

  it("keeps the members who have not completed a stage", () => {
    expect(names(filterRows(matrix, { ...NO_FILTER, stage: db }))).toEqual([
      "Hoàng Thu Hà",
      "Lê Văn Cường",
      "Phạm Minh Dũng",
      "Vũ Đức Khang",
      "Đặng Hải Phong",
    ]);
  });

  it("keeps the members without a review for a stage", () => {
    expect(names(filterRows(matrix, { ...NO_FILTER, noReview: ds }))).toEqual([
      "Hoàng Thu Hà",
      "Phạm Minh Dũng",
      "Trần Thị Bích",
      "Vũ Đức Khang",
      "Đặng Hải Phong",
    ]);
  });

  it("keeps the members with a homework past its deadline", () => {
    expect(names(filterRows(matrix, { ...NO_FILTER, overdue: true }))).toEqual([
      "Hoàng Thu Hà",
      "Phạm Minh Dũng",
      "Vũ Đức Khang",
    ]);
  });

  it("keeps the members with a homework to redo", () => {
    expect(filterRows(matrix, { ...NO_FILTER, rework: true })).toEqual([]);
    expect(names(filterRows(matrixWithRework, { ...NO_FILTER, rework: true }))).toEqual(["Trần Thị Bích"]);
  });

  it("combines the conditions", () => {
    expect(names(filterRows(matrix, { stage: db, noReview: ds, rework: false, overdue: true }))).toEqual([
      "Hoàng Thu Hà",
      "Phạm Minh Dũng",
      "Vũ Đức Khang",
    ]);
    expect(filterRows(matrix, { stage: ds, noReview: null, rework: false, overdue: false }).map((r) => r.name)).not.toContain(
      "Nguyễn Hoàng An",
    );
  });
});

describe("memberCount", () => {
  it("says how many members are shown out of the class", () => {
    expect(memberCount(3, 7)).toBe("3/7 học viên");
    expect(memberCount(0, 0)).toBe("0/0 học viên");
  });
});
