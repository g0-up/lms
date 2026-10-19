import { describe, expect, it } from "vitest";
import { isPassing, passPoints, shortTitle } from "./index";

describe("passPoints / isPassing", () => {
  it("rounds the least passing total up, as the API decides", () => {
    expect(passPoints(100, 70)).toBe(70);
    expect(passPoints(15, 70)).toBe(11);
    expect(passPoints(10, 0)).toBe(0);
  });

  it("passes exactly at the threshold and not one point below", () => {
    expect(isPassing(70, 100, 70)).toBe(true);
    expect(isPassing(69, 100, 70)).toBe(false);
    expect(isPassing(11, 15, 70)).toBe(true);
    expect(isPassing(10, 15, 70)).toBe(false);
    expect(isPassing(passPoints(15, 70), 15, 70)).toBe(true);
  });
});

describe("shortTitle", () => {
  it("drops the kind prefix only", () => {
    expect(shortTitle("Bài tập: truy vấn SELECT")).toBe("truy vấn SELECT");
    expect(shortTitle("Kiểm tra:  JOIN")).toBe("JOIN");
    expect(shortTitle("Luyện tập: GROUP BY")).toBe("GROUP BY");
    expect(shortTitle("Thiết kế bảng: Bài tập")).toBe("Thiết kế bảng: Bài tập");
  });
});
