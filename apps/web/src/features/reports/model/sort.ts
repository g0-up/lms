import type { ReportSort } from "./schemas";

export const SORT_OPTIONS: readonly { value: ReportSort; label: string }[] = [
  { value: "name", label: "Tên A → Z" },
  { value: "pct", label: "% toàn khóa thấp → cao" },
  { value: "pct-desc", label: "% toàn khóa cao → thấp" },
  { value: "activity", label: "Hoạt động mới nhất" },
  { value: "activity-asc", label: "Lâu không hoạt động" },
];

export type SortColumn = "pct" | "activity";

/** Header click: the first click picks the column's default order, the next one flips it. */
export function toggleSort(current: ReportSort, column: SortColumn): ReportSort {
  if (column === "pct") return current === "pct" ? "pct-desc" : "pct";
  return current === "activity" ? "activity-asc" : "activity";
}

/** `aria-sort` of a column header; undefined when the table is not sorted by it. */
export function ariaSort(current: ReportSort, column: SortColumn): "ascending" | "descending" | undefined {
  if (column === "pct") {
    if (current === "pct") return "ascending";
    if (current === "pct-desc") return "descending";
    return undefined;
  }
  // "Hoạt động mới nhất" lists the latest activity first: descending by time.
  if (current === "activity") return "descending";
  if (current === "activity-asc") return "ascending";
  return undefined;
}
