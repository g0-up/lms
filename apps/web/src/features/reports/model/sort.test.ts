import { describe, expect, it } from "vitest";
import { ariaSort, toggleSort } from "./sort";

describe("toggleSort", () => {
  it.each([
    ["name", "pct", "pct"],
    ["pct", "pct", "pct-desc"],
    ["pct-desc", "pct", "pct"],
    ["activity", "pct", "pct"],
    ["name", "activity", "activity"],
    ["activity", "activity", "activity-asc"],
    ["activity-asc", "activity", "activity"],
    ["pct-desc", "activity", "activity"],
  ] as const)("from %s, clicking %s sorts by %s", (current, column, expected) => {
    expect(toggleSort(current, column)).toBe(expected);
  });
});

describe("ariaSort", () => {
  it.each([
    ["pct", "pct", "ascending"],
    ["pct-desc", "pct", "descending"],
    ["activity", "activity", "descending"],
    ["activity-asc", "activity", "ascending"],
    ["name", "pct", undefined],
    ["pct", "activity", undefined],
  ] as const)("sort %s on column %s is %s", (current, column, expected) => {
    expect(ariaSort(current, column)).toBe(expected);
  });
});
