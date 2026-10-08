import { describe, expect, it } from "vitest";
import { golden } from "../test/golden";
import {
  applyResultsSchema,
  courseVersionSummarySchema,
  deleteStageVersionSchema,
  lessonSchema,
  mediaSchema,
  stageDetailSchema,
  stageListSchema,
  stageVersionSchema,
  uploadTicketSchema,
} from "./schemas";

// The API's golden responses are the contract: every one must parse with the schema the app uses.
describe("stage schemas against the API golden files", () => {
  it.each([
    ["stage_list", stageListSchema, golden.stageList],
    ["stage_detail", stageDetailSchema, golden.stageDetail],
    ["stage_version", stageVersionSchema, golden.stageVersion],
    ["stage_version_draft", stageVersionSchema, golden.stageVersionDraft],
    ["lesson_video", lessonSchema, golden.lessonVideo],
    ["lesson_markdown", lessonSchema, golden.lessonMarkdown],
    ["delete_version", deleteStageVersionSchema, golden.deleteVersion],
    ["apply_results", applyResultsSchema, golden.applyResults],
    ["course_version", courseVersionSummarySchema, golden.courseVersion],
    ["upload_ticket", uploadTicketSchema, golden.uploadTicket],
    ["media", mediaSchema, golden.media],
  ] as const)("parses %s", (_name, schema, payload) => {
    expect(schema.safeParse(payload).error).toBeUndefined();
  });

  it("keeps per-course apply errors as data", () => {
    const { results } = applyResultsSchema.parse(golden.applyResults);
    expect(results.filter((r) => r.error).map((r) => r.courseCode)).toEqual(["HASDRAFT", "WEBONLY", "DBV2"]);
    expect(results[0]).toMatchObject({ courseCode: "BASIC", newVersionNo: 2 });
  });

  it("rejects a lesson with an unknown type", () => {
    expect(lessonSchema.safeParse({ ...golden.lessonVideo, type: "quiz" }).success).toBe(false);
  });
});
