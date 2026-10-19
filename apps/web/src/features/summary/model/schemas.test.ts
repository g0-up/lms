import { describe, expect, it } from "vitest";
import { golden } from "../test/golden";
import { classMatrixSchema, learnerSummarySchema, memberStageSchema } from "./schemas";

// The API's golden responses are the contract: every one must parse with the schema the app uses.
describe("summary schemas against the API golden files", () => {
  it.each([
    ["class-summary", classMatrixSchema, golden.classSummary],
    ["learner-summary", learnerSummarySchema, golden.learnerSummary],
    ["member-stage-summary", memberStageSchema, golden.memberStage],
  ] as const)("parses %s", (_name, schema, payload) => {
    expect(schema.safeParse(payload).error).toBeUndefined();
  });

  it("gives every matrix row one cell per stage, in the stages' order", () => {
    const matrix = classMatrixSchema.parse(golden.classSummary);
    const order = matrix.stages.map((s) => s.stageVersionId);

    expect(matrix.rows).toHaveLength(7);
    expect(matrix.rows.every((r) => r.cells.map((c) => c.stageVersionId).join() === order.join())).toBe(true);
    expect(matrix.rows.every((r) => r.cells.every((c) => c.missing === c.required - c.done))).toBe(true);
  });

  it("carries the quiz facts with quizzes and the homework facts with homework only", () => {
    const summary = learnerSummarySchema.parse(golden.learnerSummary);
    const items = summary.stages.flatMap((s) => s.units.flatMap((u) => u.items));

    expect(items.every((i) => (i.type === "quiz") === (i.quiz !== undefined))).toBe(true);
    expect(items.every((i) => (i.type === "homework") === (i.homework !== undefined))).toBe(true);
  });

  it("summarises the course stage by stage without any course-wide score", () => {
    const summary = learnerSummarySchema.parse(golden.learnerSummary);

    expect(summary.course.stagesTotal).toBe(summary.stages.length);
    expect(summary.course.stagesComplete).toBe(summary.stages.filter((s) => s.complete).length);
    expect(summary.course.stages.map((s) => s.stageVersionId)).toEqual(summary.stages.map((s) => s.stageVersionId));
    expect(Object.keys(golden.learnerSummary.course).sort()).toEqual(["stages", "stagesComplete", "stagesTotal"]);
  });

  it("rejects a tone the badges do not know", () => {
    const summary = learnerSummarySchema.parse(golden.learnerSummary);
    const [stage] = summary.stages;
    const broken = {
      ...summary,
      stages: [{ ...stage, units: [{ ...stage.units[0], items: [{ ...stage.units[0].items[0], status: { ...stage.units[0].items[0].status, tone: "muted" } }] }] }],
    };

    expect(learnerSummarySchema.safeParse(broken).success).toBe(false);
  });
});
