import { describe, expect, it } from "vitest";
import { fmtAvgHours } from "./format-hours";

describe("fmtAvgHours", () => {
  it("shows whole hours below a day", () => {
    expect(fmtAvgHours(0.2)).toBe("0 giờ");
    expect(fmtAvgHours(19.09)).toBe("19 giờ");
    expect(fmtAvgHours(23.4)).toBe("23 giờ");
  });

  it("switches to days with a decimal comma from a rounded day", () => {
    expect(fmtAvgHours(23.87)).toBe("1,0 ngày");
    expect(fmtAvgHours(24)).toBe("1,0 ngày");
    expect(fmtAvgHours(60)).toBe("2,5 ngày");
  });

  it("shows a dash when nothing was handled", () => {
    expect(fmtAvgHours(null)).toBe("—");
  });
});
