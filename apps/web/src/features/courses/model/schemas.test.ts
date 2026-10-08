import { describe, expect, it } from "vitest";
import { golden } from "../test/golden";
import { courseDetailSchema, courseListSchema, courseVersionSchema, deleteCourseVersionSchema } from "./schemas";

// The API's golden responses are the contract: every one must parse with the schema the app uses.
describe("course schemas against the API golden files", () => {
  it.each([
    ["course_list", courseListSchema, golden.courseList],
    ["course_detail", courseDetailSchema, golden.courseDetail],
    ["course_version", courseVersionSchema, golden.courseVersion],
    ["course_version_draft", courseVersionSchema, golden.courseVersionDraft],
    ["delete_version", deleteCourseVersionSchema, golden.deleteVersion],
  ] as const)("parses %s", (_name, schema, payload) => {
    expect(schema.safeParse(payload).error).toBeUndefined();
  });

  it("rejects a class with an unknown status", () => {
    const classes = [{ ...golden.courseVersion.classes[0], status: "paused" }];
    expect(courseVersionSchema.safeParse({ ...golden.courseVersion, classes }).success).toBe(false);
  });
});
