import { describe, expect, it } from "vitest";
import { ApiError } from "@/shared/api/errors";
import { MAX_REVIEW, STAGE_REVIEW_MESSAGES, editsLabel, reviewErrorField, reviewProblem } from "./review-form";

describe("reviewProblem", () => {
  it("accepts a review once trimmed", () => {
    expect(reviewProblem("  Tốt.  ")).toBeNull();
    expect(reviewProblem("a".repeat(MAX_REVIEW))).toBeNull();
  });

  it("requires a review that is more than blanks", () => {
    expect(reviewProblem("")).toBe(STAGE_REVIEW_MESSAGES.required);
    expect(reviewProblem(" \n\t ")).toBe("Nhập nhận xét tổng kết chặng.");
  });

  it("counts characters as the API does, not UTF-16 units", () => {
    // An astral character: one rune for the API, two UTF-16 units for `String.length`.
    expect(reviewProblem("\u{1F600}".repeat(MAX_REVIEW))).toBeNull();
    expect(reviewProblem(`  ${"ệ".repeat(MAX_REVIEW + 1)}  `)).toBe("Nhận xét tối đa 5000 ký tự.");
  });
});

describe("reviewErrorField", () => {
  it("puts a validation error on the field and anything else on the form", () => {
    expect(reviewErrorField(new ApiError(422, "VALIDATION_FAILED", "Nhập nhận xét tổng kết chặng."))).toBe("body");
    expect(reviewErrorField(new ApiError(409, "CONFLICT", "Nhận xét vừa được người khác lưu."))).toBe("form");
    expect(reviewErrorField(new ApiError(403, "FORBIDDEN", "Bạn chỉ nhận xét học viên của lớp mình phụ trách."))).toBe("form");
    expect(reviewErrorField(new TypeError("Failed to fetch"))).toBe("form");
  });
});

describe("editsLabel", () => {
  it("counts the edits after the first save", () => {
    expect(editsLabel(1)).toBeNull();
    expect(editsLabel(2)).toBe("đã sửa 1 lần");
    expect(editsLabel(4)).toBe("đã sửa 3 lần");
  });
});
