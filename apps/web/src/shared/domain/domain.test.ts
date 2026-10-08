import { describe, expect, it } from "vitest";
import { canTransition, canTransitionClass, homeOf, percentOf, ROLES, toPercent } from "./index";

describe("percentOf", () => {
  it("rounds the share to a whole percent", () => {
    expect(percentOf(1, 3)).toBe(33);
    expect(percentOf(2, 3)).toBe(67);
    expect(percentOf(5, 5)).toBe(100);
  });

  it("treats an empty or negative total as 0%", () => {
    expect(percentOf(0, 0)).toBe(0);
    expect(percentOf(3, -1)).toBe(0);
  });

  it("clamps to [0, 100]", () => {
    expect(percentOf(7, 5)).toBe(100);
    expect(toPercent(-4)).toBe(0);
    expect(toPercent(Number.NaN)).toBe(0);
  });
});

describe("canTransition", () => {
  it("allows only draft → published → archived for versions", () => {
    expect(canTransition("draft", "published")).toBe(true);
    expect(canTransition("published", "archived")).toBe(true);
    expect(canTransition("draft", "archived")).toBe(false);
    expect(canTransition("published", "draft")).toBe(false);
    expect(canTransition("archived", "published")).toBe(false);
    expect(canTransition("draft", "draft")).toBe(false);
  });

  it("allows only draft → active → ended for classes", () => {
    expect(canTransitionClass("draft", "active")).toBe(true);
    expect(canTransitionClass("active", "ended")).toBe(true);
    expect(canTransitionClass("ended", "active")).toBe(false);
    expect(canTransitionClass("draft", "ended")).toBe(false);
  });
});

describe("homeOf", () => {
  it("maps each role to its area", () => {
    expect(ROLES.map(homeOf)).toEqual(["/admin", "/teach", "/learn"]);
  });
});
