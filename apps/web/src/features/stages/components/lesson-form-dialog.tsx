import { zodResolver } from "@hookform/resolvers/zod";
import { useId, useRef, useState, type ChangeEvent } from "react";
import { Controller, useForm, useWatch } from "react-hook-form";
import { Alert } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { Dialog, DialogBody, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/shared/ui/dialog";
import { Field } from "@/shared/ui/field";
import { Input } from "@/shared/ui/input";
import { Progress } from "@/shared/ui/progress";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/shared/ui/select";
import { useVideoUpload, type VideoUploadState } from "../hooks/use-video-upload";
import {
  fmtDuration,
  lessonFormDefaults,
  lessonFormSchema,
  lessonErrorMessage,
  type LessonFormOutput,
  type LessonFormValues,
  type MarkdownLessonDraft,
  type VideoLessonOutput,
} from "../model/lesson-form";
import type { Lesson } from "../model/schemas";
import { fmtBytes, VIDEO_CONTENT_TYPE } from "../model/video-file";

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
  /** The video lesson being edited; absent when adding. Markdown lessons are edited on their own page. */
  lesson?: Lesson;
  /** Throw (an ApiError) to keep the dialog open with the message; resolve to close it. */
  onSave: (values: VideoLessonOutput) => Promise<unknown>;
  /** A new markdown lesson goes on to the editor page with what was typed here. */
  onContinueMarkdown: (draft: MarkdownLessonDraft) => void;
}

/**
 * "Thêm học liệu"/"Sửa học liệu" for video lessons, and the first step of a markdown one. The form
 * mounts on open, so closing drops it and aborts any upload.
 */
export function LessonFormDialog({ open, onOpenChange, lesson, onSave, onContinueMarkdown }: LessonFormDialogProps) {
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

interface LessonFormProps {
  lesson?: Lesson;
  onSave: (values: VideoLessonOutput) => Promise<unknown>;
  onContinueMarkdown: (draft: MarkdownLessonDraft) => void;
  /** Lets the dialog refuse to close while a save is in flight. */
  onSubmittingChange: (submitting: boolean) => void;
  onClose: () => void;
}

function LessonForm({ lesson, onSave, onContinueMarkdown, onSubmittingChange, onClose }: LessonFormProps) {
  const editing = Boolean(lesson);
  const form = useForm<LessonFormValues, unknown, LessonFormOutput>({
    resolver: zodResolver(lessonFormSchema),
    defaultValues: lessonFormDefaults(lesson),
  });
  const { control, register, setValue, formState } = form;
  const type = useWatch({ control, name: "type" });
  const videoMediaId = useWatch({ control, name: "videoMediaId" });
  const upload = useVideoUpload();
  const [formError, setFormError] = useState<string | null>(null);
  const uploadingPct = upload.state.status === "uploading" ? upload.state.pct : null;
  const uploading = uploadingPct !== null;
  const submitting = formState.isSubmitting;

  const submit = form.handleSubmit(async (values) => {
    if (values.type === "markdown") {
      onContinueMarkdown({ title: values.title, required: values.required });
      return;
    }
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
    if (mediaId) setValue("videoMediaId", mediaId, { shouldValidate: formState.isSubmitted });
  }

  let submitLabel = editing ? "Lưu" : "Thêm";
  if (type === "markdown") submitLabel = "Tiếp tục soạn";
  if (uploading) submitLabel = `Đang tải lên ${String(uploadingPct)}%`;

  return (
    <form noValidate onSubmit={(e) => void submit(e)}>
      <DialogHeader showClose={!submitting}>
        <DialogTitle>{editing ? "Sửa học liệu" : "Thêm học liệu"}</DialogTitle>
      </DialogHeader>
      <DialogBody>
        <Field label="Tiêu đề" required error={formState.errors.title?.message}>
          <Input required {...register("title")} />
        </Field>
        <div className="grid grid-cols-2 gap-4 max-[720px]:grid-cols-1">
          <Controller
            control={control}
            name="type"
            render={({ field }) => (
              <Select value={field.value} onValueChange={field.onChange} disabled={editing}>
                <Field label="Loại">
                  <SelectTrigger onBlur={field.onBlur}>
                    <SelectValue />
                  </SelectTrigger>
                </Field>
                <SelectContent>
                  <SelectItem value="video">Video</SelectItem>
                  <SelectItem value="markdown">Markdown</SelectItem>
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
        {type === "video" ? (
          <>
            <VideoFileField
              state={upload.state}
              savedFileName={lesson?.videoMediaId && lesson.videoMediaId === videoMediaId ? lesson.videoFileName : undefined}
              error={formState.errors.videoMediaId?.message}
              onPick={(event) => void pickFile(event)}
            />
            <Field label="Thời lượng" error={formState.errors.duration?.message}>
              <Input placeholder="mm:ss" autoComplete="off" {...register("duration")} />
            </Field>
          </>
        ) : null}
        {formError ? (
          <Alert variant="danger" role="alert">
            {formError}
          </Alert>
        ) : null}
      </DialogBody>
      <DialogFooter>
        <Button variant="outline" disabled={submitting} onClick={onClose}>
          Hủy
        </Button>
        <Button type="submit" disabled={submitting || uploading} aria-busy={submitting || uploading}>
          {submitLabel}
        </Button>
      </DialogFooter>
    </form>
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
