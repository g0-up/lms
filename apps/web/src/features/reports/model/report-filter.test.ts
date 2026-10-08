import { describe, expect, it } from "vitest";
import {
  DEFAULT_FILTER,
  filterFromForm,
  hasAnyFilter,
  parseFilter,
  toApiParams,
  toQuery,
  withFilter,
  type ReportFilter,
} from "./report-filter";

const full: ReportFilter = {
  notLoggedIn: true,
  inactiveDays: 7,
  belowPercent: 50,
  includeDropped: true,
  sort: "pct-desc",
};

describe("parseFilter", () => {
  it("reads the short URL names into the filter", () => {
    expect(parseFilter(new URLSearchParams("notlogged=1&inactive=7&below=50&dropped=1&sort=pct-desc"))).toEqual(full);
  });

  it("falls back to the defaults on an empty URL", () => {
    expect(parseFilter(new URLSearchParams())).toEqual(DEFAULT_FILTER);
  });

  it.each([
    ["inactive=0", { inactiveDays: null }],
    ["inactive=abc", { inactiveDays: null }],
    ["inactive=2.5", { inactiveDays: null }],
    ["below=0", { belowPercent: null }],
    ["below=101", { belowPercent: null }],
    ["below=", { belowPercent: null }],
    ["sort=random", { sort: "name" }],
    ["notlogged=true", { notLoggedIn: false }],
    ["dropped=yes", { includeDropped: false }],
  ])("drops the malformed value in %s", (query, expected) => {
    expect(parseFilter(new URLSearchParams(query))).toMatchObject(expected);
  });

  it("ignores the API names, so they cannot be forwarded from the URL", () => {
    expect(parseFilter(new URLSearchParams("belowPercent=50&notLoggedIn=true&includeDropped=true"))).toEqual(
      DEFAULT_FILTER,
    );
  });
});

describe("toQuery and toApiParams", () => {
  it("map each field to its short URL name", () => {
    expect(toQuery(full).toString()).toBe("notlogged=1&inactive=7&below=50&dropped=1&sort=pct-desc");
  });

  it("map each field to its API name", () => {
    expect(Object.fromEntries(toApiParams(full))).toEqual({
      notLoggedIn: "true",
      inactiveDays: "7",
      belowPercent: "50",
      includeDropped: "true",
      sort: "pct-desc",
    });
  });

  it("leave the defaults out", () => {
    expect(toQuery(DEFAULT_FILTER).toString()).toBe("");
    expect(toApiParams(DEFAULT_FILTER).toString()).toBe("");
  });

  it("round-trip through the URL", () => {
    expect(parseFilter(toQuery(full))).toEqual(full);
  });
});

describe("withFilter", () => {
  it("keeps the other URL parameters and replaces the filter", () => {
    const next = withFilter(new URLSearchParams("tab=report&below=10&sort=pct"), { ...DEFAULT_FILTER, inactiveDays: 3 });
    expect(next.toString()).toBe("tab=report&inactive=3");
  });
});

describe("hasAnyFilter", () => {
  it("is false for the sort and the dropped toggle alone", () => {
    expect(hasAnyFilter({ ...DEFAULT_FILTER, sort: "pct", includeDropped: true })).toBe(false);
  });

  it.each([{ notLoggedIn: true }, { inactiveDays: 7 }, { belowPercent: 50 }])("is true with %o", (patch) => {
    expect(hasAnyFilter({ ...DEFAULT_FILTER, ...patch })).toBe(true);
  });
});

describe("filterFromForm", () => {
  it("treats blank and out-of-range numbers as no condition", () => {
    expect(
      filterFromForm({ notLoggedIn: false, inactive: " ", below: "150", includeDropped: false, sort: "activity" }),
    ).toEqual({ ...DEFAULT_FILTER, sort: "activity" });
  });

  it("reads the typed numbers", () => {
    expect(filterFromForm({ notLoggedIn: true, inactive: "14", below: "30", includeDropped: true, sort: "name" })).toEqual(
      { notLoggedIn: true, inactiveDays: 14, belowPercent: 30, includeDropped: true, sort: "name" },
    );
  });
});
