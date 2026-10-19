import { describe, expect, it } from "vitest";
import { fromLocalInput, isPastDeadline, toLocalInput } from "./deadline-time";
import { fmtWait, hoursSince, isOverdueWaiting } from "./wait";

describe("deadline in Vietnam time", () => {
  it("shows an API timestamp as Vietnam wall time", () => {
    expect(toLocalInput("2026-10-05T08:00:00Z")).toBe("2026-10-05T15:00");
    expect(toLocalInput("2026-10-05T15:00:00+07:00")).toBe("2026-10-05T15:00");
    expect(toLocalInput(null)).toBe("");
    expect(toLocalInput("not a date")).toBe("");
  });

  it("moves to the next day and year at 17:00 UTC", () => {
    expect(toLocalInput("2026-10-05T16:59:00Z")).toBe("2026-10-05T23:59");
    expect(toLocalInput("2026-10-05T17:00:00Z")).toBe("2026-10-06T00:00");
    expect(toLocalInput("2026-12-31T17:00:00Z")).toBe("2027-01-01T00:00");
  });

  it("sends the entered time with the +07:00 offset", () => {
    const sent = fromLocalInput("2026-10-06T00:00");

    expect(sent).toBe("2026-10-06T00:00:00+07:00");
    expect(new Date(sent ?? "").toISOString()).toBe("2026-10-05T17:00:00.000Z");
    expect(toLocalInput(sent)).toBe("2026-10-06T00:00");
  });

  it("refuses what is not a real time", () => {
    expect(fromLocalInput("")).toBeNull();
    expect(fromLocalInput("2026-02-30T10:00")).toBeNull();
    expect(fromLocalInput("2026-10-05T24:00")).toBeNull();
    expect(fromLocalInput("2026-10-05")).toBeNull();
    expect(fromLocalInput("2028-02-29T10:00")).toBe("2028-02-29T10:00:00+07:00");
  });
});

describe("waiting time", () => {
  const now = new Date("2026-10-09T08:00:00Z");

  it("counts whole hours, never below zero", () => {
    expect(hoursSince("2026-10-09T05:30:00Z", now)).toBe(2);
    expect(hoursSince("2026-10-10T00:00:00Z", now)).toBe(0);
  });

  it("reads hours under a day and days with hours above", () => {
    expect(fmtWait(0)).toBe("1 giờ");
    expect(fmtWait(23)).toBe("23 giờ");
    expect(fmtWait(24)).toBe("1 ngày 0 giờ");
    expect(fmtWait(108)).toBe("4 ngày 12 giờ");
  });

  it("is overdue from three full days of waiting", () => {
    expect(isOverdueWaiting(null)).toBe(false);
    expect(isOverdueWaiting(2)).toBe(false);
    expect(isOverdueWaiting(3)).toBe(true);
  });
});

describe("past deadline", () => {
  it("is past only once the moment has gone", () => {
    const now = new Date("2026-10-05T08:00:00Z");

    expect(isPastDeadline("2026-10-05T07:59:59Z", now)).toBe(true);
    expect(isPastDeadline("2026-10-05T15:00:00+07:00", now)).toBe(false);
  });
});
