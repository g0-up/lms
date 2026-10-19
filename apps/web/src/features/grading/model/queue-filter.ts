import { queueKindSchema, type QueueKind } from "./schemas";

/**
 * Grading queue filter, the single source of both the page URL (the prototype's short names: `type`, `class`,
 * `lesson`, `overdue`) and the API query (`kind`, `classId`, `lessonId`, `overdue`). Nothing from the URL reaches
 * the API without going through `parseQueueFilter`.
 */
export interface QueueFilter {
  kind: QueueKind | null;
  classId: string | null;
  lessonId: string | null;
  overdue: boolean;
}

export const NO_FILTER: QueueFilter = { kind: null, classId: null, lessonId: null, overdue: false };

// The API answers 422 to anything but a UUID; a mangled link should just show the whole queue.
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const idOrNull = (raw: string | null | undefined) => (raw && UUID.test(raw) ? raw : null);

/** Reads the filter from the page URL; malformed values are dropped, never forwarded. */
export function parseQueueFilter(params: URLSearchParams): QueueFilter {
  const kind = queueKindSchema.safeParse(params.get("type"));
  return {
    kind: kind.success ? kind.data : null,
    classId: idOrNull(params.get("class")),
    lessonId: idOrNull(params.get("lesson")),
    overdue: params.get("overdue") === "1",
  };
}

/** Page URL parameters of `filter`, unset conditions left out. */
export function toQueueQuery(filter: QueueFilter): URLSearchParams {
  const params = new URLSearchParams();
  if (filter.kind) params.set("type", filter.kind);
  if (filter.classId) params.set("class", filter.classId);
  if (filter.lessonId) params.set("lesson", filter.lessonId);
  if (filter.overdue) params.set("overdue", "1");
  return params;
}

/** API query parameters of `filter`, unset conditions left out. */
export function toQueueApiParams(filter: QueueFilter): URLSearchParams {
  const params = new URLSearchParams();
  if (filter.kind) params.set("kind", filter.kind);
  if (filter.classId) params.set("classId", filter.classId);
  if (filter.lessonId) params.set("lessonId", filter.lessonId);
  if (filter.overdue) params.set("overdue", "1");
  return params;
}

export function hasAnyQueueFilter(filter: QueueFilter): boolean {
  return filter.kind !== null || filter.classId !== null || filter.lessonId !== null || filter.overdue;
}

/** Sentinel of the "all" option: a Radix Select item cannot have an empty value. */
export const ALL = "all";

/** Filter from the form fields; `ALL` (or anything unknown) means "no condition". */
export function queueFilterFromForm(form: { type: string; classId: string; lessonId: string; overdue: boolean }) {
  const kind = queueKindSchema.safeParse(form.type);
  return {
    kind: kind.success ? kind.data : null,
    classId: idOrNull(form.classId),
    lessonId: idOrNull(form.lessonId),
    overdue: form.overdue,
  } satisfies QueueFilter;
}
