import { describe, expect, it } from "vitest";
import { tickBlocker } from "./tick-rule";

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
