import { describe, expect, it } from "vitest";
import { golden } from "../test/golden";
import { classReportSchema, memberReportSchema, teachClassSchema, teachClassesSchema } from "./schemas";

// The API's golden responses are the contract: every one must parse with the schema the app uses.
describe("report schemas against the API golden files", () => {
  it.each([
    ["class-report", classReportSchema, golden.classReport],
    ["member-report", memberReportSchema, golden.memberReport],
    ["teaching_classes", teachClassesSchema, golden.teachingClasses],
    ["class_detail", teachClassSchema, golden.classDetail],
  ] as const)("parses %s", (_name, schema, payload) => {
    expect(schema.safeParse(payload).error).toBeUndefined();
  });

  it("keeps null activity for a student who never signed in", () => {
    const row = classReportSchema.parse(golden.classReport).rows.find((r) => r.name === "Phạm Minh Dũng");
    expect(row).toMatchObject({ accountStatus: "invited", lastLoginAt: null, lastActivityAt: null });
  });

  it("keeps a null average for a class without active members", () => {
    const basic02 = teachClassesSchema.parse(golden.teachingClasses).items.find((c) => c.code === "basic02");
    expect(basic02?.avgPercent).toBeNull();
  });

  it("reads every lesson type of a member's stages", () => {
    const types = memberReportSchema.parse(golden.memberReport).stages.flatMap((s) => s.lessons.map((l) => l.type));
    expect(new Set(types)).toEqual(new Set(["video", "markdown", "quiz", "homework"]));
  });

  it("rejects a lesson type the app does not know", () => {
    const [stage] = golden.memberReport.stages;
    const lessons = stage.lessons.map((l, i) => (i === 0 ? { ...l, type: "survey" } : l));
    const payload = { ...golden.memberReport, stages: [{ ...stage, lessons }] };
    expect(memberReportSchema.safeParse(payload).success).toBe(false);
  });

  it("rejects a report whose sort is not in the allowlist", () => {
    const payload = { ...golden.classReport, filter: { ...golden.classReport.filter, sort: "email" } };
    expect(classReportSchema.safeParse(payload).success).toBe(false);
  });
});
