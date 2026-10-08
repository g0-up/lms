import { describe, expect, it } from "vitest";
import { fixtures } from "../test/golden";
import { draftExistsMessage, PUBLISH_NEEDS_LESSON, stageDeleteBlocker, stagePublishBlocker } from "./stage-rules";

describe("stage rules", () => {
  it("blocks publishing an empty draft only", () => {
    expect(stagePublishBlocker({ ...fixtures.stageVersionDraft, lessons: [] })).toBe(PUBLISH_NEEDS_LESSON);
    expect(stagePublishBlocker(fixtures.stageVersionDraft)).toBeNull();
  });

  it("blocks deleting a referenced published version with the courses named", () => {
    expect(stageDeleteBlocker(fixtures.stageVersion)).toBe(
      "Đang được dùng trong Lập trình cơ bản v1. Hãy lưu trữ thay vì xóa.",
    );
    expect(stageDeleteBlocker({ ...fixtures.stageVersion, usedBy: [] })).toBeNull();
    expect(stageDeleteBlocker(fixtures.stageVersionDraft)).toBeNull();
  });

  it("names the existing draft", () => {
    expect(draftExistsMessage(2)).toBe("Chặng đã có bản nháp v2. Mở bản nháp đó để sửa.");
  });
});
