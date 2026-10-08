import { describe, expect, it } from "vitest";
import { ApiError, NETWORK_ERROR_MESSAGE } from "@/shared/api/errors";
import { fixtures } from "../test/golden";
import {
  checkMarkdownSave,
  fmtDuration,
  LESSON_DURATION_FORMAT,
  LESSON_TITLE_REQUIRED,
  LESSON_VIDEO_REQUIRED,
  lessonEditorPath,
  lessonFormDefaults,
  lessonFormSchema,
  lessonMeta,
  lessonErrorMessage,
  markdownLessonDefaults,
  markdownLessonSchema,
  parseDuration,
  readMarkdownLessonDraft,
  requiredSplitLabel,
  summarizeViolations,
  toLessonBody,
  toLessonPatch,
} from "./lesson-form";
import { LESSON_MARKDOWN_REQUIRED, MARKDOWN_EMBEDDED_IMAGE } from "./markdown-rules";
import { VERSION_IMMUTABLE_MESSAGE } from "./stage-rules";

const issues = (values: unknown) => lessonFormSchema.safeParse(values).error?.issues.map((i) => i.message) ?? [];
const blank = { title: "", required: true, videoMediaId: "", duration: "" };
const notes = { type: "markdown" as const, title: "Notes", required: false, markdownSource: "# x" };

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

  it("requires a title, and a video file for a video", () => {
    expect(issues({ ...blank, type: "video" })).toEqual([LESSON_TITLE_REQUIRED, LESSON_VIDEO_REQUIRED]);
    expect(issues({ ...blank, type: "markdown" })).toEqual([LESSON_TITLE_REQUIRED]);
    expect(markdownLessonSchema.safeParse({ ...notes, title: " " }).error?.issues.map((i) => i.message)).toEqual([
      LESSON_TITLE_REQUIRED,
    ]);
  });

  it("validates the duration of a video only", () => {
    expect(issues({ ...blank, title: "A", type: "video", videoMediaId: "m1", duration: "abc" })).toEqual([
      LESSON_DURATION_FORMAT,
    ]);
    expect(issues({ ...blank, title: "A", type: "markdown", duration: "abc" })).toEqual([]);
  });

  it("sends only the fields of the chosen type", () => {
    const video = lessonFormSchema.parse({ ...blank, title: " Intro ", type: "video", videoMediaId: "m1", duration: "01:05" });
    if (video.type !== "video") throw new Error("expected a video");
    expect(toLessonBody(video)).toEqual({ title: "Intro", type: "video", required: true, videoMediaId: "m1", durationSeconds: 65 });
    expect(toLessonPatch(video)).toEqual({ title: "Intro", required: true, videoMediaId: "m1", durationSeconds: 65 });

    const markdown = markdownLessonSchema.parse({ ...notes, title: " Notes " });
    expect(toLessonBody(markdown)).toEqual({ title: "Notes", type: "markdown", required: false, markdownSource: "# x" });
    expect(toLessonPatch(markdown)).toEqual({ title: "Notes", required: false, markdownSource: "# x" });
    expect(toLessonPatch(markdown, { keepSource: true })).toEqual({ title: "Notes", required: false });
  });

  it("starts a new lesson as a required video and fills an edited one", () => {
    expect(lessonFormDefaults()).toEqual({ ...blank, type: "video" });
    expect(lessonFormDefaults(fixtures.lessonVideo)).toMatchObject({
      type: "video",
      title: "Video: JOIN cơ bản",
      videoMediaId: fixtures.lessonVideo.videoMediaId,
      duration: "09:00",
    });
  });

  it("fills the editor page from the lesson, else from the dialog, else blank", () => {
    expect(markdownLessonDefaults(fixtures.lessonMarkdown, { title: "x", required: true })).toEqual({
      type: "markdown",
      title: fixtures.lessonMarkdown.title,
      required: false,
      markdownSource: fixtures.lessonMarkdown.markdownSource,
    });
    expect(markdownLessonDefaults(undefined, { title: "Ghi chú", required: false })).toEqual({
      type: "markdown",
      title: "Ghi chú",
      required: false,
      markdownSource: "",
    });
    expect(markdownLessonDefaults()).toEqual({ type: "markdown", title: "", required: true, markdownSource: "" });
    expect(readMarkdownLessonDraft({ title: "A", required: false })).toEqual({ title: "A", required: false });
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
      expect(checkMarkdownSave(notes, { ...untouched, source: "# y" })).toEqual({ kind: "ok", values: sent, keepSource: false });
    });

    it("refuses empty or embedded-image content it would send", () => {
      expect(checkMarkdownSave(notes, { ...untouched, source: "  " })).toEqual({ kind: "source", message: LESSON_MARKDOWN_REQUIRED });
      expect(checkMarkdownSave(notes, { ...untouched, source: "![a](data:image/png;base64,AA)" })).toEqual({
        kind: "source",
        message: MARKDOWN_EMBEDDED_IMAGE,
      });
    });

    it("refuses edited content that still holds foreign images or unsafe links", () => {
      expect(
        checkMarkdownSave(notes, { lessonId: "l1", contentTouched: true, source: "# x", violations: ["https://x/y.png"] }),
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
    expect(summarizeViolations(["a", "b", "c", "d", "e", "f", "g"])).toEqual({ shown: ["a", "b", "c", "d", "e"], more: 2 });
  });

  it("explains a failed save or preview", () => {
    expect(lessonErrorMessage(new ApiError(409, "VERSION_IMMUTABLE", "x"))).toBe(VERSION_IMMUTABLE_MESSAGE);
    expect(lessonErrorMessage(new ApiError(400, "VALIDATION", "Dữ liệu không hợp lệ.", { body: "Dữ liệu quá lớn (tối đa 64KB)" }))).toBe(
      "Dữ liệu quá lớn (tối đa 64KB)",
    );
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
    expect(requiredSplitLabel(fixtures.stageVersionDraft.lessons)).toBe("1 bắt buộc · 1 tùy chọn");
    expect(requiredSplitLabel([])).toBe("0 bắt buộc · 0 tùy chọn");
  });
});
