import { describe, expect, it } from "vitest";
import { fixtures } from "../test/golden";
import {
  OPTION_BODY_TOO_LONG,
  QUESTION_BODY_REQUIRED,
  QUESTION_NEEDS_CORRECT,
  QUESTION_POINTS_RANGE,
  QUESTION_TOO_FEW_OPTIONS,
  questionCountLabel,
  questionFormDefaults,
  questionFormSchema,
  SINGLE_HAS_ONE_CORRECT,
  lacksCorrectAnswer,
  toQuestionBody,
  type QuestionFormValues,
} from "./question-form";

const question = fixtures.lessonQuiz.quiz?.questions[0];
if (!question) throw new Error("the quiz golden has a question");

const row = (body: string, correct = false, id = "") => ({ id, body, correct });
const blankRows = () => Array.from({ length: 4 }, () => row(""));

function values(overrides: Partial<QuestionFormValues> = {}): QuestionFormValues {
  return {
    bodySource: "JOIN nào giữ mọi dòng bảng trái?",
    type: "single",
    points: "1",
    options: [row("LEFT JOIN", true), row("INNER JOIN"), ...blankRows()],
    explanationSource: "",
    ...overrides,
  };
}

const issues = (v: QuestionFormValues) =>
  questionFormSchema.safeParse(v).error?.issues.map((i) => ({ path: i.path.join("."), message: i.message })) ?? [];

describe("question form", () => {
  it("accepts a single-answer question with two options and one correct", () => {
    expect(issues(values())).toEqual([]);
  });

  it("requires the question text and whole points from 1 to 100", () => {
    expect(issues(values({ bodySource: "  " }))).toEqual([{ path: "bodySource", message: QUESTION_BODY_REQUIRED }]);
    for (const points of ["0", "101", "1.5", "", "abc"]) {
      expect(issues(values({ points }))).toEqual([{ path: "points", message: QUESTION_POINTS_RANGE }]);
    }
    expect(issues(values({ points: "100" }))).toEqual([]);
  });

  it("needs at least two filled options, blank rows not counting", () => {
    const one = values({ options: [row("LEFT JOIN", true), row("   ", true), ...blankRows()] });
    expect(issues(one)).toEqual([{ path: "options.root", message: QUESTION_TOO_FEW_OPTIONS }]);
  });

  it("takes up to six options", () => {
    const six = values({ type: "multi", options: Array.from({ length: 6 }, (_, i) => row(`Phương án ${String(i + 1)}`, i < 3)) });
    expect(issues(six)).toEqual([]);
    expect(toQuestionBody(questionFormSchema.parse(six)).options).toHaveLength(6);
    expect(questionFormSchema.safeParse({ ...six, options: [...six.options, row("Thứ bảy")] }).success).toBe(false);
  });

  it("needs a correct answer among the filled options", () => {
    const none = values({ options: [row("LEFT JOIN"), row("INNER JOIN"), row("", true), ...blankRows().slice(1)] });
    expect(issues(none)).toEqual([{ path: "options.root", message: QUESTION_NEEDS_CORRECT }]);
  });

  it("allows one correct answer on a single-answer question and several on a multi-answer one", () => {
    const two = [row("LEFT JOIN", true), row("RIGHT JOIN", true), row("INNER JOIN"), ...blankRows().slice(1)];
    expect(issues(values({ options: two }))).toEqual([{ path: "options.root", message: SINGLE_HAS_ONE_CORRECT }]);
    expect(issues(values({ type: "multi", options: two }))).toEqual([]);
  });

  it("limits an option to 500 characters after trimming", () => {
    const long = values({ options: [row("a".repeat(501), true), row("b"), ...blankRows()] });
    expect(issues(long)).toEqual([{ path: "options.0.body", message: OPTION_BODY_TOO_LONG }]);
    expect(issues(values({ options: [row(` ${"ệ".repeat(500)} `, true), row("b"), ...blankRows()] }))).toEqual([]);
  });

  it("sends the filled options trimmed, keeping the ids of existing ones", () => {
    const parsed = questionFormSchema.parse(
      values({
        bodySource: "  Câu hỏi  ",
        points: " 3 ",
        options: [row(" LEFT JOIN ", true, "o1"), row(""), row("INNER JOIN"), ...blankRows().slice(1)],
        explanationSource: " Giữ bảng trái. ",
      }),
    );
    expect(toQuestionBody(parsed)).toEqual({
      type: "single",
      bodySource: "Câu hỏi",
      explanationSource: "Giữ bảng trái.",
      points: 3,
      options: [
        { id: "o1", body: "LEFT JOIN", correct: true },
        { body: "INNER JOIN", correct: false },
      ],
    });
  });

  it("fills six rows from the question being edited, else starts blank", () => {
    const defaults = questionFormDefaults(question);
    expect(defaults).toMatchObject({ bodySource: question.bodySource, type: "single", points: "1" });
    expect(defaults.options).toHaveLength(6);
    expect(defaults.options.slice(0, 2)).toEqual(question.options.map((o) => ({ id: o.id, body: o.body, correct: o.isCorrect })));
    expect(defaults.options[2]).toEqual(row(""));
    expect(questionFormDefaults()).toMatchObject({ bodySource: "", type: "single", points: "1", explanationSource: "" });
  });

  it("flags a saved question without a correct answer and counts the list", () => {
    expect(lacksCorrectAnswer(question)).toBe(false);
    expect(lacksCorrectAnswer({ options: question.options.map((o) => ({ ...o, isCorrect: false })) })).toBe(true);
    expect(questionCountLabel([question, { ...question, points: 4 }])).toBe("2 câu · 5 điểm");
    expect(questionCountLabel([])).toBe("0 câu · 0 điểm");
  });
});
