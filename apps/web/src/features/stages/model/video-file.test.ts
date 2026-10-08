import { describe, expect, it } from "vitest";
import { fmtBytes, MAX_VIDEO_BYTES, VIDEO_SIZE_MESSAGE, VIDEO_TYPE_MESSAGE, videoFileError } from "./video-file";

describe("video file checks", () => {
  it("accepts an mp4 within the limit", () => {
    expect(videoFileError({ type: "video/mp4", size: MAX_VIDEO_BYTES })).toBeNull();
  });

  it("refuses other types and oversized files", () => {
    expect(videoFileError({ type: "video/webm", size: 10 })).toBe(VIDEO_TYPE_MESSAGE);
    expect(videoFileError({ type: "video/mp4", size: MAX_VIDEO_BYTES + 1 })).toBe(VIDEO_SIZE_MESSAGE);
  });

  it("formats sizes with a Vietnamese decimal comma", () => {
    expect(fmtBytes(512)).toBe("512 B");
    expect(fmtBytes(1536)).toBe("1,5 KB");
    expect(fmtBytes(5 * 1024 ** 2)).toBe("5,0 MB");
    expect(fmtBytes(3 * 1024 ** 4)).toBe("3072,0 GB");
  });
});
