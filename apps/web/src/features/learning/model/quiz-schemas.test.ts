import { describe, expect, it } from "vitest";
import { golden } from "../test/golden";
import {
  assessmentResultSchema,
  attemptSchema,
  openAttemptSchema,
  practiceResultSchema,
  quizOverviewSchema,
  startedSchema,
} from "./quiz-schemas";
import { lessonPageSchema } from "./schemas";

// The API's golden responses are the contract: every one must parse with the schema the app uses.
describe("quiz schemas against the API golden files", () => {
  it.each([
    ["attempt-open", openAttemptSchema, golden.attemptOpen],
    ["attempts-start", startedSchema, golden.attemptsStart],
    ["attempt-result-practice", practiceResultSchema, golden.resultPractice],
    ["attempt-result-assessment", assessmentResultSchema, golden.resultAssessment],
    ["attempt-result-assessment-reviewed", assessmentResultSchema, golden.resultAssessmentReviewed],
    ["lesson-quiz-attempt", lessonPageSchema, golden.lessonQuiz],
  ] as const)("parses %s", (_name, schema, payload) => {
    expect(schema.safeParse(payload).error).toBeUndefined();
  });

  it.each([
    ["attempt-open", golden.attemptOpen, "open"],
    ["attempt-result-practice", golden.resultPractice, "practice"],
    ["attempt-result-assessment", golden.resultAssessment, "assessment"],
  ] as const)("reads GET /me/attempts/{aid} as %s", (_name, payload, kind) => {
    const attempt = attemptSchema.parse(payload);
    expect(attempt.status === "open" ? "open" : attempt.mode).toBe(kind);
  });

  it("keeps the answer key out of an open attempt", () => {
    const attempt = openAttemptSchema.parse(golden.attemptOpen);
    for (const q of attempt.questions) {
      expect(q).not.toHaveProperty("correct");
      for (const o of q.options) expect(o).not.toHaveProperty("isCorrect");
    }
    expect(JSON.stringify(golden.attemptsStart)).not.toContain("isCorrect");
  });

  it("keeps the answer key out of an assessment result", () => {
    for (const payload of [golden.resultAssessment, golden.resultAssessmentReviewed]) {
      expect(JSON.stringify(payload)).not.toContain("isCorrect");
      expect(assessmentResultSchema.parse(payload)).not.toHaveProperty("questions");
    }
  });

  it("does not take a practice result for an assessment one", () => {
    expect(assessmentResultSchema.safeParse(golden.resultPractice).success).toBe(false);
    expect(practiceResultSchema.safeParse(golden.resultAssessment).success).toBe(false);
  });

  it("reads the overview's omitted fields as absent", () => {
    const overview = quizOverviewSchema.parse({
      mode: "practice",
      questionCount: 3,
      maxScore: 6,
      used: 0,
      granted: 0,
      attempts: [],
      canStart: true,
    });
    expect(overview.limit).toBeUndefined();
    expect(overview.openAttemptId).toBeUndefined();
    expect(overview.best).toBeUndefined();
  });

  it("ignores fields the server adds later", () => {
    const attempt = openAttemptSchema.parse({ ...golden.attemptOpen, extra: true });
    expect(attempt).not.toHaveProperty("extra");
  });
});
