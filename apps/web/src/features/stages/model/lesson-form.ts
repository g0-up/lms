import { z } from "zod";
import { hasCode, isApiError, NETWORK_ERROR_MESSAGE } from "@/shared/api/errors";
import { lessonBodySizeError, markdownSourceError } from "./markdown-rules";
import type { Lesson } from "./schemas";
import { VERSION_IMMUTABLE_MESSAGE } from "./stage-rules";

export const LESSON_TITLE_REQUIRED = "Nhập tiêu đề học liệu.";
export const LESSON_VIDEO_REQUIRED = "Học liệu video bắt buộc có file video.";
export const LESSON_DURATION_FORMAT = "Nhập thời lượng dạng mm:ss, ví dụ 09:30.";

const DURATION = /^(\d{1,3}):([0-5]\d)$/;

/** "mm:ss" → seconds; `undefined` when empty or malformed. */
export function parseDuration(text: string): number | undefined {
  const match = DURATION.exec(text.trim());
  return match ? Number(match[1]) * 60 + Number(match[2]) : undefined;
}

/** Seconds → "mm:ss" (minutes keep counting past an hour: 3700 → "61:40"). */
export function fmtDuration(seconds: number | undefined): string {
  if (seconds === undefined) return "";
  const s = Math.max(0, Math.round(seconds));
  return `${String(Math.floor(s / 60)).padStart(2, "0")}:${String(s % 60).padStart(2, "0")}`;
}

const common = {
  title: z.string().trim().min(1, LESSON_TITLE_REQUIRED),
  required: z.boolean(),
};

const videoLessonSchema = z.object({
  ...common,
  type: z.literal("video"),
  videoMediaId: z.string().min(1, LESSON_VIDEO_REQUIRED),
  duration: z.string().refine((s) => s.trim() === "" || DURATION.test(s.trim()), LESSON_DURATION_FORMAT),
});

/**
 * Lesson dialog values. Both branches carry the video fields so the form keeps what was typed when
 * the type is switched; only the selected branch is validated. A markdown lesson leaves the dialog
 * with its title and flag, and its content is written on the editor page.
 */
export const lessonFormSchema = z.discriminatedUnion("type", [
  videoLessonSchema,
  z.object({ ...common, type: z.literal("markdown"), videoMediaId: z.string(), duration: z.string() }),
]);
export type LessonFormValues = z.input<typeof lessonFormSchema>;
export type LessonFormOutput = z.output<typeof lessonFormSchema>;
export type VideoLessonOutput = z.output<typeof videoLessonSchema>;

/**
 * Editor page values. The source is checked on submit, and only when it is sent: a lesson whose
 * content was not touched keeps its stored source as is.
 */
export const markdownLessonSchema = z.object({ ...common, type: z.literal("markdown"), markdownSource: z.string() });
export type MarkdownLessonValues = z.input<typeof markdownLessonSchema>;
export type MarkdownLessonOutput = z.output<typeof markdownLessonSchema>;

/** What a lesson save sends: a video from the dialog or a markdown lesson from the editor page. */
export type LessonValues = VideoLessonOutput | MarkdownLessonOutput;

/** Body of `POST /stage-versions/{vid}/lessons`; `PATCH` sends the same minus `type`. */
export type LessonBody =
  | { title: string; type: "video"; required: boolean; videoMediaId: string; durationSeconds?: number }
  | { title: string; type: "markdown"; required: boolean; markdownSource: string };

/** `PATCH` body: the type cannot change after creation, and an untouched source is not resent. */
export type LessonPatch =
  | { title: string; required: boolean; videoMediaId: string; durationSeconds?: number }
  | { title: string; required: boolean; markdownSource?: string };

export function toLessonBody(values: LessonValues): LessonBody {
  const { title, required } = values;
  if (values.type === "video") {
    return { title, type: "video", required, videoMediaId: values.videoMediaId, durationSeconds: parseDuration(values.duration) };
  }
  return { title, type: "markdown", required, markdownSource: values.markdownSource };
}

/** `keepSource` leaves the markdown out, so the server keeps what it has and skips its source checks. */
export function toLessonPatch(values: LessonValues, { keepSource = false }: { keepSource?: boolean } = {}): LessonPatch {
  const { title, required } = values;
  if (values.type === "video") {
    return { title, required, videoMediaId: values.videoMediaId, durationSeconds: parseDuration(values.duration) };
  }
  return keepSource ? { title, required } : { title, required, markdownSource: values.markdownSource };
}

/** Dialog defaults: a new required video lesson, or the video lesson being edited. */
export function lessonFormDefaults(lesson?: Lesson): LessonFormValues {
  return {
    type: "video",
    title: lesson?.title ?? "",
    required: lesson?.required ?? true,
    videoMediaId: lesson?.videoMediaId ?? "",
    duration: fmtDuration(lesson?.durationSeconds),
  };
}

