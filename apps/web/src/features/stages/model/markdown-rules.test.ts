import { describe, expect, it } from "vitest";
import {
  jsonBodyBytes,
  LESSON_MARKDOWN_REQUIRED,
  LESSON_BODY_MAX_BYTES,
  lessonBodySizeError,
  MARKDOWN_EMBEDDED_IMAGE,
  markdownSourceError,
  MEDIA_SRC,
  SAFE_URL,
  utf8Bytes,
} from "./markdown-rules";

const UUID = "01990000-0000-7000-8000-000000000123";

describe("markdown rules", () => {
  it("counts UTF-8 bytes, not characters", () => {
    expect(utf8Bytes("abc")).toBe(3);
    expect(utf8Bytes("Tiếng Việt")).toBeGreaterThan("Tiếng Việt".length);
  });

  it("measures the JSON body, where newlines, quotes and backslashes take two bytes", () => {
    const source = '\n"\\';
    expect(utf8Bytes(source)).toBe(3);
    expect(jsonBodyBytes({ markdownSource: source })).toBe(utf8Bytes('{"markdownSource":"\\n\\"\\\\"}'));
  });

  it("refuses a body over 64 KB even when the source alone is under it", () => {
    const source = "\n".repeat(40 * 1024);
    expect(utf8Bytes(source)).toBeLessThan(LESSON_BODY_MAX_BYTES);
    expect(lessonBodySizeError({ title: "A", markdownSource: source })).toMatch(/^Nội dung quá dài \(.+ \/ 64 KB\)\.$/);
  });

  it("accepts a body of exactly 64 KB and refuses one byte more", () => {
    const overhead = jsonBodyBytes({ markdownSource: "" });
    const exact = { markdownSource: "a".repeat(LESSON_BODY_MAX_BYTES - overhead) };
    expect(jsonBodyBytes(exact)).toBe(LESSON_BODY_MAX_BYTES);
    expect(lessonBodySizeError(exact)).toBeNull();
    expect(lessonBodySizeError({ markdownSource: `${exact.markdownSource}a` })).not.toBeNull();
  });

  it("requires content and refuses base64 images", () => {
    expect(markdownSourceError("  \n")).toBe(LESSON_MARKDOWN_REQUIRED);
    expect(markdownSourceError("![a](data:image/png;base64,AAAA)")).toBe(MARKDOWN_EMBEDDED_IMAGE);
    expect(markdownSourceError("# Bài 1")).toBeNull();
  });

  it("matches only uploaded media paths", () => {
    expect(MEDIA_SRC.test(`/api/v1/media/${UUID}/content`)).toBe(true);
    expect(MEDIA_SRC.test(`/api/v1/media/${UUID}/content?x=1`)).toBe(false);
    expect(MEDIA_SRC.test("/api/v1/media/not-a-uuid/content")).toBe(false);
    expect(MEDIA_SRC.test(`https://cdn.example/api/v1/media/${UUID}/content`)).toBe(false);
    expect(MEDIA_SRC.test("")).toBe(false);
  });

  it.each([
    ["https://example.com", true],
    ["http://example.com", true],
    ["mailto:a@example.com", true],
    ["/a", true],
    ["#x", true],
    ["javascript:alert(1)", false],
    ["JavaScript:alert(1)", false],
    ["//evil.example", false],
    ["data:text/html,x", false],
    ["", false],
  ])("SAFE_URL %s → %s", (url, safe) => {
    expect(SAFE_URL.test(url)).toBe(safe);
  });
});
