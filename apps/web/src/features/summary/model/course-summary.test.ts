import { describe, expect, it } from "vitest";
import { fixtures } from "../test/golden";
import { assessmentScoreText, homeworkScoreText, missingText } from "./course-summary";
import type { HomeworkScore } from "./schemas";

const [database, dataStructure, golang, react] = fixtures.learner.course.stages;
const homework = (over: Partial<HomeworkScore>): HomeworkScore => ({
  lessonId: "hw",
  title: "Bài tập",
  total: null,
  max: null,
  status: "pending",
  ...over,
});

describe("course summary texts", () => {
  it("shows an assessment's best attempt and verdict, or that it was not taken", () => {
    expect(assessmentScoreText(database.assessments[0])).toBe("Tốt nhất 80% · Đạt");
    expect(assessmentScoreText(golang.assessments[0])).toBe("Tốt nhất 60% · Chưa đạt");
    expect(assessmentScoreText(react.assessments[0])).toBe("Chưa làm");
  });

  it("shows a graded homework's points with its state, and the state alone before grading", () => {
    expect(homeworkScoreText(dataStructure.homework[0])).toBe("94/100 · Đạt");
    expect(homeworkScoreText(golang.homework[0])).toBe("Chưa nộp");
    expect(homeworkScoreText(homework({ status: "pending" }))).toBe("Chờ chấm");
    expect(homeworkScoreText(homework({ status: "rework", total: 40, max: 100 }))).toBe("40/100 · Cần làm lại");
  });

  it("counts the missing lessons of a stage, and says nothing when none is missing", () => {
    expect(missingText(golang.missingCount)).toBe("còn 2 học liệu");
    expect(missingText(database.missingCount)).toBeNull();
  });
});
