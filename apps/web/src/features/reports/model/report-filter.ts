import { z } from "zod";
import { sortSchema, type ReportSort } from "./schemas";

/**
 * Report filter, the single source of both the page URL (short names, shareable) and the API
 * query (API names). Nothing from the URL reaches the API without going through `parseFilter`.
 */
export interface ReportFilter {
  notLoggedIn: boolean;
  inactiveDays: number | null;
  belowPercent: number | null;
  includeDropped: boolean;
  sort: ReportSort;
}

export const DEFAULT_FILTER: ReportFilter = {
  notLoggedIn: false,
  inactiveDays: null,
  belowPercent: null,
  includeDropped: false,
  sort: "name",
};

/** URL parameter names, as in the prototype. */
const URL_KEYS = ["notlogged", "inactive", "below", "dropped", "sort"] as const;

const inactiveDays = z.coerce.number().int().min(1).max(3650);
const belowPercent = z.coerce.number().int().min(1).max(100);

function parseOr<T>(schema: z.ZodType<T>, raw: string | null): T | null {
  if (raw === null || raw.trim() === "") return null;
  const parsed = schema.safeParse(raw);
  return parsed.success ? parsed.data : null;
}

/** Reads the filter from the page URL; malformed values are dropped, never forwarded. */
export function parseFilter(params: URLSearchParams): ReportFilter {
  const sort = sortSchema.safeParse(params.get("sort"));
  return {
    notLoggedIn: params.get("notlogged") === "1",
    inactiveDays: parseOr(inactiveDays, params.get("inactive")),
    belowPercent: parseOr(belowPercent, params.get("below")),
    includeDropped: params.get("dropped") === "1",
    sort: sort.success ? sort.data : DEFAULT_FILTER.sort,
  };
}

/** URL parameters for `filter` (short names), defaults left out. */
export function toQuery(filter: ReportFilter): URLSearchParams {
  const params = new URLSearchParams();
  if (filter.notLoggedIn) params.set("notlogged", "1");
  if (filter.inactiveDays !== null) params.set("inactive", String(filter.inactiveDays));
  if (filter.belowPercent !== null) params.set("below", String(filter.belowPercent));
  if (filter.includeDropped) params.set("dropped", "1");
  if (filter.sort !== DEFAULT_FILTER.sort) params.set("sort", filter.sort);
  return params;
}

/** `params` with its filter replaced by `filter`; other parameters (e.g. `tab`) are kept. */
export function withFilter(params: URLSearchParams, filter: ReportFilter): URLSearchParams {
  const next = new URLSearchParams(params);
  for (const key of URL_KEYS) next.delete(key);
  for (const [key, value] of toQuery(filter)) next.set(key, value);
  return next;
}

/** API query parameters for `filter` (API names), defaults left out. */
export function toApiParams(filter: ReportFilter): URLSearchParams {
  const params = new URLSearchParams();
  if (filter.notLoggedIn) params.set("notLoggedIn", "true");
  if (filter.inactiveDays !== null) params.set("inactiveDays", String(filter.inactiveDays));
  if (filter.belowPercent !== null) params.set("belowPercent", String(filter.belowPercent));
  if (filter.includeDropped) params.set("includeDropped", "true");
  if (filter.sort !== DEFAULT_FILTER.sort) params.set("sort", filter.sort);
  return params;
}

/** Whether a row-narrowing condition is set; sort and the dropped toggle do not narrow. */
export function hasAnyFilter(filter: ReportFilter): boolean {
  return filter.notLoggedIn || filter.inactiveDays !== null || filter.belowPercent !== null;
}

/** Filter for the form fields: blank or out-of-range numbers mean "no condition". */
export function filterFromForm(form: {
  notLoggedIn: boolean;
  inactive: string;
  below: string;
  includeDropped: boolean;
  sort: string;
}): ReportFilter {
  const sort = sortSchema.safeParse(form.sort);
  return {
    notLoggedIn: form.notLoggedIn,
    inactiveDays: parseOr(inactiveDays, form.inactive),
    belowPercent: parseOr(belowPercent, form.below),
    includeDropped: form.includeDropped,
    sort: sort.success ? sort.data : DEFAULT_FILTER.sort,
  };
}
