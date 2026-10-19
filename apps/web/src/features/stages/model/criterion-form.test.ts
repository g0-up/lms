import { describe, expect, it } from "vitest";
import { fixtures } from "../test/golden";
import {
  CRITERION_DESCRIPTION_REQUIRED,
  CRITERION_POINTS_RANGE,
  criteriaCountLabel,
  criterionFormDefaults,
  criterionFormSchema,
  homeworkPassLabel,
} from "./criterion-form";

const homework = fixtures.lessonHomework.homework;
if (!homework) throw new Error("the homework golden has a homework");

const issues = (description: string, maxPoints: string) =>
  criterionFormSchema.safeParse({ description, maxPoints }).error?.issues.map((i) => i.message) ?? [];

describe("criterion form", () => {
  it("trims the description and reads whole points from 1 to 100", () => {
    expect(criterionFormSchema.parse({ description: "  Truy vấn chạy đúng ", maxPoints: " 60 " })).toEqual({
      description: "Truy vấn chạy đúng",
      maxPoints: 60,
    });
    expect(issues("Đúng", "1")).toEqual([]);
    expect(issues("Đúng", "100")).toEqual([]);
  });

  it("refuses an empty description and points outside 1 to 100", () => {
    expect(issues("   ", "10")).toEqual([CRITERION_DESCRIPTION_REQUIRED]);
    for (const points of ["", "0", "101", "2.5", "-1"]) expect(issues("Đúng", points)).toEqual([CRITERION_POINTS_RANGE]);
  });

  it("fills the criterion being edited, else starts blank", () => {
    expect(criterionFormDefaults(homework.criteria[0])).toEqual({ description: "Truy vấn chạy đúng", maxPoints: "60" });
    expect(criterionFormDefaults()).toEqual({ description: "", maxPoints: "" });
  });

  it("sums up the rubric and the lowest passing score", () => {
    expect(criteriaCountLabel(homework)).toBe("1 tiêu chí · 60 điểm");
    expect(homeworkPassLabel(homework)).toBe("70% tổng điểm (từ 42/60)");
    expect(homeworkPassLabel({ passPercent: 70, maxPoints: 25 })).toBe("70% tổng điểm (từ 18/25)");
    expect(homeworkPassLabel({ passPercent: 70, maxPoints: 0 })).toBe("70% tổng điểm");
  });
});
