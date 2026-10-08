import { z } from "zod";
import { NETWORK_ERROR_MESSAGE, hasCode, isApiError } from "@/shared/api/errors";
import { STATUS_VI } from "@/shared/domain";
import type { CourseList } from "./schemas";

export const DATES_REQUIRED = "Nhập ngày bắt đầu và kết thúc dự kiến.";
export const END_BEFORE_START = "Ngày kết thúc phải sau ngày bắt đầu.";
export const TEACHER_REQUIRED = "Chọn giảng viên phụ trách.";
export const VERSION_REQUIRED = "Chọn một phiên bản khóa học đã phát hành.";
export const CODE_TAKEN = "Mã lớp đã tồn tại.";
export const NO_PUBLISHED_VERSION = "Chưa có phiên bản khóa học nào được phát hành.";

const calendarDay = /^\d{4}-\d{2}-\d{2}$/;

/** Planned dates rule shared by create and settings; ISO days compare as strings. */
export function datesError(startDate: string, endDate: string): { field: "startDate" | "endDate"; message: string } | null {
  if (!calendarDay.test(startDate)) return { field: "startDate", message: DATES_REQUIRED };
  if (!calendarDay.test(endDate)) return { field: "endDate", message: DATES_REQUIRED };
  if (endDate <= startDate) return { field: "endDate", message: END_BEFORE_START };
  return null;
}

const planFields = {
  name: z.string().trim(),
  courseVersionId: z.string(),
  teacherId: z.string(),
  startDate: z.string(),
  endDate: z.string(),
};

function checkPlan(
  v: { courseVersionId: string; teacherId: string; startDate: string; endDate: string },
  ctx: z.RefinementCtx,
) {
  if (!v.courseVersionId) ctx.addIssue({ code: "custom", path: ["courseVersionId"], message: VERSION_REQUIRED });
  const dates = datesError(v.startDate, v.endDate);
  if (dates) ctx.addIssue({ code: "custom", path: [dates.field], message: dates.message });
  if (!v.teacherId) ctx.addIssue({ code: "custom", path: ["teacherId"], message: TEACHER_REQUIRED });
}

/** New class form; the class code is only trimmed, its format is the API's to judge. */
export const classFormSchema = z.object({ code: z.string().trim(), ...planFields }).superRefine((v, ctx) => {
  if (!v.code || !v.name) {
    ctx.addIssue({ code: "custom", path: [v.code ? "name" : "code"], message: "Nhập mã và tên lớp." });
  }
  checkPlan(v, ctx);
});
export type ClassFormInput = z.input<typeof classFormSchema>;
export type ClassFormValues = z.output<typeof classFormSchema>;

/** Settings form: everything but the code, which never changes. */
export const settingsFormSchema = z.object(planFields).superRefine((v, ctx) => {
  if (!v.name) ctx.addIssue({ code: "custom", path: ["name"], message: "Nhập tên lớp." });
  checkPlan(v, ctx);
});
export type SettingsFormInput = z.input<typeof settingsFormSchema>;
export type SettingsFormValues = z.output<typeof settingsFormSchema>;

const pad = (n: number) => String(n).padStart(2, "0");

/** Local calendar day of `date` as "yyyy-mm-dd". */
export function toDay(date: Date): string {
  return `${String(date.getFullYear())}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

/** Prototype defaults for a new class: starts in two weeks, ends 120 days from today. */
export function defaultDates(today: Date = new Date()): { startDate: string; endDate: string } {
  const plus = (days: number) => new Date(today.getFullYear(), today.getMonth(), today.getDate() + days);
  return { startDate: toDay(plus(14)), endDate: toDay(plus(120)) };
}

/** Fields of `values` that differ from `initial`: the PATCH sends nothing else. */
export function changedFields<T extends Record<string, string>>(initial: T, values: T): Partial<T> {
  const out: Partial<T> = {};
  for (const key of Object.keys(values) as (keyof T)[]) {
    if (values[key] !== initial[key]) out[key] = values[key];
  }
  return out;
}

export interface VersionOption {
  id: string;
  label: string;
  publishedAt: string | null;
  versionNo: number;
}

/** Published course versions for the select, newest first within each course. */
export function publishedVersions(courses: CourseList): VersionOption[] {
  return courses.items
    .flatMap((course) =>
      course.versions
        .filter((v) => v.status === "published")
        .map((v) => ({
          id: v.id,
          label: `${course.name} v${String(v.versionNo)}`,
          publishedAt: v.publishedAt ?? null,
          versionNo: v.versionNo,
          course: course.name,
        })),
    )
    .sort((a, b) => a.course.localeCompare(b.course, "vi") || b.versionNo - a.versionNo)
    .map(({ course: _course, ...option }) => option);
}

/** The latest published version: the most recent `publishedAt`, else the highest version number. */
export function defaultVersionId(options: readonly VersionOption[]): string {
  let best: VersionOption | undefined;
  for (const option of options) {
    if (!best) {
      best = option;
      continue;
    }
    const a = option.publishedAt ?? "";
    const b = best.publishedAt ?? "";
    if (a > b || (a === b && option.versionNo > best.versionNo)) best = option;
  }
  return best?.id ?? "";
}

/**
 * Settings select: the published versions plus the class's current one, which may since have been
 * archived; a non-published current version says so in its label.
 */
export function settingsVersionOptions(
  courses: CourseList | undefined,
  current: { id: string; courseName: string; versionNo: number },
): VersionOption[] {
  const published = courses ? publishedVersions(courses) : [];
  if (published.some((o) => o.id === current.id)) return published;
  const status = courses?.items.flatMap((c) => c.versions).find((v) => v.id === current.id)?.status;
  const suffix = status && status !== "published" ? ` (${STATUS_VI[status]})` : "";
  return [
    { id: current.id, label: `${current.courseName} v${String(current.versionNo)}${suffix}`, publishedAt: null, versionNo: current.versionNo },
    ...published,
  ];
}

/** Where a refused create or save is shown: a taken code on its field, anything else in the form alert. */
export function classFormError(error: unknown): { field: "code" | null; message: string } {
  if (hasCode(error, "CONFLICT")) return { field: "code", message: CODE_TAKEN };
  return { field: null, message: isApiError(error) ? error.message : NETWORK_ERROR_MESSAGE };
}
