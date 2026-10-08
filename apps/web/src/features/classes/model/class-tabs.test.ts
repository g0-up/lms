import { describe, expect, it } from "vitest";
import { parseTab } from "./class-tabs";

describe("parseTab", () => {
  it("reads the three tabs of the class page", () => {
    expect(parseTab("students")).toBe("students");
    expect(parseTab("report")).toBe("report");
    expect(parseTab("settings")).toBe("settings");
  });

  it("falls back to the students for a missing or unknown tab", () => {
    expect(parseTab(null)).toBe("students");
    expect(parseTab("")).toBe("students");
    expect(parseTab("REPORT")).toBe("students");
    expect(parseTab("audit")).toBe("students");
  });
});
