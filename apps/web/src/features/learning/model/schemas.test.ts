import { describe, expect, it } from "vitest";
import { golden } from "../test/golden";
import {
  completionSchema,
  lessonPageSchema,
  myClassesSchema,
  mySubmissionsSchema,
  roadmapSchema,
  signedUrlSchema,
} from "./schemas";

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
    ["my-submissions", mySubmissionsSchema, golden.mySubmissions],
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

  it("reads every MVP stage as one bài học holding the stage's lessons in order", () => {
    for (const payload of [golden.myClassActive, golden.myClassDraft]) {
      for (const stage of roadmapSchema.parse(payload).stages) {
        expect(stage.units.map((u) => u.title)).toEqual(["Nội dung chặng"]);
        expect(stage.unitCount).toBe(1);
        expect(stage.units[0].lessons).toEqual(stage.lessons);
        expect(stage.lessons.every((l) => l.unitId === stage.units[0].id)).toBe(true);
      }
    }
  });

  it("tells whether the next lesson starts another bài học", () => {
    const ended = lessonPageSchema.parse(golden.lessonEnded);
    expect(ended).toMatchObject({ crossesUnit: true, nextUnitTitle: "Nội dung chặng" });
    expect(ended.lesson.unit.title).toBe("Nội dung chặng");
    expect(lessonPageSchema.parse(golden.lessonMarkdown).crossesUnit).toBe(false);
    const { crossesUnit: _crossesUnit, ...rest } = golden.lessonMarkdown;
    expect(lessonPageSchema.safeParse(rest).success).toBe(false);
  });

  it("reads homework and quiz lesson content", () => {
    const base = { ...golden.lessonMarkdown, lesson: { ...golden.lessonMarkdown.lesson, type: "homework" } };
    const homework = lessonPageSchema.parse({
      ...base,
      content: {
        type: "homework",
        homework: { briefHtml: "<p>Đề</p>", passPercent: 70, maxPoints: 10, criteria: [], attachments: [] },
      },
    });
    expect(homework.content.type).toBe("homework");
    const quiz = lessonPageSchema.parse({ ...base, lesson: { ...base.lesson, type: "quiz" }, content: { type: "quiz" } });
    expect(quiz.lesson.type).toBe("quiz");
    expect(lessonPageSchema.safeParse({ ...base, content: { type: "homework" } }).success).toBe(false);
  });

  it("reads the student's submissions newest first, with the grade of each", () => {
    const mine = mySubmissionsSchema.parse(golden.mySubmissions);
    expect(mine.submissions.map((s) => [s.no, s.status])).toEqual([
      [2, "pending"],
      [1, "rework"],
    ]);
    expect(mine.submissions[0].grade).toBeNull();
    expect(mine.submissions[1].grade?.scores.map((s) => s.points)).toEqual([30, 20]);
    expect(mine.lockedReason).toBeNull();
  });

  it("rejects an unknown submission status", () => {
    const [latest, ...rest] = golden.mySubmissions.submissions;
    const payload = { ...golden.mySubmissions, submissions: [{ ...latest, status: "draft" }, ...rest] };
    expect(mySubmissionsSchema.safeParse(payload).success).toBe(false);
  });

  it("rejects a lesson page without content", () => {
    const { content: _content, ...rest } = golden.lessonMarkdown;
    expect(lessonPageSchema.safeParse(rest).success).toBe(false);
  });
});
