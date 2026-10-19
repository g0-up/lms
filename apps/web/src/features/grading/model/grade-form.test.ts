import { describe, expect, it } from "vitest";
import { ApiError } from "@/shared/api/errors";
import { fixtures } from "../test/golden";
import {
  GRADE_MESSAGES,
  gradeDefaults,
  gradeFieldErrors,
  gradeFormSchema,
  gradeTotal,
  gradedMessage,
  type GradeFormInput,
} from "./grade-form";

const { criteria, passPercent } = fixtures.submissionPending.lesson;
const [query, explain] = criteria;
const schema = gradeFormSchema(criteria);

const input = (points: [string, string], comment = "Tốt.", notes: [string, string] = ["", ""]): GradeFormInput => ({
  scores: points.map((p, i) => ({ points: p, comment: notes[i] })),
  comment,
});

const errorsOf = (value: GradeFormInput) => {
  const result = schema.safeParse(value);
  return result.success ? {} : Object.fromEntries(result.error.issues.map((i) => [i.path.join("."), i.message]));
};

describe("grade form defaults", () => {
  it("starts blank before the first grading", () => {
    expect(gradeDefaults(criteria, null)).toEqual(input(["", ""], ""));
  });

  it("prefills the current grading for an edit", () => {
    const { grade } = fixtures.submissionPassed.submission;

    expect(gradeDefaults(criteria, grade)).toEqual(
      input(["40", "45"], "Đã bổ sung giải thích, đạt yêu cầu.", ["", "Nên nêu lý do chọn JOIN."]),
    );
  });
});

describe("grade form validation", () => {
  it("asks for every criterion and the general comment with the API's words", () => {
    expect(errorsOf(input(["", " "], "  "))).toEqual({
      "scores.0.points": GRADE_MESSAGES.missing(query.description),
      "scores.1.points": 'Nhập điểm cho tiêu chí "Giải thích cách làm".',
      comment: "Nhập nhận xét chung cho học viên.",
    });
  });

  it("keeps points whole and within each criterion's maximum", () => {
    expect(errorsOf(input(["41", "-1"]))).toEqual({
      "scores.0.points": 'Điểm tiêu chí "Truy vấn đúng kết quả" phải từ 0 đến 40.',
      "scores.1.points": GRADE_MESSAGES.range(explain.description, 60),
    });
    expect(errorsOf(input(["2.5", "abc"]))).toHaveProperty("scores.0.points", GRADE_MESSAGES.range(query.description, 40));
    expect(errorsOf(input(["40", "0"]))).toEqual({});
  });

  it("caps a criterion note at 1000 characters and the comment at 5000", () => {
    expect(errorsOf(input(["40", "60"], "x".repeat(5001), ["y".repeat(1001), "y".repeat(1000)]))).toEqual({
      "scores.0.comment": 'Nhận xét tiêu chí "Truy vấn đúng kết quả" tối đa 1000 ký tự.',
      comment: "Nhận xét tối đa 5000 ký tự.",
    });
    expect(errorsOf(input(["40", "60"], "x".repeat(5000)))).toEqual({});
  });

  it("sends trimmed whole points keyed by criterion", () => {
    const body = schema.parse(input([" 40 ", "30"], "  Ổn.  ", ["", " Cần giải thích thêm "]));

    expect(body).toEqual({
      scores: [
        { criterionId: query.id, points: 40, comment: "" },
        { criterionId: explain.id, points: 30, comment: "Cần giải thích thêm" },
      ],
      comment: "Ổn.",
    });
  });
});

describe("live total", () => {
  it("counts blank criteria and holds the verdict until they are filled", () => {
    expect(gradeTotal(criteria, ["", ""], passPercent)).toMatchObject({ total: 0, max: 100, missing: 2 });
    expect(gradeTotal(criteria, ["30", ""], passPercent)).toMatchObject({ total: 30, pct: 30, missing: 1 });
  });

  it("passes from the threshold on, by the API's rule", () => {
    expect(gradeTotal(criteria, ["40", "30"], passPercent)).toEqual({
      total: 70,
      max: 100,
      pct: 70,
      missing: 0,
      passed: true,
    });
    expect(gradeTotal(criteria, ["40", "29"], passPercent)).toMatchObject({ total: 69, passed: false });
  });

  it("clamps what the form will refuse anyway", () => {
    expect(gradeTotal(criteria, ["99", "60"], passPercent)).toMatchObject({ total: 100, pct: 100, passed: true });
    expect(gradeTotal(criteria, ["-5", "abc"], passPercent)).toMatchObject({ total: 0, missing: 0 });
  });
});

describe("server errors", () => {
  it("maps 422 details to the comment and to criteria by id", () => {
    const error = new ApiError(422, "VALIDATION", "Dữ liệu không hợp lệ.", {
      [`scores.${explain.id}`]: GRADE_MESSAGES.range(explain.description, 60),
      comment: GRADE_MESSAGES.commentRequired,
      scores: "Tiêu chí không thuộc bài tập này.",
    });

    expect(gradeFieldErrors(error)).toEqual({
      comment: GRADE_MESSAGES.commentRequired,
      scores: { [explain.id]: GRADE_MESSAGES.range(explain.description, 60) },
    });
  });

  it("ignores errors that are not field errors", () => {
    expect(gradeFieldErrors(new ApiError(409, "CONFLICT", "Chỉ chấm bài nộp mới nhất."))).toEqual({ scores: {} });
    expect(gradeFieldErrors(new Error("offline"))).toEqual({ scores: {} });
  });
});

describe("saved message", () => {
  it("states the total, the verdict and the email", () => {
    expect(gradedMessage({ totalPoints: 85, maxPoints: 100, result: "passed" })).toBe(
      "Đã lưu 85/100 · Đạt. Email kết quả đã gửi cho học viên.",
    );
    expect(gradedMessage({ totalPoints: 50, maxPoints: 100, result: "rework" })).toBe(
      "Đã lưu 50/100 · Cần làm lại. Email kết quả đã gửi cho học viên.",
    );
  });
});
