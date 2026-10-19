import { describe, expect, it } from "vitest";
import { dueIn, fmtDate, fmtDateTime, fmtSize, initials, rel, versionLabel } from "./format";

describe("fmtDate / fmtDateTime", () => {
  it("formats in Vietnamese day-first order in the local time zone", () => {
    expect(fmtDate("2026-10-05T09:30:00+07:00")).toBe("05/10/2026");
    expect(fmtDateTime("2026-10-05T09:05:00+07:00")).toBe("05/10/2026 09:05");
  });

  it("converts UTC timestamps to local time, including across midnight", () => {
    expect(fmtDateTime("2026-10-04T18:30:00Z")).toBe("05/10/2026 01:30");
  });

  it("renders an em dash for empty or invalid input", () => {
    expect(fmtDate(null)).toBe("—");
    expect(fmtDate(undefined)).toBe("—");
    expect(fmtDateTime("")).toBe("—");
    expect(fmtDateTime("not a date")).toBe("—");
  });
});

describe("rel", () => {
  const now = new Date("2026-10-05T12:00:00+07:00");

  it("says Vừa xong under a minute, including clock skew into the future", () => {
    expect(rel("2026-10-05T11:59:31+07:00", now)).toBe("Vừa xong");
    expect(rel("2026-10-05T12:00:20+07:00", now)).toBe("Vừa xong");
  });

  it("counts minutes, hours and days", () => {
    expect(rel("2026-10-05T11:55:00+07:00", now)).toBe("5 phút trước");
    expect(rel("2026-10-05T09:00:00+07:00", now)).toBe("3 giờ trước");
    expect(rel("2026-10-02T12:00:00+07:00", now)).toBe("3 ngày trước");
  });

  it("switches to the date after 30 days", () => {
    expect(rel("2026-08-01T12:00:00+07:00", now)).toBe("01/08/2026");
  });

  it("says Chưa có when there is no timestamp", () => {
    expect(rel(null, now)).toBe("Chưa có");
  });
});

describe("fmtSize", () => {
  it("shows whole KB below 1 MiB, never less than 1 KB", () => {
    expect(fmtSize(5)).toBe("1 KB");
    expect(fmtSize(1536)).toBe("2 KB");
    expect(fmtSize(1_048_575)).toBe("1024 KB");
  });

  it("shows MB with one decimal (comma) from 1 MiB", () => {
    expect(fmtSize(1_048_576)).toBe("1,0 MB");
    expect(fmtSize(1.5 * 1_048_576)).toBe("1,5 MB");
    expect(fmtSize(20 * 1_048_576)).toBe("20,0 MB");
  });
});

describe("dueIn", () => {
  const now = new Date("2026-10-19T08:00:00Z");
  it("counts minutes under an hour, hours under two days, then days", () => {
    expect(dueIn("2026-10-19T08:20:00Z", now)).toBe("còn 20 phút");
    expect(dueIn("2026-10-19T08:00:10Z", now)).toBe("còn 1 phút");
    expect(dueIn("2026-10-20T07:00:00Z", now)).toBe("còn 23 giờ");
    expect(dueIn("2026-10-22T08:00:00Z", now)).toBe("còn 3 ngày");
  });

  it("says how long ago a passed deadline was", () => {
    expect(dueIn("2026-10-19T05:00:00Z", now)).toBe("quá hạn 3 giờ");
    expect(dueIn("2026-10-15T08:00:00Z", now)).toBe("quá hạn 4 ngày");
  });
});

describe("versionLabel / initials", () => {
  it("labels versions as v{n}", () => {
    expect(versionLabel({ no: 3 })).toBe("v3");
  });

  it("takes the last two words of a Vietnamese name", () => {
    expect(initials("Trần Minh Quân")).toBe("MQ");
    expect(initials("  Lê   Hương ")).toBe("LH");
    expect(initials("An")).toBe("A");
  });
});
