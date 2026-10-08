import { describe, expect, it } from "vitest";
import {
  altFromFileName,
  DEFAULT_IMAGE_ALT,
  IMAGE_SIZE_MESSAGE,
  IMAGE_TYPE_MESSAGE,
  imageFileError,
  MAX_IMAGE_BYTES,
  mediaContentUrl,
} from "./image-file";
import { MEDIA_SRC } from "./markdown-rules";

describe("image file checks", () => {
  it.each(["image/png", "image/jpeg", "image/webp", "image/gif"])("accepts %s within the limit", (type) => {
    expect(imageFileError({ type, size: MAX_IMAGE_BYTES })).toBeNull();
  });

  it("refuses other types and oversized files", () => {
    expect(imageFileError({ type: "image/svg+xml", size: 10 })).toBe(IMAGE_TYPE_MESSAGE);
    expect(imageFileError({ type: "", size: 10 })).toBe(IMAGE_TYPE_MESSAGE);
    expect(imageFileError({ type: "image/png", size: MAX_IMAGE_BYTES + 1 })).toBe(IMAGE_SIZE_MESSAGE);
  });

  it("derives alt text from the file name without its last extension", () => {
    expect(altFromFileName("so-do_bai 1.final.png")).toBe("so-do_bai 1.final");
    expect(altFromFileName("  sơ đồ lớp .jpg")).toBe("sơ đồ lớp");
    expect(altFromFileName("README")).toBe("README");
    expect(altFromFileName(".png")).toBe(DEFAULT_IMAGE_ALT);
    expect(altFromFileName("")).toBe(DEFAULT_IMAGE_ALT);
  });

  it("builds the media content path the server accepts", () => {
    const url = mediaContentUrl("01990000-0000-7000-8000-000000000123");
    expect(url).toBe("/api/v1/media/01990000-0000-7000-8000-000000000123/content");
    expect(MEDIA_SRC.test(url)).toBe(true);
  });
});