/** Title and flag typed in the dialog before moving on to the editor page. */
export interface MarkdownLessonDraft {
  title: string;
  required: boolean;
}

/** Editor page defaults: the lesson being edited, else what the dialog passed on, else a blank required lesson. */
export function markdownLessonDefaults(lesson?: Lesson, draft?: MarkdownLessonDraft): MarkdownLessonValues {
  return {
    type: "markdown",
    title: lesson?.title ?? draft?.title ?? "",
    required: lesson?.required ?? draft?.required ?? true,
    markdownSource: lesson?.markdownSource ?? "",
  };
}

/** Router state is untyped: accept only a well-formed draft. */
export function readMarkdownLessonDraft(state: unknown): MarkdownLessonDraft | undefined {
  if (typeof state !== "object" || state === null) return undefined;
  const { title, required } = state as Partial<Record<keyof MarkdownLessonDraft, unknown>>;
  return typeof title === "string" && typeof required === "boolean" ? { title, required } : undefined;
}

/** The editor page of a new markdown lesson, or of an existing one. */
export function lessonEditorPath(stageId: string, vid: string, lessonId?: string): string {
  const base = `/admin/stages/${stageId}/versions/${vid}/lessons`;
  return lessonId ? `${base}/${lessonId}/edit` : `${base}/new`;
}

/** What the editor page sends, or why it cannot: on the content field, as a violation list, or as a size error. */
export type MarkdownSaveCheck =
  | { kind: "ok"; values: MarkdownLessonOutput; keepSource: boolean }
  | { kind: "source"; message: string }
  | { kind: "violations"; violations: string[] }
  | { kind: "size"; message: string };

export interface MarkdownSaveContext {
  /** Absent when adding. */
  lessonId?: string;
  /** Whether the editor reported an edit; only then is the source sent and checked. */
  contentTouched: boolean;
  /** The editor's markdown at submit time. */
  source: string;
  /** Images and links the editor tree holds that the server would refuse. */
  violations: readonly string[];
}

/**
 * Checks a markdown lesson save. Untouched content of an existing lesson is left out of the body, so
 * a title change saves even when old content breaks today's rules; anything sent is checked first.
 */
export function checkMarkdownSave(values: MarkdownLessonOutput, ctx: MarkdownSaveContext): MarkdownSaveCheck {
  const keepSource = Boolean(ctx.lessonId) && !ctx.contentTouched;
  const sent = keepSource ? values : { ...values, markdownSource: ctx.source };
  if (!keepSource) {
    const sourceError = markdownSourceError(sent.markdownSource);
    if (sourceError) return { kind: "source", message: sourceError };
    if (ctx.violations.length > 0) return { kind: "violations", violations: [...ctx.violations] };
  }
  const sizeError = lessonBodySizeError(ctx.lessonId ? toLessonPatch(sent, { keepSource }) : toLessonBody(sent));
  if (sizeError) return { kind: "size", message: sizeError };
  return { kind: "ok", values: sent, keepSource };
}

export const LESSON_VIOLATIONS_SHOWN = 5;

/** The first offending image sources or link targets, and how many more there are. */
export function summarizeViolations(violations: readonly string[]): { shown: string[]; more: number } {
  return {
    shown: violations.slice(0, LESSON_VIOLATIONS_SHOWN),
    more: Math.max(0, violations.length - LESSON_VIOLATIONS_SHOWN),
  };
}

/** Message for a failed lesson save or preview: the body-size detail of a 400 when there is one, else the server's text. */
export function lessonErrorMessage(error: unknown): string {
  if (hasCode(error, "VERSION_IMMUTABLE")) return VERSION_IMMUTABLE_MESSAGE;
  if (!isApiError(error)) return NETWORK_ERROR_MESSAGE;
  const details = error.details;
  if (typeof details === "object" && details !== null && "body" in details && typeof details.body === "string") {
    return details.body;
  }
  return error.message;
}

/** Lesson row meta: "Video · {file} · {mm:ss}" or "Markdown · {n} ký tự". */
export function lessonMeta(lesson: Lesson): string {
  if (lesson.type === "video") {
    return ["Video", lesson.videoFileName, fmtDuration(lesson.durationSeconds)].filter(Boolean).join(" · ");
  }
  return `Markdown · ${String(lesson.markdownSource?.length ?? 0)} ký tự`;
}

/** "{n} bắt buộc · {m} tùy chọn". */
export function requiredSplitLabel(lessons: readonly Pick<Lesson, "required">[]): string {
  const required = lessons.filter((l) => l.required).length;
  return `${String(required)} bắt buộc · ${String(lessons.length - required)} tùy chọn`;
}
