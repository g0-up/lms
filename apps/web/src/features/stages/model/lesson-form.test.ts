import { describe, expect, it } from "vitest";
import { ApiError, NETWORK_ERROR_MESSAGE } from "@/shared/api/errors";
import { fixtures } from "../test/golden";
import {
  checkMarkdownSave,
  fmtDuration,
  LESSON_DURATION_FORMAT,
  LESSON_TITLE_REQUIRED,
  LESSON_UNIT_REQUIRED,
  LESSON_VIDEO_REQUIRED,
  dropsContent,
  LESSON_KINDS,
  lessonEditorPath,
  lessonFormDefaults,
  lessonKindOf,
  lessonTypeOf,
  lessonFormSchema,
  lessonMeta,
  lessonErrorMessage,
  markdownLessonDefaults,
  markdownLessonSchema,
  parseDuration,
  QUIZ_ATTEMPTS_RANGE,
  QUIZ_PASS_PERCENT_RANGE,
  QUIZ_TIME_LIMIT_RANGE,
  readMarkdownLessonDraft,
  requiredSplitLabel,
  summarizeViolations,
  toLessonBody,
  toLessonPatch,
  unitMeta,
} from "./lesson-form";
import { LESSON_MARKDOWN_REQUIRED, MARKDOWN_EMBEDDED_IMAGE } from "./markdown-rules";
import { VERSION_IMMUTABLE_MESSAGE } from "./stage-rules";

const quiz = fixtures.lessonQuiz.quiz;
const homework = fixtures.lessonHomework.homework;
if (!quiz || !homework) throw new Error("the golden quiz and homework lessons carry their settings");

const issues = (values: unknown) => lessonFormSchema.safeParse(values).error?.issues.map((i) => i.message) ?? [];
const blank = {
  title: "",
  unitId: "",
  required: true,
  videoMediaId: "",
  duration: "",
  markdownSource: "",
  shuffle: true,
  quizPassPercent: "70",
  maxAttempts: "2",
  timeLimitMinutes: "20",
  homeworkPassPercent: "70",
};
const named = { ...blank, title: " Kiểm tra ", unitId: "u1" };
const notes = { kind: "markdown" as const, title: "Notes", unitId: "u1", required: false, markdownSource: "# x" };

