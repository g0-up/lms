import { z } from "zod";
import { hasCode, isApiError, NETWORK_ERROR_MESSAGE } from "@/shared/api/errors";
import { lessonBodySizeError, markdownSourceError } from "./markdown-rules";
import { quizTotalPoints } from "./question-form";
import type { Lesson, LessonType, QuizMode } from "./schemas";
import { VERSION_IMMUTABLE_MESSAGE } from "./stage-rules";

export const LESSON_TITLE_REQUIRED = "Nhập tiêu đề học liệu.";
export const LESSON_VIDEO_REQUIRED = "Học liệu video bắt buộc có file video.";
export const LESSON_DURATION_FORMAT = "Nhập thời lượng dạng mm:ss, ví dụ 09:30.";
export const LESSON_UNIT_REQUIRED = "Chọn bài học cho học liệu.";
export const LESSON_UNIT_HELP_CREATE = "Học liệu mới nằm cuối bài học đã chọn.";
export const LESSON_UNIT_HELP_EDIT = "Đổi bài học sẽ đưa học liệu xuống cuối bài học mới.";

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

export const QUIZ_PASS_PERCENT_RANGE = "Ngưỡng đạt là số nguyên từ 1 đến 100%.";
export const QUIZ_ATTEMPTS_RANGE = "Số lượt làm bài từ 1 đến 10.";
export const QUIZ_TIME_LIMIT_RANGE = "Thời gian làm bài từ 1 đến 180 phút.";

/** The dialog's "Loại": the API's lesson type, with the quiz mode folded in. */
export type LessonKind = "video" | "markdown" | "quiz:practice" | "quiz:assessment" | "homework";

/** "Loại" options, worded as in the prototype. */
export const LESSON_KINDS: readonly { value: LessonKind; label: string }[] = [
  { value: "video", label: "Video" },
  { value: "markdown", label: "Bài đọc (Markdown)" },
  { value: "quiz:practice", label: "Quiz học liệu (luyện tập, chấm tự động)" },
  { value: "quiz:assessment", label: "Quiz bài tập (giới hạn lượt và giờ, giảng viên xác nhận)" },
  { value: "homework", label: "Bài tập nộp repo hoặc file (giảng viên chấm)" },
];

export const LESSON_KIND_HELP_CREATE =
  "Quiz học liệu tính hoàn thành khi nộp một lượt. Quiz bài tập tính khi đạt ngưỡng.";
export const LESSON_KIND_HELP_EDIT =
  "Đổi giữa hai loại quiz giữ nguyên câu hỏi. Đổi sang loại khác sẽ bỏ nội dung của loại cũ.";

/** Whether a lesson is written on the quiz/homework page rather than in the dialog or the markdown editor. */
export function isGraded(lesson: Pick<Lesson, "type">): boolean {
  return lesson.type === "quiz" || lesson.type === "homework";
}

/** The kind a saved lesson shows in "Loại". */
export function lessonKindOf(lesson: Pick<Lesson, "type" | "quiz">): LessonKind {
  if (lesson.type === "quiz") return lesson.quiz?.mode === "assessment" ? "quiz:assessment" : "quiz:practice";
  return lesson.type;
}

/** The API lesson type behind a kind. */
export function lessonTypeOf(kind: LessonKind): LessonType {
  return kind === "quiz:practice" || kind === "quiz:assessment" ? "quiz" : kind;
}

/**
 * Whether saving `kind` over a lesson of `from` throws its content away: switching between the two
 * quiz kinds keeps the questions, any other change of type starts the content over.
 */
export function dropsContent(from: LessonType, kind: LessonKind): boolean {
  return lessonTypeOf(kind) !== from;
}

/** An integer typed in a text field, within `[min, max]`. */
const intText = (min: number, max: number, message: string) =>
  z
    .string()
    .trim()
    .refine((s) => /^\d+$/.test(s) && Number(s) >= min && Number(s) <= max, message)
    .transform(Number);

const common = {
  title: z.string().trim().min(1, LESSON_TITLE_REQUIRED),
  unitId: z.string().min(1, LESSON_UNIT_REQUIRED),
  required: z.boolean(),
};

/**
 * Every field of every kind, unchecked. Each branch carries all of them so the form keeps what was
 * typed when the kind is switched; only the selected branch's own fields are validated.
 */
const loose = {
  ...common,
  videoMediaId: z.string(),
  duration: z.string(),
  markdownSource: z.string(),
  shuffle: z.boolean(),
  quizPassPercent: z.string(),
  maxAttempts: z.string(),
  timeLimitMinutes: z.string(),
  homeworkPassPercent: z.string(),
};

const videoLessonSchema = z.object({
  ...loose,
  kind: z.literal("video"),
  videoMediaId: z.string().min(1, LESSON_VIDEO_REQUIRED),
  duration: z.string().refine((s) => s.trim() === "" || DURATION.test(s.trim()), LESSON_DURATION_FORMAT),
});

