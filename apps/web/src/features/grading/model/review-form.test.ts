import { describe, expect, it } from "vitest";
import { ApiError } from "@/shared/api/errors";
import { fixtures } from "../test/golden";
import {
  REVIEW_MESSAGES,
  reviewDefaults,
  reviewErrorField,
  reviewFormCopy,
  reviewFormSchema,
  reviewedMessage,
  type ReviewFormInput,
} from "./review-form";

const detail = fixtures.reviewPending;
const questionIds = detail.questions.map((q) => q.id);
const [q733, q732, q731] = questionIds;
const schema = reviewFormSchema(questionIds);

const input = (comment: string, notes: [string, string, string] = ["", "", ""]): ReviewFormInput => ({
  comment,
  notes: notes.map((text) => ({ text })),
});

const errorsOf = (value: ReviewFormInput) => {
  const result = schema.safeParse(value);
  return result.success ? {} : Object.fromEntries(result.error.issues.map((i) => [i.path.join("."), i.message]));
};

describe("review form defaults", () => {
  it("starts blank before the first review", () => {
    expect(reviewDefaults(detail, null)).toEqual(input(""));
  });

  it("prefills the current review, each note under its question", () => {
    const current = fixtures.reviewConfirmed.reviews[0];

    expect(reviewDefaults(fixtures.reviewConfirmed, current)).toEqual(
      input("Đạt ngưỡng. Lượt sau chú ý câu về chỉ mục kết hợp.", [
        "Tiền tố trái quyết định chỉ mục có dùng được không.",
        "",
        "",
      ]),
    );
  });
});

describe("review form validation", () => {
  it("asks for the general comment with the API's words", () => {
    expect(errorsOf(input("   "))).toEqual({ comment: "Nhập nhận xét chung cho học viên." });
  });

  it("caps the comment at 5000 and each note at 1000 characters, counted as the API does", () => {
    // An emoji is one character for the API, two UTF-16 units for JavaScript.
    const emoji = String.fromCodePoint(0x1f600);
    expect(errorsOf(input(emoji.repeat(5000), [emoji.repeat(1000), "", ""]))).toEqual({});
    expect(errorsOf(input("a".repeat(5001), ["", "a".repeat(1001), ""]))).toEqual({
      comment: REVIEW_MESSAGES.commentTooLong,
      "notes.1.text": "Ghi chú từng câu tối đa 1000 ký tự.",
    });
  });

  it("sends the trimmed comment and only the notes that were written, in question order", () => {
    expect(schema.parse(input("  Tốt.  ", ["", "  Xem lại INSERT. ", " "]))).toEqual({
      comment: "Tốt.",
      notes: [{ questionId: q732, note: "Xem lại INSERT." }],
    });
    expect(schema.parse(input("Tốt.", ["a", "", "c"])).notes).toEqual([
      { questionId: q733, note: "a" },
      { questionId: q731, note: "c" },
    ]);
  });
});

describe("review API errors", () => {
  it("puts the comment checks on the comment field", () => {
    expect(reviewErrorField(new ApiError(400, "VALIDATION_FAILED", REVIEW_MESSAGES.commentRequired))).toBe("comment");
    expect(reviewErrorField(new ApiError(400, "VALIDATION_FAILED", "Nhận xét tối đa 5000 ký tự."))).toBe("comment");
  });

  it("puts every other refusal on the form", () => {
    expect(reviewErrorField(new ApiError(400, "VALIDATION_FAILED", "Mỗi câu chỉ có một ghi chú."))).toBe("form");
    expect(
      reviewErrorField(new ApiError(409, "CONFLICT", "Học viên đã nộp lượt mới hơn. Hãy xác nhận lượt mới nhất.")),
    ).toBe("form");
    expect(reviewErrorField(new TypeError("Failed to fetch"))).toBe("form");
  });
});

describe("review form copy", () => {
  it("names the action by quiz mode and by whether a review exists", () => {
    expect(reviewFormCopy("assessment", false)).toEqual({ title: "Xác nhận kết quả", submit: "Xác nhận" });
    expect(reviewFormCopy("practice", false)).toEqual({ title: "Nhận xét cho học viên", submit: "Gửi nhận xét" });
    expect(reviewFormCopy("assessment", true)).toEqual({ title: "Sửa nhận xét", submit: "Lưu nhận xét mới" });
    expect(reviewFormCopy("practice", true)).toEqual({ title: "Sửa nhận xét", submit: "Lưu nhận xét mới" });
  });

  it("tells what the student receives", () => {
    expect(reviewedMessage("assessment", { updated: false })).toBe(
      "Đã xác nhận kết quả. Email nhận xét đã gửi cho học viên.",
    );
    expect(reviewedMessage("practice", { updated: false })).toBe("Đã gửi nhận xét cho học viên.");
    expect(reviewedMessage("practice", { updated: true })).toBe("Đã lưu nhận xét mới. Học viên nhận email cập nhật.");
  });
});
