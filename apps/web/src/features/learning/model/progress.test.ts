import { describe, expect, it } from "vitest";
import { percentOf } from "@/shared/domain";
import { fixtures } from "../test/golden";
import { formatDuration, lessonSub, stageProgress } from "./progress";
import type { RoadmapLesson } from "./schemas";

const lesson = (over: Partial<RoadmapLesson>): RoadmapLesson => ({
  id: "l",
  title: "L",
  type: "markdown",
  required: true,
  position: 1,
  ...over,
});

describe("percentOf (shared rounding used for stage bars)", () => {
  it("rounds half up to whole percents", () => {
    expect(percentOf(2, 3)).toBe(67);
    expect(percentOf(1, 3)).toBe(33);
    expect(percentOf(1, 2)).toBe(50);
    expect(percentOf(0, 0)).toBe(0);
  });
});

describe("stageProgress", () => {
  it("counts required lessons only", () => {
    const [database, dataStructure] = fixtures.roadmapActive.stages;
    // Database: video done, markdown opened, optional reading untouched.
    expect(stageProgress(database)).toEqual({ done: 1, total: 2, percent: 50 });
    expect(stageProgress(dataStructure)).toEqual({ done: 0, total: 3, percent: 0 });
  });

  it("ignores a completed optional lesson", () => {
    const stage = {
      lessons: [
        lesson({ completedAt: "2026-10-05T08:00:00Z" }),
        lesson({}),
        lesson({}),
        lesson({ required: false, completedAt: "2026-10-05T08:00:00Z" }),
      ],
    };
    expect(stageProgress(stage)).toEqual({ done: 1, total: 3, percent: 33 });
  });
});

describe("formatDuration", () => {
  it("formats seconds as mm:ss", () => {
    expect(formatDuration(1104)).toBe("18:24");
    expect(formatDuration(65)).toBe("01:05");
    expect(formatDuration(3725)).toBe("62:05");
  });
});

describe("lessonSub", () => {
  it("describes the lesson type and its markers", () => {
    expect(lessonSub(lesson({ type: "video", durationSeconds: 1104 }))).toBe("Video · 18:24");
    expect(lessonSub(lesson({ type: "video" }))).toBe("Video");
    expect(lessonSub(lesson({ required: false }))).toBe("Bài đọc · không bắt buộc");
    expect(lessonSub(lesson({ firstOpenedAt: "2026-10-05T08:00:00Z" }))).toBe("Bài đọc · đã mở");
    expect(
      lessonSub(lesson({ firstOpenedAt: "2026-10-05T08:00:00Z", completedAt: "2026-10-05T09:00:00Z" })),
    ).toBe("Bài đọc");
  });
});