describe("lesson form", () => {
  it("reads and writes mm:ss", () => {
    expect(parseDuration("09:30")).toBe(570);
    expect(parseDuration(" 61:40 ")).toBe(3700);
    expect(parseDuration("9:7")).toBeUndefined();
    expect(parseDuration("")).toBeUndefined();
    expect(fmtDuration(540)).toBe("09:00");
    expect(fmtDuration(3700)).toBe("61:40");
    expect(fmtDuration(undefined)).toBe("");
  });

  it("offers the five kinds with the prototype's labels", () => {
    expect(LESSON_KINDS.map((k) => k.label)).toEqual([
      "Video",
      "Bài đọc (Markdown)",
      "Quiz học liệu (luyện tập, chấm tự động)",
      "Quiz bài tập (giới hạn lượt và giờ, giảng viên xác nhận)",
      "Bài tập nộp repo hoặc file (giảng viên chấm)",
    ]);
    expect(lessonKindOf(fixtures.lessonQuiz)).toBe("quiz:assessment");
    expect(lessonKindOf({ type: "quiz", quiz: { ...quiz, mode: "practice" } })).toBe("quiz:practice");
    expect(lessonKindOf(fixtures.lessonHomework)).toBe("homework");
    expect(lessonTypeOf("quiz:practice")).toBe("quiz");
  });

  it("keeps the content only when switching between the two quiz kinds", () => {
    expect(dropsContent("quiz", "quiz:practice")).toBe(false);
    expect(dropsContent("quiz", "quiz:assessment")).toBe(false);
    expect(dropsContent("video", "video")).toBe(false);
    expect(dropsContent("quiz", "homework")).toBe(true);
    expect(dropsContent("homework", "video")).toBe(true);
    expect(dropsContent("video", "quiz:practice")).toBe(true);
  });

  it("requires a title, a unit, and a video file for a video", () => {
    expect(issues({ ...blank, kind: "video" })).toEqual([
      LESSON_TITLE_REQUIRED,
      LESSON_UNIT_REQUIRED,
      LESSON_VIDEO_REQUIRED,
    ]);
    expect(issues({ ...blank, kind: "markdown" })).toEqual([LESSON_TITLE_REQUIRED, LESSON_UNIT_REQUIRED]);
    expect(
      markdownLessonSchema.safeParse({ ...notes, title: " ", unitId: "" }).error?.issues.map((i) => i.message),
    ).toEqual([LESSON_TITLE_REQUIRED, LESSON_UNIT_REQUIRED]);
  });

  it("validates the duration of a video only", () => {
    expect(issues({ ...blank, title: "A", unitId: "u1", kind: "video", videoMediaId: "m1", duration: "abc" })).toEqual([
      LESSON_DURATION_FORMAT,
    ]);
    expect(issues({ ...blank, title: "A", unitId: "u1", kind: "markdown", duration: "abc" })).toEqual([]);
  });

  it("checks an assessment's limits with the server's messages, and nothing on a practice quiz", () => {
    const limits = { quizPassPercent: "0", maxAttempts: "11", timeLimitMinutes: "181" };
    expect(issues({ ...named, ...limits, kind: "quiz:assessment" })).toEqual([
      QUIZ_PASS_PERCENT_RANGE,
      QUIZ_ATTEMPTS_RANGE,
      QUIZ_TIME_LIMIT_RANGE,
    ]);
    expect(issues({ ...named, ...limits, kind: "quiz:practice" })).toEqual([]);
    expect(
      issues({ ...named, kind: "quiz:assessment", quizPassPercent: "100", maxAttempts: "10", timeLimitMinutes: "180" }),
    ).toEqual([]);
    expect(issues({ ...named, kind: "quiz:assessment", maxAttempts: "1.5" })).toEqual([QUIZ_ATTEMPTS_RANGE]);
    expect(QUIZ_TIME_LIMIT_RANGE).toBe("Thời gian làm bài từ 1 đến 180 phút.");
  });

  it("checks a homework's pass threshold only", () => {
    expect(issues({ ...named, kind: "homework", homeworkPassPercent: "" })).toEqual([QUIZ_PASS_PERCENT_RANGE]);
    expect(issues({ ...named, kind: "homework", quizPassPercent: "0", homeworkPassPercent: "1" })).toEqual([]);
  });

  it("sends only the fields of the chosen type, always with the unit", () => {
    const video = lessonFormSchema.parse({
      ...blank,
      title: " Intro ",
      unitId: "u1",
      kind: "video",
      videoMediaId: "m1",
      duration: "01:05",
    });
    expect(toLessonBody(video)).toEqual({
      title: "Intro",
      unitId: "u1",
      type: "video",
      required: true,
      videoMediaId: "m1",
      durationSeconds: 65,
    });
    expect(toLessonPatch(video)).toEqual({
      title: "Intro",
      unitId: "u1",
      required: true,
      videoMediaId: "m1",
      durationSeconds: 65,
    });

    const markdown = markdownLessonSchema.parse({ ...notes, title: " Notes " });
    expect(toLessonBody(markdown)).toEqual({
      title: "Notes",
      unitId: "u1",
      type: "markdown",
      required: false,
      markdownSource: "# x",
    });
    expect(toLessonPatch(markdown)).toEqual({ title: "Notes", unitId: "u1", required: false, markdownSource: "# x" });
    expect(toLessonPatch(markdown, { keepSource: true })).toEqual({ title: "Notes", unitId: "u1", required: false });
  });

  it("sends no limits for a practice quiz and every limit for an assessment", () => {
    const practice = lessonFormSchema.parse({ ...named, kind: "quiz:practice", shuffle: false });
    expect(toLessonBody(practice)).toEqual({
      title: "Kiểm tra",
      unitId: "u1",
      type: "quiz",
      required: true,
      quiz: { mode: "practice", shuffle: false },
    });
    const assessment = lessonFormSchema.parse({
      ...named,
      kind: "quiz:assessment",
      quizPassPercent: " 80 ",
      maxAttempts: "3",
      timeLimitMinutes: "45",
    });
    expect(toLessonBody(assessment)).toMatchObject({
      type: "quiz",
      quiz: { mode: "assessment", shuffle: true, passPercent: 80, maxAttempts: 3, timeLimitMinutes: 45 },
    });
    const homework = lessonFormSchema.parse({ ...named, kind: "homework", homeworkPassPercent: "60" });
    expect(toLessonBody(homework)).toMatchObject({ type: "homework", homework: { passPercent: 60 } });
  });

  it("sends the type on a patch only when it changes", () => {
    const assessment = lessonFormSchema.parse({ ...named, kind: "quiz:assessment" });
    expect(toLessonPatch(assessment, { from: "quiz" })).not.toHaveProperty("type");
    expect(toLessonPatch(assessment, { from: "quiz" })).toMatchObject({ quiz: { mode: "assessment" } });
    const homework = lessonFormSchema.parse({ ...named, kind: "homework" });
    expect(toLessonPatch(homework, { from: "quiz" })).toEqual({
      title: "Kiểm tra",
      unitId: "u1",
      required: true,
      type: "homework",
      homework: { passPercent: 70 },
    });
    const video = lessonFormSchema.parse({ ...named, kind: "video", videoMediaId: "m1" });
    expect(toLessonPatch(video, { from: "homework" })).toMatchObject({ type: "video", videoMediaId: "m1" });
  });

  it("starts a new lesson as a required video in the given unit and fills an edited one", () => {
    expect(lessonFormDefaults()).toEqual({ ...blank, kind: "video" });
    expect(lessonFormDefaults(undefined, "u2")).toEqual({ ...blank, unitId: "u2", kind: "video" });
    expect(lessonFormDefaults(fixtures.lessonVideo, "u2")).toMatchObject({
      kind: "video",
      title: "Video: JOIN cơ bản",
      unitId: fixtures.lessonVideo.unitId,
      videoMediaId: fixtures.lessonVideo.videoMediaId,
      duration: "09:00",
    });
  });

  it("fills a quiz's settings and a homework's threshold from the lesson", () => {
    expect(lessonFormDefaults(fixtures.lessonQuiz)).toMatchObject({
      kind: "quiz:assessment",
      title: "Kiểm tra JOIN",
      shuffle: true,
      quizPassPercent: "70",
      maxAttempts: "2",
      timeLimitMinutes: "10",
    });
    expect(
      lessonFormDefaults({ ...fixtures.lessonHomework, homework: { ...homework, passPercent: 55 } }),
    ).toMatchObject({
      kind: "homework",
      homeworkPassPercent: "55",
    });
  });

  it("fills the editor page from the lesson, else from the dialog, else blank", () => {
    expect(markdownLessonDefaults(fixtures.lessonMarkdown, { title: "x", unitId: "u9", required: true })).toEqual({
      kind: "markdown",
      title: fixtures.lessonMarkdown.title,
      unitId: fixtures.lessonMarkdown.unitId,
      required: false,
      markdownSource: fixtures.lessonMarkdown.markdownSource,
    });
    expect(markdownLessonDefaults(undefined, { title: "Ghi chú", unitId: "u2", required: false })).toEqual({
      kind: "markdown",
      title: "Ghi chú",
      unitId: "u2",
      required: false,
      markdownSource: "",
    });
    expect(markdownLessonDefaults()).toEqual({
      kind: "markdown",
      title: "",
      unitId: "",
      required: true,
      markdownSource: "",
    });
    expect(readMarkdownLessonDraft({ title: "A", unitId: "u2", required: false })).toEqual({
      title: "A",
      unitId: "u2",
      required: false,
    });
    expect(readMarkdownLessonDraft({ title: "A", required: false })).toBeUndefined();
    expect(readMarkdownLessonDraft({ title: "A" })).toBeUndefined();
    expect(readMarkdownLessonDraft(null)).toBeUndefined();
  });

  it("builds editor page paths", () => {
    expect(lessonEditorPath("s1", "v1")).toBe("/admin/stages/s1/versions/v1/lessons/new");
    expect(lessonEditorPath("s1", "v1", "l1")).toBe("/admin/stages/s1/versions/v1/lessons/l1/edit");
  });

  describe("markdown save check", () => {
    const untouched = { contentTouched: false, source: "# x", violations: [] };

    it("leaves untouched content of an existing lesson out, whatever it holds", () => {
      const legacy = { ...notes, markdownSource: "![a](https://x/y.png) data:image/png;base64,AA" };
      expect(checkMarkdownSave(legacy, { ...untouched, lessonId: "l1", violations: ["https://x/y.png"] })).toEqual({
        kind: "ok",
        values: legacy,
        keepSource: true,
      });
    });

    it("sends the editor's markdown once the content was edited, and always for a new lesson", () => {
      const sent = { ...notes, markdownSource: "# y" };
      expect(checkMarkdownSave(notes, { ...untouched, lessonId: "l1", contentTouched: true, source: "# y" })).toEqual({
        kind: "ok",
        values: sent,
        keepSource: false,
      });
      expect(checkMarkdownSave(notes, { ...untouched, source: "# y" })).toEqual({
        kind: "ok",
        values: sent,
        keepSource: false,
      });
    });

    it("refuses empty or embedded-image content it would send", () => {
      expect(checkMarkdownSave(notes, { ...untouched, source: "  " })).toEqual({
        kind: "source",
        message: LESSON_MARKDOWN_REQUIRED,
      });
      expect(checkMarkdownSave(notes, { ...untouched, source: "![a](data:image/png;base64,AA)" })).toEqual({
        kind: "source",
        message: MARKDOWN_EMBEDDED_IMAGE,
      });
    });

    it("refuses edited content that still holds foreign images or unsafe links", () => {
      expect(
        checkMarkdownSave(notes, {
          lessonId: "l1",
          contentTouched: true,
          source: "# x",
          violations: ["https://x/y.png"],
        }),
      ).toEqual({ kind: "violations", violations: ["https://x/y.png"] });
    });

    it("refuses a JSON body over 64 KB, escaping included", () => {
      // 40 000 newlines are 40 000 bytes of text but 80 000 once escaped as \n.
      const source = `#${"\n".repeat(40_000)}x`;
      const check = checkMarkdownSave(notes, { ...untouched, source });
      expect(check.kind).toBe("size");
    });
  });

  it("lists the first five violations and counts the rest", () => {
    expect(summarizeViolations(["a", "b"])).toEqual({ shown: ["a", "b"], more: 0 });
    expect(summarizeViolations(["a", "b", "c", "d", "e", "f", "g"])).toEqual({
      shown: ["a", "b", "c", "d", "e"],
      more: 2,
    });
  });

  it("explains a failed save or preview", () => {
    expect(lessonErrorMessage(new ApiError(409, "VERSION_IMMUTABLE", "x"))).toBe(VERSION_IMMUTABLE_MESSAGE);
    expect(
      lessonErrorMessage(
        new ApiError(400, "VALIDATION", "Dữ liệu không hợp lệ.", { body: "Dữ liệu quá lớn (tối đa 64KB)" }),
      ),
    ).toBe("Dữ liệu quá lớn (tối đa 64KB)");
    expect(lessonErrorMessage(new ApiError(422, "INVALID", "Ảnh trong markdown phải tải lên qua hệ thống."))).toBe(
      "Ảnh trong markdown phải tải lên qua hệ thống.",
    );
    expect(lessonErrorMessage(new TypeError("Failed to fetch"))).toBe(NETWORK_ERROR_MESSAGE);
  });

  it("describes lessons for the list", () => {
    expect(lessonMeta(fixtures.lessonVideo)).toBe("Video · join-co-ban.mp4 · 09:00");
    expect(lessonMeta(fixtures.lessonMarkdown)).toBe(
      `Markdown · ${String(fixtures.lessonMarkdown.markdownSource?.length)} ký tự`,
    );
    expect(lessonMeta(fixtures.lessonQuiz)).toBe("Quiz bài tập · 1 câu · 1 điểm · đạt từ 70% · 2 lượt · 10 phút");
    const practice = {
      ...quiz,
      mode: "practice" as const,
      passPercent: null,
      maxAttempts: null,
      timeLimitMinutes: null,
    };
    expect(lessonMeta({ ...fixtures.lessonQuiz, quiz: practice })).toBe(
      "Quiz học liệu · 1 câu · 1 điểm · không giới hạn lượt",
    );
    expect(lessonMeta(fixtures.lessonHomework)).toBe("Bài tập · 1 tiêu chí · 60 điểm · đạt từ 70%");
    expect(requiredSplitLabel(1, fixtures.stageVersionDraft.lessons)).toBe(
      "1 bài học · 1 học liệu bắt buộc · 1 tùy chọn",
    );
    expect(requiredSplitLabel(0, [])).toBe("0 bài học · 0 học liệu bắt buộc · 0 tùy chọn");
    expect(unitMeta(fixtures.stageVersionDraft.units[0])).toBe("2 học liệu · 1 bắt buộc");
  });
});
