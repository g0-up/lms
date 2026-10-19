import { describe, expect, it } from "vitest";
import { TEACH_CLASS_TABS, parseTeachTab } from "./teach-class-tabs";

describe("parseTeachTab", () => {
  it("reads the tabs of the teacher's class page", () => {
    expect(parseTeachTab("report")).toBe("report");
    expect(parseTeachTab("summary")).toBe("summary");
    expect(parseTeachTab("homework")).toBe("homework");
    expect(parseTeachTab("quiz")).toBe("quiz");
    expect(TEACH_CLASS_TABS).toEqual(["report", "summary", "homework", "quiz"]);
  });

  it("falls back to the report for a missing or unknown tab", () => {
    expect(parseTeachTab(null)).toBe("report");
    expect(parseTeachTab("")).toBe("report");
    expect(parseTeachTab("students")).toBe("report");
    expect(parseTeachTab("HOMEWORK")).toBe("report");
  });
});
