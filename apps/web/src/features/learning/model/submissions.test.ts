import { describe, expect, it } from "vitest";
import { fixtures } from "../test/golden";
import { isPast, latestResult, repoLabel } from "./submissions";

describe("latestResult", () => {
  it("is null before the first grading", () => {
    expect(latestResult(fixtures.mySubmissionsNone)).toBeNull();
    expect(latestResult({ submissions: [fixtures.pendingSubmission] })).toBeNull();
  });

  it("shows the newest graded submission and says a newer one waits", () => {
    const result = latestResult(fixtures.mySubmissions);
    expect(result?.submission.no).toBe(1);
    expect(result?.grade.result).toBe("rework");
    expect(result?.newerPending).toBe(true);
  });

  it("shows the latest submission's own grade when nothing newer waits", () => {
    const result = latestResult(fixtures.mySubmissionsPassed);
    expect(result?.submission.no).toBe(2);
    expect(result?.grade.result).toBe("passed");
    expect(result?.newerPending).toBe(false);
  });
});

describe("repoLabel / isPast", () => {
  it("drops the https scheme only", () => {
    expect(repoLabel("https://github.com/an/sql-select")).toBe("github.com/an/sql-select");
  });

  it("compares a deadline with now", () => {
    const now = new Date("2026-10-19T05:00:00Z");
    expect(isPast("2026-10-19T04:59:59Z", now)).toBe(true);
    expect(isPast("2026-10-19T05:00:00Z", now)).toBe(false);
  });
});
