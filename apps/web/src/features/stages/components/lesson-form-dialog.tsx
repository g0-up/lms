import { zodResolver } from "@hookform/resolvers/zod";
import { useId, useRef, useState, type ChangeEvent } from "react";
import { Controller, useForm, useWatch } from "react-hook-form";
import { Alert } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Dialog, DialogBody, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/shared/ui/dialog";
import { Field } from "@/shared/ui/field";
import { Input } from "@/shared/ui/input";
import { Progress } from "@/shared/ui/progress";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui/select";
import { Textarea } from "@/shared/ui/textarea";
import { useVideoUpload, type VideoUploadState } from "../hooks/use-video-upload";
import {
  dropsContent,
  fmtDuration,
  LESSON_KIND_HELP_CREATE,
  LESSON_KIND_HELP_EDIT,
  LESSON_KINDS,
  lessonFormDefaults,
  lessonFormSchema,
  lessonErrorMessage,
  type LessonFormOutput,
  type LessonFormValues,
  type MarkdownLessonDraft,
} from "../model/lesson-form";
import { markdownSourceError } from "../model/markdown-rules";
import type { Lesson, Unit } from "../model/schemas";
import { fmtBytes, VIDEO_CONTENT_TYPE } from "../model/video-file";
import { UnitSelect } from "./unit-select";

/**
 * Duration of a local video file from its metadata, or `undefined` when the browser cannot read
 * it (no object URLs, unsupported codec, or a media policy that refuses `blob:` sources).
 */
function readVideoDuration(file: File): Promise<number | undefined> {
  if (typeof URL.createObjectURL !== "function") return Promise.resolve(undefined);
  return new Promise((resolve) => {
    const url = URL.createObjectURL(file);
    const video = document.createElement("video");
    const finish = (seconds: number | undefined) => {
      video.onloadedmetadata = null;
      video.onerror = null;
      video.removeAttribute("src");
      URL.revokeObjectURL(url);
      resolve(seconds);
    };
    video.preload = "metadata";
    video.onloadedmetadata = () => {
      finish(Number.isFinite(video.duration) ? video.duration : undefined);
    };
    video.onerror = () => {
      finish(undefined);
    };
    video.src = url;
  });
}

export interface LessonFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The lesson being edited; absent when adding. Markdown lessons are edited on their own page. */
  lesson?: Lesson;
  /** The version's units, in order, for the "Bài học" select. */
  units: readonly Pick<Unit, "id" | "title">[];
  /** Unit preselected for a new lesson. */
  unitId?: string;
  /** Throw (an ApiError) to keep the dialog open with the message; resolve to close it. */
  onSave: (values: LessonFormOutput) => Promise<unknown>;
  /** A new markdown lesson goes on to the editor page with what was typed here; needed only when adding. */
  onContinueMarkdown?: (draft: MarkdownLessonDraft) => void;
}

/**
 * "Thêm học liệu"/"Sửa học liệu" for every type, and the first step of a new markdown lesson. The
 * form mounts on open, so closing drops it and aborts any upload.
 */
export function LessonFormDialog({
  open,
  onOpenChange,
  lesson,
  units,
  unitId,
  onSave,
  onContinueMarkdown,
}: LessonFormDialogProps) {
  const contentRef = useRef<HTMLDivElement>(null);
  const [submitting, setSubmitting] = useState(false);
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!submitting) onOpenChange(next);
      }}
    >
      <DialogContent
        ref={contentRef}
        aria-describedby={undefined}
        onOpenAutoFocus={(event) => {
          const first = contentRef.current?.querySelector<HTMLElement>("input:not([type=hidden]):not([type=file])");
          if (first) {
            event.preventDefault();
            first.focus();
          }
        }}
      >
        <LessonForm
          lesson={lesson}
          units={units}
          unitId={unitId}
          onSave={onSave}
          onContinueMarkdown={onContinueMarkdown}
          onSubmittingChange={setSubmitting}
          onClose={() => {
            onOpenChange(false);
          }}
        />
      </DialogContent>
    </Dialog>
  );
}

