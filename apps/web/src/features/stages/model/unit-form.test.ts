import { describe, expect, it } from "vitest";
import { UNIT_TITLE_REQUIRED, UNIT_TITLE_TOO_LONG, unitFormSchema } from "./unit-form";

const issues = (title: string) => unitFormSchema.safeParse({ title }).error?.issues.map((i) => i.message) ?? [];

describe("unit form", () => {
  it("trims the title and requires one", () => {
    expect(unitFormSchema.parse({ title: "  Cài đặt  " })).toEqual({ title: "Cài đặt" });
    expect(issues("   ")).toEqual([UNIT_TITLE_REQUIRED]);
  });

  it("allows 120 characters, accents included, and refuses more", () => {
    expect(issues("ệ".repeat(120))).toEqual([]);
    expect(issues(`  ${"a".repeat(120)}  `)).toEqual([]);
    expect(issues("a".repeat(121))).toEqual([UNIT_TITLE_TOO_LONG]);
    expect(UNIT_TITLE_TOO_LONG).toBe("Tên bài học tối đa 120 ký tự.");
  });
});
