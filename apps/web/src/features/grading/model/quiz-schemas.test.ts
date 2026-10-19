import { describe, expect, it } from "vitest";
import { golden } from "../test/golden";
import { attemptDetailSchema, classQuizzesSchema, reviewOutcomeSchema } from "./quiz-schemas";

// The API's golden responses are the contract: every one must parse with the schema the app uses.
describe("quiz review schemas against the API golden files", () => {
  it.each([
    ["review-assessment", attemptDetailSchema, golden.reviewAssessment],
    ["review-practice", attemptDetailSchema, golden.reviewPractice],
    ["class-quizzes", classQuizzesSchema, golden.classQuizzes],
  ] as const)("parses %s", (_name, schema, payload) => {
    expect(schema.safeParse(payload).error).toBeUndefined();
  });

  it("reads an assessment attempt with the key, its reviews newest first and the other attempts", () => {
    const detail = attemptDetailSchema.parse(golden.reviewAssessment);

    expect(detail).toMatchObject({ mode: "assessment", no: 2, limit: 2, passed: true, passPercent: 70 });
    expect(detail.questions.map((q) => q.status)).toEqual(["correct", "blank", "correct"]);
    expect(detail.questions.flatMap((q) => q.options).some((o) => o.isCorrect)).toBe(true);
    expect(detail.reviews.at(0)?.comment).toBe("Đạt ngưỡng. Lượt sau chú ý câu về chỉ mục kết hợp.");
    expect(detail.attempts.map((a) => [a.no, a.reviewStatus])).toEqual([
      [2, "confirmed"],
      [1, "superseded"],
    ]);
  });

  it("reads a practice attempt without attempt limit, time limit or threshold", () => {
    const detail = attemptDetailSchema.parse(golden.reviewPractice);

    expect(detail).toMatchObject({ mode: "practice", limit: null, timeLimitMinutes: null, passed: null, passPercent: null });
  });

  it("reads a practice attempt nobody commented on yet", () => {
    const payload = { ...golden.reviewPractice, reviewStatus: null, reviews: [] };

    expect(attemptDetailSchema.parse(payload).reviewStatus).toBeNull();
  });

  it("rejects a review state the app does not know", () => {
    const payload = { ...golden.reviewAssessment, reviewStatus: "graded" };

    expect(attemptDetailSchema.safeParse(payload).success).toBe(false);
  });

  it("reads the assessment settings only where the quiz has them", () => {
    const { items } = classQuizzesSchema.parse(golden.classQuizzes);
    const practice = items.find((i) => i.mode === "practice");
    const assessment = items.find((i) => i.mode === "assessment");

    expect(practice).not.toHaveProperty("maxAttempts");
    expect(practice).not.toHaveProperty("timeLimitMinutes");
    expect(practice).not.toHaveProperty("passPercent");
    expect(assessment).toMatchObject({ maxAttempts: 2, timeLimitMinutes: 10, passPercent: 70 });
  });

  it("rejects a member status the app does not know", () => {
    const [item, ...rest] = golden.classQuizzes.items;
    const [member, ...others] = item.members;
    const payload = { ...golden.classQuizzes, items: [{ ...item, members: [{ ...member, status: "late" }, ...others] }, ...rest] };

    expect(classQuizzesSchema.safeParse(payload).success).toBe(false);
  });

  it("reads a review outcome without a next item", () => {
    const [review] = golden.reviewAssessment.reviews;
    const payload = { review, reviewStatus: "confirmed", updated: true, nextQueueItem: null };

    expect(reviewOutcomeSchema.parse(payload).nextQueueItem).toBeNull();
  });
});
