declare const percentBrand: unique symbol;

/** Whole number in [0, 100]; only `percentOf` and `toPercent` create one. */
export type Percent = number & { readonly [percentBrand]: true };

export function toPercent(value: number): Percent {
  if (!Number.isFinite(value)) return 0 as Percent;
  return Math.min(100, Math.max(0, Math.round(value))) as Percent;
}

/** Rounded share of `done` over `total`; an empty total counts as 0%. */
export function percentOf(done: number, total: number): Percent {
  if (total <= 0) return 0 as Percent;
  return toPercent((done / total) * 100);
}