/**
 * Lesson dialog values. A new markdown lesson leaves the dialog with its title and flag and its
 * content is written on the editor page; a lesson switched to markdown brings its content here, checked
 * by the dialog because only that case sends it.
 */
export const lessonFormSchema = z.discriminatedUnion("kind", [
  videoLessonSchema,
  z.object({ ...loose, kind: z.literal("markdown") }),
  z.object({ ...loose, kind: z.literal("quiz:practice") }),
  z.object({
    ...loose,
    kind: z.literal("quiz:assessment"),
    quizPassPercent: intText(1, 100, QUIZ_PASS_PERCENT_RANGE),
    maxAttempts: intText(1, 10, QUIZ_ATTEMPTS_RANGE),
    timeLimitMinutes: intText(1, 180, QUIZ_TIME_LIMIT_RANGE),
  }),
  z.object({ ...loose, kind: z.literal("homework"), homeworkPassPercent: intText(1, 100, QUIZ_PASS_PERCENT_RANGE) }),
]);
export type LessonFormValues = z.input<typeof lessonFormSchema>;
export type LessonFormOutput = z.output<typeof lessonFormSchema>;

/**
 * Editor page values. The source is checked on submit, and only when it is sent: a lesson whose
 * content was not touched keeps its stored source as is.
 */
export const markdownLessonSchema = z.object({ ...common, kind: z.literal("markdown"), markdownSource: z.string() });
export type MarkdownLessonValues = z.input<typeof markdownLessonSchema>;
export type MarkdownLessonOutput = z.output<typeof markdownLessonSchema>;

/** What a lesson save sends: the dialog's values, or a markdown lesson from the editor page. */
export type LessonValues = LessonFormOutput | MarkdownLessonOutput;

/** `quiz` of a lesson body: a practice quiz has no limits, so only an assessment sends them. */
export interface QuizSettingsBody {
  mode: QuizMode;
  shuffle: boolean;
  passPercent?: number;
  maxAttempts?: number;
  timeLimitMinutes?: number;
}

/** `homework` of a lesson body; an absent field keeps what the server has, `attachmentMediaIds: []` removes every file. */
export interface HomeworkSettingsBody {
  briefSource?: string;
  passPercent?: number;
  attachmentMediaIds?: string[];
}

interface LessonBase {
  title: string;
  unitId: string;
  required: boolean;
}

/** Body of `POST /stage-versions/{vid}/lessons`, added at the end of its unit. */
export type LessonBody =
  | (LessonBase & { type: "video"; videoMediaId: string; durationSeconds?: number })
  | (LessonBase & { type: "markdown"; markdownSource: string })
  | (LessonBase & { type: "quiz"; quiz: QuizSettingsBody })
  | (LessonBase & { type: "homework"; homework: HomeworkSettingsBody });

/**
 * `PATCH` body: every field is optional and an absent one is kept. `type` is sent only to change it;
 * the unit, when sent, moves the lesson to the end of a different unit (the same one is ignored).
 */
export interface LessonPatch extends Partial<LessonBase> {
  type?: LessonType;
  videoMediaId?: string;
  durationSeconds?: number;
  markdownSource?: string;
  quiz?: QuizSettingsBody;
  homework?: HomeworkSettingsBody;
}

function quizSettings(
  values: Extract<LessonFormOutput, { kind: "quiz:practice" | "quiz:assessment" }>,
): QuizSettingsBody {
  if (values.kind === "quiz:practice") return { mode: "practice", shuffle: values.shuffle };
  return {
    mode: "assessment",
    shuffle: values.shuffle,
    passPercent: values.quizPassPercent,
    maxAttempts: values.maxAttempts,
    timeLimitMinutes: values.timeLimitMinutes,
  };
}

export function toLessonBody(values: LessonValues): LessonBody {
  const base = { title: values.title, unitId: values.unitId, required: values.required };
  switch (values.kind) {
    case "video":
      return {
        ...base,
        type: "video",
        videoMediaId: values.videoMediaId,
        durationSeconds: parseDuration(values.duration),
      };
    case "markdown":
      return { ...base, type: "markdown", markdownSource: values.markdownSource };
    case "quiz:practice":
    case "quiz:assessment":
      return { ...base, type: "quiz", quiz: quizSettings(values) };
    case "homework":
      return { ...base, type: "homework", homework: { passPercent: values.homeworkPassPercent } };
  }
}

export interface LessonPatchOptions {
  /** Leave the markdown out, so the server keeps what it has and skips its source checks. */
  keepSource?: boolean;
  /** The lesson's type before the save; `type` is sent only when the kind changes it. */
  from?: LessonType;
}

export function toLessonPatch(
  values: LessonValues,
  { keepSource = false, from }: LessonPatchOptions = {},
): LessonPatch {
  const { type, ...body } = toLessonBody(values);
  const patch: LessonPatch = body;
  if (from !== undefined && from !== type) patch.type = type;
  if (keepSource) delete patch.markdownSource;
  return patch;
}

