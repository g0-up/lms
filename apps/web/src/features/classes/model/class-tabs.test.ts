import { describe, expect, it } from "vitest";
import { CLASS_TABS, parseTab } from "./class-tabs";

describe("parseTab", () => {
  it("reads the six tabs of the class page", () => {
    expect(parseTab("students")).toBe("students");
    expect(parseTab("report")).toBe("report");
    expect(parseTab("summary")).toBe("summary");
    expect(parseTab("homework")).toBe("homework");
    expect(parseTab("quiz")).toBe("quiz");
    expect(parseTab("settings")).toBe("settings");
    expect(CLASS_TABS).toEqual(["students", "report", "summary", "homework", "quiz", "settings"]);
  });

  it("falls back to the students for a missing or unknown tab", () => {
    expect(parseTab(null)).toBe("students");
    expect(parseTab("")).toBe("students");
    expect(parseTab("REPORT")).toBe("students");
    expect(parseTab("audit")).toBe("students");
  });
});
