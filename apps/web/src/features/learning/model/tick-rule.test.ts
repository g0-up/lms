import { describe, expect, it } from "vitest";
import { tickBlocker, tickable } from "./tick-rule";

describe("tickBlocker", () => {
  it("allows ticking an opened lesson of an active class", () => {
    expect(tickBlocker({ opened: true })).toBeNull();
    expect(tickBlocker({ readOnlyReason: null, opened: true })).toBeNull();
  });

  it("asks to open the lesson first", () => {
    expect(tickBlocker({ opened: false })).toBe("Mở học liệu trước khi tích");
  });

  it("puts the class state before the open rule", () => {
    expect(tickBlocker({ readOnlyReason: "ended", opened: false })).toBe("Lớp đã kết thúc");
    expect(tickBlocker({ readOnlyReason: "ended", opened: true })).toBe("Lớp đã kết thúc");
    expect(tickBlocker({ readOnlyReason: "draft", opened: false })).toBe("Lớp chưa bắt đầu");
  });
});

describe("tickable", () => {
  it("lets the student tick videos and readings", () => {
    expect(tickable("video")).toBe(true);
    expect(tickable("markdown")).toBe(true);
  });

  it("leaves quizzes and homework to their result", () => {
    expect(tickable("quiz")).toBe(false);
    expect(tickable("homework")).toBe(false);
  });
});