/** Assessment limits a quiz starts with, as on the server. */
const QUIZ_DEFAULTS = { passPercent: 70, maxAttempts: 2, timeLimitMinutes: 20 };

/**
 * Dialog defaults: a new required video lesson in `unitId` (the unit whose "Thêm học liệu" was
 * clicked), or the lesson being edited. Quiz limits start from the server's defaults so switching to
 * an assessment shows what would be saved.
 */
export function lessonFormDefaults(lesson?: Lesson, unitId = ""): LessonFormValues {
  const quiz = lesson?.quiz;
  return {
    kind: lesson ? lessonKindOf(lesson) : "video",
    title: lesson?.title ?? "",
    unitId: lesson?.unitId ?? unitId,
    required: lesson?.required ?? true,
    videoMediaId: lesson?.videoMediaId ?? "",
    duration: fmtDuration(lesson?.durationSeconds),
    markdownSource: lesson?.markdownSource ?? "",
    shuffle: quiz?.shuffle ?? true,
    quizPassPercent: String(quiz?.passPercent ?? QUIZ_DEFAULTS.passPercent),
    maxAttempts: String(quiz?.maxAttempts ?? QUIZ_DEFAULTS.maxAttempts),
    timeLimitMinutes: String(quiz?.timeLimitMinutes ?? QUIZ_DEFAULTS.timeLimitMinutes),
    homeworkPassPercent: String(lesson?.homework?.passPercent ?? QUIZ_DEFAULTS.passPercent),
  };
}

/** Title, unit and flag picked in the dialog before moving on to the editor page. */
export interface MarkdownLessonDraft {
  title: string;
  unitId: string;
  required: boolean;
}

/** Editor page defaults: the lesson being edited, else what the dialog passed on, else a blank required lesson. */
export function markdownLessonDefaults(lesson?: Lesson, draft?: MarkdownLessonDraft): MarkdownLessonValues {
  return {
    kind: "markdown",
    title: lesson?.title ?? draft?.title ?? "",
    unitId: lesson?.unitId ?? draft?.unitId ?? "",
    required: lesson?.required ?? draft?.required ?? true,
    markdownSource: lesson?.markdownSource ?? "",
  };
}

/** Router state is untyped: accept only a well-formed draft. */
export function readMarkdownLessonDraft(state: unknown): MarkdownLessonDraft | undefined {
  if (typeof state !== "object" || state === null) return undefined;
  const { title, unitId, required } = state as Partial<Record<keyof MarkdownLessonDraft, unknown>>;
  return typeof title === "string" && typeof unitId === "string" && typeof required === "boolean"
    ? { title, unitId, required }
    : undefined;
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

export const QUIZ_KIND_LABEL: Record<QuizMode, string> = { practice: "Quiz học liệu", assessment: "Quiz bài tập" };

/**
 * Lesson row meta: "Video · {file} · {mm:ss}", "Markdown · {n} ký tự",
 * "{quiz kind} · {n} câu · {p} điểm · …limits" or "Bài tập · {n} tiêu chí · {max} điểm · đạt từ {p}%".
 */
export function lessonMeta(lesson: Lesson): string {
  switch (lesson.type) {
    case "video":
      return ["Video", lesson.videoFileName, fmtDuration(lesson.durationSeconds)].filter(Boolean).join(" · ");
    case "markdown":
      return `Markdown · ${String(lesson.markdownSource?.length ?? 0)} ký tự`;
    case "quiz": {
      const quiz = lesson.quiz;
      if (!quiz) return QUIZ_KIND_LABEL.practice;
      const parts = [
        QUIZ_KIND_LABEL[quiz.mode],
        `${String(quiz.questions.length)} câu`,
        `${String(quizTotalPoints(quiz.questions))} điểm`,
      ];
      if (quiz.mode === "assessment") {
        parts.push(
          `đạt từ ${String(quiz.passPercent)}%`,
          `${String(quiz.maxAttempts)} lượt`,
          `${String(quiz.timeLimitMinutes)} phút`,
        );
      } else {
        parts.push("không giới hạn lượt");
      }
      return parts.join(" · ");
    }
    case "homework": {
      const homework = lesson.homework;
      if (!homework) return "Bài tập";
      return `Bài tập · ${String(homework.criteria.length)} tiêu chí · ${String(homework.maxPoints)} điểm · đạt từ ${String(homework.passPercent)}%`;
    }
  }
}

/** "{u} bài học · {n} học liệu bắt buộc · {m} tùy chọn". */
export function requiredSplitLabel(unitCount: number, lessons: readonly Pick<Lesson, "required">[]): string {
  const required = lessons.filter((l) => l.required).length;
  return `${String(unitCount)} bài học · ${String(required)} học liệu bắt buộc · ${String(lessons.length - required)} tùy chọn`;
}

/** "{n} học liệu · {m} bắt buộc" under a unit's title. */
export function unitMeta(unit: { lessonCount: number; requiredCount: number }): string {
  return `${String(unit.lessonCount)} học liệu · ${String(unit.requiredCount)} bắt buộc`;
}
