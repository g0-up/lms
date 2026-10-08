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
