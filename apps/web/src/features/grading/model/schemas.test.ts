import { describe, expect, it } from "vitest";
import { golden } from "../test/golden";
import { classHomeworkSchema, queueSchema, recentListSchema, submissionDetailSchema } from "./schemas";

// The API's golden responses are the contract: every one must parse with the schema the app uses.
describe("grading schemas against the API golden files", () => {
  it.each([
    ["queue-teacher", queueSchema, golden.queueTeacher],
    ["queue-admin", queueSchema, golden.queueAdmin],
    ["queue-recent", recentListSchema, golden.recent],
    ["submission-detail", submissionDetailSchema, golden.submissionDetail],
    ["class-homework", classHomeworkSchema, golden.classHomework],
  ] as const)("parses %s", (_name, schema, payload) => {
    expect(schema.safeParse(payload).error).toBeUndefined();
  });

  it.each([
    ["teacher", golden.queueTeacher],
    ["admin", golden.queueAdmin],
  ] as const)("keeps the %s queue oldest first with counts over the whole queue", (_name, payload) => {
    const queue = queueSchema.parse(payload);
    const waits = queue.items.map((i) => i.waitedHours);

    expect(waits).toEqual([...waits].sort((a, b) => b - a));
    expect(queue.items.every((i) => i.overdue === i.waitedHours > 72)).toBe(true);
    expect(queue).toMatchObject({
      total: queue.items.length,
      quizPending: queue.items.filter((i) => i.kind === "quiz").length,
      overdueCount: queue.items.filter((i) => i.overdue).length,
    });
    expect(queue.overdueCount).toBeGreaterThan(0);
  });

  it("reads an ungraded submission without its grades list", () => {
    const { grades: _omitted, ...submission } = golden.submissionDetail.submission;
    const payload = { ...golden.submissionDetail, submission: { ...submission, status: "pending", grade: null } };

    expect(submissionDetailSchema.parse(payload).submission.grades).toEqual([]);
  });

  it("rejects a submission status the app does not know", () => {
    const payload = { ...golden.submissionDetail, submission: { ...golden.submissionDetail.submission, status: "late" } };

    expect(submissionDetailSchema.safeParse(payload).success).toBe(false);
  });

  it("rejects a queue item that is neither homework nor quiz", () => {
    const [item, ...rest] = golden.queueTeacher.items;
    const payload = { ...golden.queueTeacher, items: [{ ...item, kind: "video" }, ...rest] };

    expect(queueSchema.safeParse(payload).success).toBe(false);
  });
});
