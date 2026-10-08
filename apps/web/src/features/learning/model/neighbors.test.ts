import { describe, expect, it } from "vitest";
import { fixtures, ids } from "../test/golden";
import { firstIncomplete, flattenLessons, neighbors } from "./neighbors";

const flat = flattenLessons(fixtures.roadmapActive.stages);

describe("flattenLessons", () => {
  it("keeps stage order, then lesson order, with the stage of each lesson", () => {
    expect(flat.map((f) => f.lesson.title)).toEqual([
      "Giới thiệu SQL",
      "Thiết kế bảng và khóa",
      "Đọc thêm: chỉ mục",
      "Mảng và danh sách liên kết",
      "Stack và Queue",
      "Hash table",
    ]);
    expect(flat[3]?.stage).toEqual({ id: fixtures.roadmapActive.stages[1]?.id, name: "Data structure" });
  });
});

describe("neighbors", () => {
  it("matches the API's prev/next for every golden lesson page", () => {
    for (const page of [fixtures.lessonVideo, fixtures.lessonMarkdown, fixtures.lessonEnded]) {
      const { prev, next } = neighbors(flat, page.lesson.id);
      expect(prev?.lesson.id).toBe(page.prev?.lessonId);
      expect(next?.lesson.id).toBe(page.next?.lessonId);
    }
  });

  it("crosses stage boundaries and stops at the edges", () => {
    expect(neighbors(flat, ids.optional).next?.stage.name).toBe("Data structure");
    expect(neighbors(flat, ids.video).prev).toBeUndefined();
    expect(neighbors(flat, flat[flat.length - 1].lesson.id).next).toBeUndefined();
    expect(neighbors(flat, "missing")).toEqual({ prev: undefined, next: undefined });
  });
});

describe("firstIncomplete", () => {
  it("agrees with the API's nextLesson", () => {
    expect(firstIncomplete(flat)?.lesson.id).toBe(fixtures.roadmapActive.nextLesson?.lessonId);
  });

  it("is undefined once everything is done", () => {
    const done = flat.map((f) => ({ ...f, lesson: { ...f.lesson, completedAt: "2026-10-05T08:00:00Z" } }));
    expect(firstIncomplete(done)).toBeUndefined();
  });
});
