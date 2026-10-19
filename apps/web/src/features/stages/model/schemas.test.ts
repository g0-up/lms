import { describe, expect, it } from "vitest";
import { mediaSchema, uploadTicketSchema } from "@/shared/api/upload-media";
import { golden } from "../test/golden";
import {
  applyResultsSchema,
  courseVersionSummarySchema,
  deleteStageVersionSchema,
  lessonSchema,
  questionSchema,
  quizSchema,
  stageDetailSchema,
  stageListSchema,
  stageVersionSchema,
  unitSchema,
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
    ["lesson_quiz", lessonSchema, golden.lessonQuiz],
    ["lesson_homework", lessonSchema, golden.lessonHomework],
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

  it("reads an MVP version as one bài học holding every lesson", () => {
    for (const payload of [golden.stageVersion, golden.stageVersionDraft]) {
      const version = stageVersionSchema.parse(payload);
      expect(version.units).toHaveLength(1);
      expect(version.units[0]).toMatchObject({
        title: "Nội dung chặng",
        unitKey: "noi-dung-chang",
        position: 1,
        lessonCount: version.lessons.length,
        requiredCount: version.lessons.filter((l) => l.required).length,
      });
      expect(version.lessons.every((l) => l.unitId === version.units[0].id)).toBe(true);
    }
  });

  it("requires a bài học on every lesson and a whole unit", () => {
    const { unitId: _unitId, ...withoutUnit } = golden.lessonVideo;
    expect(lessonSchema.safeParse(withoutUnit).success).toBe(false);
    const [unit] = stageVersionSchema.parse(golden.stageVersion).units;
    expect(unitSchema.safeParse(unit).success).toBe(true);
    expect(unitSchema.safeParse({ ...unit, unitKey: undefined }).success).toBe(false);
    expect(unitSchema.safeParse({ ...unit, lessonCount: -1 }).success).toBe(false);
  });

  it("rejects a lesson with an unknown type", () => {
    expect(lessonSchema.safeParse({ ...golden.lessonVideo, type: "survey" }).success).toBe(false);
  });

  it("reads a quiz with its questions, options and correct answers", () => {
    const { quiz, homework } = lessonSchema.parse(golden.lessonQuiz);
    expect(homework).toBeUndefined();
    expect(quiz).toMatchObject({ mode: "assessment", maxAttempts: 2, timeLimitMinutes: 10, passPercent: 70, shuffle: true });
    expect(quiz?.questions[0]).toMatchObject({ questionKey: "join-left", type: "single", points: 1 });
    expect(quiz?.questions[0].options.map((o) => o.isCorrect)).toEqual([true, false]);
  });

  it("accepts a practice quiz without limits but not a question of an unknown type", () => {
    const quiz = lessonSchema.parse(golden.lessonQuiz).quiz;
    const practice = { ...quiz, mode: "practice", maxAttempts: null, timeLimitMinutes: null, passPercent: null };
    expect(quizSchema.safeParse(practice).success).toBe(true);
    expect(quizSchema.safeParse({ ...practice, maxAttempts: undefined }).success).toBe(false);
    expect(questionSchema.safeParse({ ...quiz?.questions[0], type: "text" }).success).toBe(false);
  });

  it("reads a homework with its rubric total and attachments", () => {
    const { homework, quiz } = lessonSchema.parse(golden.lessonHomework);
    expect(quiz).toBeUndefined();
    expect(homework).toMatchObject({ briefSource: "# Viết truy vấn JOIN", passPercent: 70, maxPoints: 60 });
    expect(homework?.criteria).toEqual([expect.objectContaining({ description: "Truy vấn chạy đúng", maxPoints: 60 })]);
    expect(homework?.attachments).toEqual([expect.objectContaining({ fileName: "starter.zip", sizeBytes: 1024 })]);
  });

  it("requires the headers the upload URL was signed with", () => {
    expect(uploadTicketSchema.parse(golden.uploadTicket).headers).toEqual({ "Content-Type": "video/mp4" });
    const { headers: _headers, ...withoutHeaders } = golden.uploadTicket;
    expect(uploadTicketSchema.safeParse(withoutHeaders).success).toBe(false);
  });
});
