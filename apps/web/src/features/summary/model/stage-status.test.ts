import { describe, expect, it } from "vitest";
import { fixtures } from "../test/golden";
import type { MatrixCell } from "./schemas";
import { cellLabel, cellMissing, cellState, cellTone, stageBadge } from "./stage-status";

const cell = (over: Partial<MatrixCell>): MatrixCell => ({
  stageVersionId: "sv",
  complete: false,
  done: 2,
  required: 5,
  missing: 3,
  rework: false,
  overdue: false,
  hasReview: false,
  ...over,
});
const done = cell({ complete: true, done: 5, missing: 0 });

describe("stageBadge", () => {
  it("names the student's stages from the golden", () => {
    expect(fixtures.learner.stages.map((s) => stageBadge(s).label)).toEqual([
      "Hoàn thành chặng",
      "Hoàn thành chặng",
      "Đang học",
      "Đang học",
      "Đang học",
    ]);
  });

  it("puts a complete stage first, then overdue, then rework", () => {
    expect(stageBadge({ complete: true, overdue: true, rework: true })).toEqual({ tone: "ok", label: "Hoàn thành chặng" });
    expect(stageBadge({ complete: false, overdue: true, rework: true })).toEqual({ tone: "danger", label: "Có bài quá hạn" });
    expect(stageBadge({ complete: false, overdue: false, rework: true })).toEqual({ tone: "warn", label: "Có bài cần làm lại" });
    expect(stageBadge({ complete: false, overdue: false, rework: false })).toEqual({ tone: "info", label: "Đang học" });
  });
});

describe("matrix cells", () => {
  it("colours a cell by the same order as the legend", () => {
    expect(cellTone(done)).toBe("complete");
    expect(cellTone(cell({ overdue: true, rework: true }))).toBe("overdue");
    expect(cellTone(cell({ rework: true }))).toBe("rework");
    expect(cellTone(cell({}))).toBe("progress");
    expect(cellTone(cell({ done: 0, missing: 5 }))).toBe("todo");
  });

  it("says the state and what is missing in words", () => {
    expect(cellState(done)).toBe("Hoàn thành");
    expect(cellMissing(done)).toBeNull();
    expect(cellState(cell({}))).toBe("2/5 bắt buộc");
    expect(cellMissing(cell({}))).toBe("Còn 3 mục");
  });

  it("names a cell with the member, stage, state, flags and review", () => {
    expect(cellLabel("Nguyễn Hoàng An", "Data structure", { ...done, hasReview: true })).toBe(
      "Nguyễn Hoàng An, Data structure: Hoàn thành, đã nhận xét. Mở tổng kết chặng",
    );
    expect(cellLabel("Hoàng Thu Hà", "Golang basic", cell({ overdue: true, rework: true }))).toBe(
      "Hoàng Thu Hà, Golang basic: Chưa hoàn thành, 2/5 bắt buộc, còn 3 mục, có bài quá hạn, có bài cần làm lại, chưa nhận xét. Mở tổng kết chặng",
    );
  });
});
