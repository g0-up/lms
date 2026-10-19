import { describe, expect, it } from "vitest";
import { fixtures } from "../test/golden";
import { applyLayout, layoutOf, moveItem, moveLessonAcross, moveUnit, orderByIds, type UnitLessons } from "./order";

describe("ordering", () => {
  it("moves an item up or down", () => {
    expect(moveItem(["a", "b", "c"], 2, -1)).toEqual(["a", "c", "b"]);
    expect(moveItem(["a", "b", "c"], 0, 1)).toEqual(["b", "a", "c"]);
  });

  it("returns an unchanged copy past either end", () => {
    const items = ["a", "b"];
    const moved = moveItem(items, 0, -1);
    expect(moved).toEqual(items);
    expect(moved).not.toBe(items);
    expect(moveItem(items, 1, 1)).toEqual(items);
  });

  it("orders by ids and keeps unlisted items last", () => {
    const items = [{ id: "a" }, { id: "b" }, { id: "c" }];
    expect(orderByIds(items, ["c", "a"]).map((x) => x.id)).toEqual(["c", "a", "b"]);
  });
});

describe("lesson layout", () => {
  const layout: UnitLessons[] = [
    { unitId: "u1", lessonIds: ["a", "b", "c"] },
    { unitId: "u2", lessonIds: [] },
    { unitId: "u3", lessonIds: ["d", "e"] },
  ];

  it("swaps a lesson with its neighbour inside the unit", () => {
    expect(moveLessonAcross(layout, "b", -1)[0].lessonIds).toEqual(["b", "a", "c"]);
    expect(moveLessonAcross(layout, "d", 1)[2].lessonIds).toEqual(["e", "d"]);
  });

  it("moves the first lesson of a unit up to the end of the previous unit", () => {
    const moved = moveLessonAcross(layout, "d", -1);
    expect(moved.map((u) => u.lessonIds)).toEqual([["a", "b", "c"], ["d"], ["e"]]);
    expect(moveLessonAcross(moved, "d", -1).map((u) => u.lessonIds)).toEqual([["a", "b", "c", "d"], [], ["e"]]);
  });

  it("moves the last lesson of a unit down to the start of the next unit", () => {
    const moved = moveLessonAcross(layout, "c", 1);
    expect(moved.map((u) => u.lessonIds)).toEqual([["a", "b"], ["c"], ["d", "e"]]);
    expect(moveLessonAcross(moved, "c", 1).map((u) => u.lessonIds)).toEqual([["a", "b"], [], ["c", "d", "e"]]);
  });

  it("leaves the first lesson of the first unit and the last of the last unit in place", () => {
    expect(moveLessonAcross(layout, "a", -1)).toEqual(layout);
    expect(moveLessonAcross(layout, "e", 1)).toEqual(layout);
    expect(moveLessonAcross(layout, "missing", 1)).toEqual(layout);
  });

  it("never mutates the layout it was given", () => {
    const before = structuredClone(layout);
    moveLessonAcross(layout, "c", 1);
    moveLessonAcross(layout, "b", -1);
    expect(layout).toEqual(before);
  });

  it("moves a unit with its lessons", () => {
    expect(moveUnit(layout, "u3", -1).map((u) => u.unitId)).toEqual(["u1", "u3", "u2"]);
    expect(moveUnit(layout, "u1", -1)).toEqual(layout);
    expect(moveUnit(layout, "u3", 1)).toEqual(layout);
  });

  it("reads the layout of a version and applies a new one with renumbered positions and counts", () => {
    const version = fixtures.stageVersionDraft;
    const [unit] = version.units;
    const [first, second] = version.lessons;
    expect(layoutOf(version)).toEqual([{ unitId: unit.id, lessonIds: [first.id, second.id] }]);

    const other = { ...unit, id: "u-other", unitKey: "khac", title: "Khác", position: 2, lessonCount: 0, requiredCount: 0 };
    const twoUnits = { ...version, units: [unit, other] };
    const applied = applyLayout(twoUnits, [
      { unitId: other.id, lessonIds: [first.id] },
      { unitId: unit.id, lessonIds: [second.id] },
    ]);

    expect(applied.units.map((u) => [u.id, u.position, u.lessonCount, u.requiredCount])).toEqual([
      [other.id, 1, 1, 1],
      [unit.id, 2, 1, 0],
    ]);
    expect(applied.lessons.map((l) => [l.id, l.unitId, l.position])).toEqual([
      [first.id, other.id, 1],
      [second.id, unit.id, 2],
    ]);
  });
});
