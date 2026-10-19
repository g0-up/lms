import { afterEach, describe, expect, it, vi } from "vitest";
import {
  clearPosition,
  fmtCountdown,
  readPosition,
  remaining,
  resumeIndex,
  toggleOption,
  unansweredCount,
  warningsDue,
  writePosition,
  type WarningMinute,
} from "./quiz-session";

const questions = [{ id: "q1" }, { id: "q2" }, { id: "q3" }];
const MIN = 60_000;

describe("unansweredCount", () => {
  it("counts questions without a picked option", () => {
    expect(unansweredCount(questions, {})).toBe(3);
    expect(unansweredCount(questions, { q1: ["a"], q2: [], q3: ["b", "c"] })).toBe(1);
  });
});

describe("resumeIndex", () => {
  it("returns to the question the student was on", () => {
    expect(resumeIndex(questions, { q1: ["a"] }, 2)).toBe(2);
    expect(resumeIndex(questions, {}, 0)).toBe(0);
  });

  it("falls back to the first unanswered question without a stored position", () => {
    expect(resumeIndex(questions, { q1: ["a"] }, null)).toBe(1);
    expect(resumeIndex(questions, { q1: ["a"], q2: [] }, null)).toBe(1);
  });

  it("ignores a stored position the attempt no longer has", () => {
    expect(resumeIndex(questions, { q1: ["a"], q2: ["b"] }, 3)).toBe(2);
    expect(resumeIndex(questions, {}, -1)).toBe(0);
    expect(resumeIndex(questions, {}, 1.5)).toBe(0);
  });

  it("opens the first question when all are answered", () => {
    expect(resumeIndex(questions, { q1: ["a"], q2: ["b"], q3: ["c"] }, null)).toBe(0);
  });
});

describe("remaining", () => {
  const serverNow = "2026-10-05T08:00:00Z";
  const deadlineAt = "2026-10-05T08:10:00Z";
  const server = Date.parse(serverNow);

  it("counts down from the server's clock as client time passes", () => {
    expect(remaining(deadlineAt, serverNow, server, server)).toBe(10 * MIN);
    expect(remaining(deadlineAt, serverNow, server + 3 * MIN, server)).toBe(7 * MIN);
  });

  it.each([
    ["2 minutes fast", 2 * MIN],
    ["2 minutes slow", -2 * MIN],
  ])("is unaffected by a client clock %s", (_name, skew) => {
    const receivedAt = server + skew;
    expect(remaining(deadlineAt, serverNow, receivedAt, receivedAt)).toBe(10 * MIN);
    expect(remaining(deadlineAt, serverNow, receivedAt + 4 * MIN, receivedAt)).toBe(6 * MIN);
  });

  it("never goes below zero", () => {
    expect(remaining(deadlineAt, serverNow, server + 11 * MIN, server)).toBe(0);
  });
});

describe("warningsDue", () => {
  const none = new Set<WarningMinute>();

  it("fires nothing before five minutes are left", () => {
    expect(warningsDue(5 * MIN + 1, none)).toEqual([]);
  });

  it("fires each mark once it is reached", () => {
    expect(warningsDue(5 * MIN, none)).toEqual([5]);
    expect(warningsDue(MIN, new Set<WarningMinute>([5]))).toEqual([1]);
  });

  it("does not fire a mark twice", () => {
    expect(warningsDue(4 * MIN, new Set<WarningMinute>([5]))).toEqual([]);
    expect(warningsDue(30_000, new Set<WarningMinute>([5, 1]))).toEqual([]);
  });

  it("reports every mark passed while the page was away", () => {
    expect(warningsDue(30_000, none)).toEqual([5, 1]);
  });

  it("stays quiet once time is up", () => {
    expect(warningsDue(0, none)).toEqual([]);
  });
});

describe("fmtCountdown", () => {
  it("shows mm:ss rounded up to the second", () => {
    expect(fmtCountdown(10 * MIN)).toBe("10:00");
    expect(fmtCountdown(5 * MIN - 500)).toBe("05:00");
    expect(fmtCountdown(61_000)).toBe("01:01");
    expect(fmtCountdown(1)).toBe("00:01");
    expect(fmtCountdown(0)).toBe("00:00");
    expect(fmtCountdown(-5)).toBe("00:00");
  });
});

describe("toggleOption", () => {
  it("replaces the pick of a single-answer question", () => {
    expect(toggleOption("single", ["a"], "b")).toEqual(["b"]);
  });

  it("adds and removes picks of a multi-answer question", () => {
    expect(toggleOption("multi", ["a"], "b")).toEqual(["a", "b"]);
    expect(toggleOption("multi", ["a", "b"], "a")).toEqual(["b"]);
  });
});

describe("stored position", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    window.sessionStorage.clear();
  });

  it("keeps the question index per attempt", () => {
    writePosition("a1", 2);
    expect(readPosition("a1")).toBe(2);
    expect(readPosition("a2")).toBeNull();
    clearPosition("a1");
    expect(readPosition("a1")).toBeNull();
  });

  it("ignores a value that is not an index", () => {
    window.sessionStorage.setItem("quiz-position:a1", "two");
    expect(readPosition("a1")).toBeNull();
  });

  it("works without storage", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new DOMException("denied", "SecurityError");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("denied", "SecurityError");
    });
    expect(() => {
      writePosition("a1", 1);
    }).not.toThrow();
    expect(readPosition("a1")).toBeNull();
  });
});