/** What a lesson of `lesson.type` loses when its type changes, for the confirmation. */
function lostContent(lesson: Lesson): string {
  switch (lesson.type) {
    case "video":
      return "file video";
    case "markdown":
      return "nội dung bài đọc";
    case "quiz":
      return `${String(lesson.quiz?.questions.length ?? 0)} câu hỏi`;
    case "homework":
      return `đề bài, ${String(lesson.homework?.criteria.length ?? 0)} tiêu chí chấm và file kèm`;
  }
}

interface LessonFormProps {
  lesson?: Lesson;
  units: readonly Pick<Unit, "id" | "title">[];
  unitId?: string;
  onSave: (values: LessonFormOutput) => Promise<unknown>;
  onContinueMarkdown?: (draft: MarkdownLessonDraft) => void;
  /** Lets the dialog refuse to close while a save is in flight. */
  onSubmittingChange: (submitting: boolean) => void;
  onClose: () => void;
}

function LessonForm({
  lesson,
  units,
  unitId,
  onSave,
  onContinueMarkdown,
  onSubmittingChange,
  onClose,
}: LessonFormProps) {
  const editing = Boolean(lesson);
  const form = useForm<LessonFormValues, unknown, LessonFormOutput>({
    resolver: zodResolver(lessonFormSchema),
    defaultValues: lessonFormDefaults(lesson, unitId),
  });
  const { control, register, setValue, setError, formState } = form;
  const kind = useWatch({ control, name: "kind" });
  const videoMediaId = useWatch({ control, name: "videoMediaId" });
  const upload = useVideoUpload();
  const [formError, setFormError] = useState<string | null>(null);
  // Values held back until the type change is confirmed.
  const [pending, setPending] = useState<LessonFormOutput | null>(null);
  const uploadingPct = upload.state.status === "uploading" ? upload.state.pct : null;
  const uploading = uploadingPct !== null;
  const submitting = formState.isSubmitting;
  const continuesToEditor = !editing && kind === "markdown";

  async function save(values: LessonFormOutput) {
    setFormError(null);
    onSubmittingChange(true);
    try {
      await onSave(values);
      onClose();
    } catch (error) {
      setFormError(lessonErrorMessage(error));
    } finally {
      onSubmittingChange(false);
    }
  }

  const submit = form.handleSubmit(async (values) => {
    if (values.kind === "markdown") {
      if (!editing) {
        onContinueMarkdown?.({
          title: values.title,
          unitId: values.unitId,
          required: values.required,
        });
        return;
      }
      const sourceError = markdownSourceError(values.markdownSource);
      if (sourceError) {
        setError("markdownSource", { message: sourceError });
        return;
      }
    }
    if (lesson && dropsContent(lesson.type, values.kind)) {
      setPending(values);
      return;
    }
    await save(values);
  });

  async function pickFile(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    // Clearing lets the same file be picked again after a failure.
    event.target.value = "";
    if (!file) return;
    setValue("videoMediaId", "");
    void readVideoDuration(file).then((seconds) => {
      if (seconds !== undefined) setValue("duration", fmtDuration(seconds));
    });
    const mediaId = await upload.start(file);
    if (mediaId)
      setValue("videoMediaId", mediaId, {
        shouldValidate: formState.isSubmitted,
      });
  }

  let submitLabel = editing ? "Lưu" : "Thêm";
  if (continuesToEditor) submitLabel = "Tiếp tục soạn";
  if (uploading) submitLabel = `Đang tải lên ${String(uploadingPct)}%`;
  const busy = submitting || pending !== null;
  const kindLabel = LESSON_KINDS.find((k) => k.value === pending?.kind)?.label ?? "";

  return (
    <>
      <form noValidate onSubmit={(e) => void submit(e)}>
        <DialogHeader showClose={!busy}>
          <DialogTitle>{editing ? "Sửa học liệu" : "Thêm học liệu"}</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <Field label="Tiêu đề" required error={formState.errors.title?.message}>
            <Input required {...register("title")} />
          </Field>
          <Controller
            control={control}
            name="unitId"
            render={({ field }) => (
              <UnitSelect
                units={units}
                value={field.value}
                onChange={field.onChange}
                onBlur={field.onBlur}
                editing={editing}
                error={formState.errors.unitId?.message}
              />
            )}
          />
          <div className="grid grid-cols-2 gap-4 max-[720px]:grid-cols-1">
            <Controller
              control={control}
              name="kind"
              render={({ field }) => (
                <Select value={field.value} onValueChange={field.onChange}>
                  <Field label="Loại" help={editing ? LESSON_KIND_HELP_EDIT : LESSON_KIND_HELP_CREATE}>
                    <SelectTrigger onBlur={field.onBlur}>
                      <SelectValue />
                    </SelectTrigger>
                  </Field>
                  <SelectContent>
                    {LESSON_KINDS.map((k) => (
                      <SelectItem key={k.value} value={k.value}>
                        {k.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            />
            <Controller
              control={control}
              name="required"
              render={({ field }) => (
                <label className="flex min-h-11 cursor-pointer items-center gap-2 self-end text-sm text-ink">
                  <Checkbox
                    checked={field.value}
                    onCheckedChange={(checked) => {
                      field.onChange(checked === true);
                    }}
                    onBlur={field.onBlur}
                  />
                  Bắt buộc (tính vào %)
                </label>
              )}
            />
          </div>
          {kind === "video" ? (
            <>
              <VideoFileField
                state={upload.state}
                savedFileName={
                  lesson?.videoMediaId && lesson.videoMediaId === videoMediaId ? lesson.videoFileName : undefined
                }
                error={formState.errors.videoMediaId?.message}
                onPick={(event) => void pickFile(event)}
              />
              <Field label="Thời lượng" error={formState.errors.duration?.message}>
                <Input placeholder="mm:ss" autoComplete="off" {...register("duration")} />
              </Field>
            </>
          ) : null}
          {kind === "markdown" && editing ? (
            <Field
              label="Nội dung Markdown"
              required
              help="Hỗ trợ tiêu đề, danh sách, khối mã, trích dẫn, **đậm** và `mã`. HTML được lọc khi hiển thị."
              error={formState.errors.markdownSource?.message}
            >
              <Textarea rows={10} {...register("markdownSource")} />
            </Field>
          ) : null}
          {kind === "quiz:assessment" ? (
            <div className="grid grid-cols-3 gap-4 max-[720px]:grid-cols-1">
              <Field label="Ngưỡng đạt (%)" required error={formState.errors.quizPassPercent?.message}>
                <Input inputMode="numeric" autoComplete="off" {...register("quizPassPercent")} />
              </Field>
              <Field
                label="Số lượt làm bài"
                required
                help="Tính lượt tốt nhất"
                error={formState.errors.maxAttempts?.message}
              >
                <Input inputMode="numeric" autoComplete="off" {...register("maxAttempts")} />
              </Field>
              <Field label="Thời gian mỗi lượt (phút)" required error={formState.errors.timeLimitMinutes?.message}>
                <Input inputMode="numeric" autoComplete="off" {...register("timeLimitMinutes")} />
              </Field>
            </div>
          ) : null}
          {kind === "quiz:practice" || kind === "quiz:assessment" ? (
            <>
              <Controller
                control={control}
                name="shuffle"
                render={({ field }) => (
                  <label className="flex min-h-11 cursor-pointer items-center gap-2 text-sm text-ink">
                    <Checkbox
                      checked={field.value}
                      onCheckedChange={(checked) => {
                        field.onChange(checked === true);
                      }}
                      onBlur={field.onBlur}
                    />
                    Đảo thứ tự câu hỏi và đáp án mỗi lượt
                  </label>
                )}
              />
              <p className="text-sm text-ink-3">
                {editing
                  ? "Soạn câu hỏi ở trang chi tiết học liệu."
                  : "Sau khi tạo, bạn được chuyển đến trang soạn câu hỏi."}
              </p>
            </>
          ) : null}
          {kind === "homework" ? (
            <>
              <Field label="Ngưỡng đạt (%)" required error={formState.errors.homeworkPassPercent?.message}>
                <Input inputMode="numeric" autoComplete="off" {...register("homeworkPassPercent")} />
              </Field>
              <p className="text-sm text-ink-3">
                {editing
                  ? "Soạn đề bài và tiêu chí chấm ở trang chi tiết học liệu."
                  : "Sau khi tạo, bạn được chuyển đến trang soạn tiêu chí chấm."}
              </p>
            </>
          ) : null}
          {formError ? (
            <Alert variant="danger" role="alert">
              {formError}
            </Alert>
          ) : null}
        </DialogBody>
        <DialogFooter>
          <Button variant="outline" disabled={busy} onClick={onClose}>
            Hủy
          </Button>
          <Button type="submit" disabled={busy || uploading} aria-busy={submitting || uploading}>
            {submitLabel}
          </Button>
        </DialogFooter>
      </form>
      {lesson ? (
        <ConfirmDialog
          open={pending !== null}
          onOpenChange={(open) => {
            if (!open) setPending(null);
          }}
          danger
          title={`Đổi loại "${lesson.title}"?`}
          text={`Đổi sang ${kindLabel} sẽ bỏ ${lostContent(lesson)} của học liệu này. Không hoàn tác được.`}
          confirm="Đổi loại"
          loading={submitting}
          onConfirm={() => {
            const values = pending;
            setPending(null);
            if (values) void save(values);
          }}
        />
      ) : null}
    </>
  );
}

interface VideoFileFieldProps {
  state: VideoUploadState;
  /** File name of the lesson's current video while it is still the selected one. */
  savedFileName: string | undefined;
  error: string | undefined;
  onPick: (event: ChangeEvent<HTMLInputElement>) => void;
}

/** File picker with name, size and upload progress; the native input stays hidden behind a button. */
function VideoFileField({ state, savedFileName, error, onPick }: VideoFileFieldProps) {
  const id = useId();
  const inputRef = useRef<HTMLInputElement>(null);
  const labelId = `${id}-label`;
  const infoId = `${id}-info`;
  const errorId = `${id}-error`;
  const shownError = state.status === "error" ? state.message : error;
  const hasFile = state.status !== "idle" || Boolean(savedFileName);

  let info;
  if (state.status === "idle") {
    info = savedFileName ? <span className="text-sm text-ink">{savedFileName}</span> : null;
  } else {
    info = (
      <span className="text-sm text-ink">
        {state.fileName} <span className="text-ink-3">· {fmtBytes(state.sizeBytes)}</span>
        {state.status === "done" ? <span className="text-ink-3"> · Đã tải lên</span> : null}
      </span>
    );
  }

  return (
    <div role="group" aria-labelledby={labelId} className="grid gap-1.5">
      <span id={labelId} className="text-sm font-semibold text-ink">
        File video
      </span>
      <input
        ref={inputRef}
        type="file"
        accept={VIDEO_CONTENT_TYPE}
        tabIndex={-1}
        aria-labelledby={labelId}
        className="hidden"
        onChange={onPick}
      />
      <div className="flex flex-wrap items-center gap-3">
        <Button
          variant="outline"
          size="sm"
          aria-describedby={[info ? infoId : "", shownError ? errorId : ""].filter(Boolean).join(" ") || undefined}
          onClick={() => inputRef.current?.click()}
        >
          {hasFile ? "Chọn file khác" : "Chọn file"}
        </Button>
        {info ? <span id={infoId}>{info}</span> : null}
      </div>
      {state.status === "uploading" ? (
        <Progress value={state.pct} aria-label={`Đã tải lên ${String(state.pct)}%`} />
      ) : null}
      {shownError ? (
        <div id={errorId} role="alert" className="text-xs text-danger">
          {shownError}
        </div>
      ) : null}
    </div>
  );
}
