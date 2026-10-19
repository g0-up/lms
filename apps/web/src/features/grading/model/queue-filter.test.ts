import { describe, expect, it } from "vitest";
import {
  ALL,
  NO_FILTER,
  hasAnyQueueFilter,
  parseQueueFilter,
  queueFilterFromForm,
  toQueueApiParams,
  toQueueQuery,
} from "./queue-filter";

const classId = "01990000-0000-7000-8000-000000000301";
const lessonId = "01990000-0000-7000-8000-000000000928";

describe("queue filter", () => {
  it("reads the prototype's URL parameters", () => {
    const filter = parseQueueFilter(new URLSearchParams(`type=homework&class=${classId}&lesson=${lessonId}&overdue=1`));

    expect(filter).toEqual({ kind: "homework", classId, lessonId, overdue: true });
    expect(hasAnyQueueFilter(filter)).toBe(true);
  });

  it("drops what the API would refuse instead of forwarding it", () => {
    const filter = parseQueueFilter(new URLSearchParams("type=video&class=basic01&lesson=1%20OR%201=1&overdue=true"));

    expect(filter).toEqual(NO_FILTER);
    expect(hasAnyQueueFilter(filter)).toBe(false);
    expect(toQueueApiParams(filter).toString()).toBe("");
  });

  it("sends the API its own parameter names", () => {
    const params = toQueueApiParams({ kind: "quiz", classId, lessonId, overdue: true });

    expect(Object.fromEntries(params)).toEqual({ kind: "quiz", classId, lessonId, overdue: "1" });
  });

  it("writes back the URL it reads, unset conditions left out", () => {
    const filter = { ...NO_FILTER, classId, overdue: true };
    const query = toQueueQuery(filter);

    expect(query.toString()).toBe(`class=${classId}&overdue=1`);
    expect(parseQueueFilter(query)).toEqual(filter);
    expect(toQueueQuery(NO_FILTER).toString()).toBe("");
  });

  it("treats the form's 'all' options as no condition", () => {
    expect(queueFilterFromForm({ type: ALL, classId: ALL, lessonId: ALL, overdue: false })).toEqual(NO_FILTER);
    expect(queueFilterFromForm({ type: "quiz", classId, lessonId: ALL, overdue: true })).toEqual({
      kind: "quiz",
      classId,
      lessonId: null,
      overdue: true,
    });
  });
});
