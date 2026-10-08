import { fmtBytes } from "./video-file";

/** The API's request body limit (`httpx.MaxBodyBytes`); it applies to the JSON body, not the source. */
export const LESSON_BODY_MAX_BYTES = 64 * 1024;

export const LESSON_MARKDOWN_REQUIRED = "Học liệu markdown bắt buộc có nội dung.";
export const MARKDOWN_EMBEDDED_IMAGE = "Ảnh trong markdown phải tải lên, không nhúng base64.";

/** Image src the server keeps: an uploaded media item served by the API (mirrors `mediaContentPath`). */
export const MEDIA_SRC = /^\/api\/v1\/media\/[0-9a-f-]{36}\/content$/;
/** Link targets that survive the server's sanitizer: http(s), mailto, same-site paths and anchors. */
export const SAFE_URL = /^(https?:|mailto:|\/(?!\/)|#)/i;

const encoder = new TextEncoder();

export function utf8Bytes(text: string): number {
  return encoder.encode(text).length;
}

/** Size of the exact string `http()` sends: JSON escaping of newlines, quotes and backslashes included. */
export function jsonBodyBytes(body: unknown): number {
  return utf8Bytes(JSON.stringify(body));
}

/** Why the lesson body would be refused as too large, or null. */
export function lessonBodySizeError(body: unknown): string | null {
  const bytes = jsonBodyBytes(body);
  return bytes > LESSON_BODY_MAX_BYTES ? `Nội dung quá dài (${fmtBytes(bytes)} / 64 KB).` : null;
}

/**
 * Checks that need only the source text. External images and unsafe links are checked on the
 * editor tree instead (it also sees content loaded at mount), and the server has the last word.
 */
export function markdownSourceError(source: string): string | null {
  if (source.trim() === "") return LESSON_MARKDOWN_REQUIRED;
  if (source.includes("data:image/")) return MARKDOWN_EMBEDDED_IMAGE;
  return null;
}
