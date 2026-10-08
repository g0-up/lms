import { describe, expect, it } from "vitest";
import { golden } from "../test/golden";
import { completionSchema, lessonPageSchema, myClassesSchema, roadmapSchema, signedUrlSchema } from "./schemas";

// The API's golden responses are the contract: every one must parse with the schema the app uses.
describe("learning schemas against the API golden files", () => {
  it.each([
    ["my-classes", myClassesSchema, golden.myClasses],
    ["my-class-basic01", roadmapSchema, golden.myClassActive],
    ["my-class-draft", roadmapSchema, golden.myClassDraft],
    ["lesson-video", lessonPageSchema, golden.lessonVideo],
    ["lesson-markdown", lessonPageSchema, golden.lessonMarkdown],
    ["lesson-ended", lessonPageSchema, golden.lessonEnded],
    ["completion", completionSchema, golden.completion],
    ["completion-unticked", completionSchema, golden.completionUnticked],
    ["signed_url", signedUrlSchema, golden.signedUrl],
  ] as const)("parses %s", (_name, schema, payload) => {
    expect(schema.safeParse(payload).error).toBeUndefined();
  });

  it("reads the draft roadmap as read-only with its reason", () => {
    const roadmap = roadmapSchema.parse(golden.myClassDraft);
    expect(roadmap).toMatchObject({ readOnly: true, readOnlyReason: "draft" });
  });

  it("keeps a null completedAt after unticking", () => {
    expect(completionSchema.parse(golden.completionUnticked).completedAt).toBeNull();
  });

  it("rejects a lesson page without content", () => {
    const { content: _content, ...rest } = golden.lessonMarkdown;
    expect(lessonPageSchema.safeParse(rest).success).toBe(false);
  });
});
