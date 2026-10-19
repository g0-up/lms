import type { Lesson, StageVersion, Unit } from "./schemas";

/** Copy of `items` with the item at `index` moved by `delta` places; unchanged copy when out of range. */
export function moveItem<T>(items: readonly T[], index: number, delta: number): T[] {
  const next = [...items];
  const target = index + delta;
  if (index < 0 || index >= items.length || target < 0 || target >= items.length) return next;
  const [item] = next.splice(index, 1);
  next.splice(target, 0, item);
  return next;
}

/** `items` in the order of `ids`; items whose id is not listed keep their order after the listed ones. */
export function orderByIds<T extends { id: string }>(items: readonly T[], ids: readonly string[]): T[] {
  const rank = new Map(ids.map((id, i) => [id, i]));
  return [...items].sort((a, b) => (rank.get(a.id) ?? ids.length) - (rank.get(b.id) ?? ids.length));
}

/** One bài học and its lessons in order: the shape `PUT lessons/order` takes for every unit. */
export interface UnitLessons {
  unitId: string;
  lessonIds: string[];
}

/** The version's units in order, each with its lessons in order. */
export function layoutOf(version: Pick<StageVersion, "units" | "lessons">): UnitLessons[] {
  return version.units.map((u) => ({
    unitId: u.id,
    lessonIds: version.lessons.filter((l) => l.unitId === u.id).map((l) => l.id),
  }));
}

/**
 * Copy of `layout` with the lesson moved one place. Inside a unit it swaps with its neighbour; past
 * the unit's edge it goes to the end of the previous unit or the start of the next, even an empty
 * one. Unchanged copy at the very first or last place, or for an unknown lesson.
 */
export function moveLessonAcross(layout: readonly UnitLessons[], lessonId: string, dir: -1 | 1): UnitLessons[] {
  const next = layout.map((u) => ({ unitId: u.unitId, lessonIds: [...u.lessonIds] }));
  const from = next.findIndex((u) => u.lessonIds.includes(lessonId));
  if (from < 0) return next;
  const ids = next[from].lessonIds;
  const at = ids.indexOf(lessonId);
  if (at + dir >= 0 && at + dir < ids.length) {
    next[from].lessonIds = moveItem(ids, at, dir);
    return next;
  }
  if (from + dir < 0 || from + dir >= next.length) return next;
  const to = next[from + dir];
  ids.splice(at, 1);
  if (dir < 0) to.lessonIds.push(lessonId);
  else to.lessonIds.unshift(lessonId);
  return next;
}

/** Copy of `layout` with the unit moved one place; unchanged copy at either end or for an unknown unit. */
export function moveUnit(layout: readonly UnitLessons[], unitId: string, dir: -1 | 1): UnitLessons[] {
  return moveItem(layout, layout.findIndex((u) => u.unitId === unitId), dir);
}

/**
 * `version` rearranged to `layout`, as the server would answer the reorder: units and lessons in the
 * new order, lessons tagged with their unit, flat positions renumbered and unit counts recounted.
 */
export function applyLayout(version: StageVersion, layout: readonly UnitLessons[]): StageVersion {
  const unitById = new Map(version.units.map((u) => [u.id, u]));
  const lessonById = new Map(version.lessons.map((l) => [l.id, l]));
  const units: Unit[] = [];
  const lessons: Lesson[] = [];
  for (const { unitId, lessonIds } of layout) {
    const unit = unitById.get(unitId);
    if (!unit) continue;
    const own = lessonIds.flatMap((id) => lessonById.get(id) ?? []);
    units.push({
      ...unit,
      position: units.length + 1,
      lessonCount: own.length,
      requiredCount: own.filter((l) => l.required).length,
    });
    for (const l of own) lessons.push({ ...l, unitId, position: lessons.length + 1 });
  }
  return { ...version, units, lessons };
}
