import { Paperclip, Upload, X } from "lucide-react";
import { useEffect, useId, useRef, useState, type ChangeEvent } from "react";
import { toast } from "sonner";
import { uploadErrorMessage, uploadMedia } from "@/shared/api/upload-media";
import { Button } from "@/shared/ui/button";
import { IconButton } from "@/shared/ui/icon-button";
import { Progress } from "@/shared/ui/progress";
import { useHomeworkPatch } from "../hooks/use-criterion-mutations";
import { DOCUMENT_ACCEPT, documentFileError } from "../model/attachment-file";
import { mediaContentUrl } from "../model/image-file";
import { lessonErrorMessage } from "../model/lesson-form";
import type { Attachment } from "../model/schemas";
import { fmtBytes } from "../model/video-file";

export interface AttachmentListProps {
  stageId: string;
  vid: string;
  lessonId: string;
  attachments: readonly Attachment[];
  /** Only a draft can upload or remove files. */
  editable: boolean;
}

type UploadState = { fileName: string; pct: number } | null;

/**
 * "File đính kèm" of a homework brief: download chips, and in a draft an upload with progress. A file
 * is uploaded as a `document` first, then the full attachment list is saved with it appended.
 */
export function AttachmentList({ stageId, vid, lessonId, attachments, editable }: AttachmentListProps) {
  const patch = useHomeworkPatch(stageId, vid, lessonId);
  const labelId = useId();
  const errorId = useId();
  const inputRef = useRef<HTMLInputElement>(null);
  const controllerRef = useRef<AbortController | null>(null);
  const [upload, setUpload] = useState<UploadState>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => () => controllerRef.current?.abort(), []);

  const busy = upload !== null || patch.isPending;
  const ids = attachments.map((a) => a.mediaId);

  async function onPick(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    // Picking the same file again after a failure must fire `change` again.
    event.target.value = "";
    if (!file) return;
    setError(null);
    const invalid = documentFileError(file);
    if (invalid) {
      setError(invalid);
      return;
    }
    const controller = new AbortController();
    controllerRef.current = controller;
    setUpload({ fileName: file.name, pct: 0 });
    let mediaId: string;
    try {
      const media = await uploadMedia("document", file, {
        onProgress: (pct) => {
          setUpload({ fileName: file.name, pct });
        },
        signal: controller.signal,
      });
      mediaId = media.id;
    } catch (err) {
      if (controller.signal.aborted) return;
      setUpload(null);
      setError(uploadErrorMessage(err));
      return;
    }
    if (controller.signal.aborted) return;
    setUpload(null);
    try {
      await patch.mutateAsync({ attachmentMediaIds: [...ids, mediaId] });
      toast.success(`Đã đính kèm ${file.name}.`);
    } catch (err) {
      setError(lessonErrorMessage(err));
    }
  }

  async function detach(attachment: Attachment) {
    setError(null);
    try {
      await patch.mutateAsync({ attachmentMediaIds: ids.filter((id) => id !== attachment.mediaId) });
      toast.success(`Đã gỡ ${attachment.fileName}.`);
    } catch (err) {
      setError(lessonErrorMessage(err));
    }
  }

  if (!editable && attachments.length === 0) return null;
  return (
    <div role="group" aria-labelledby={labelId} className="grid gap-2">
      <span id={labelId} className="text-xs font-semibold tracking-[.5px] text-ink-3 uppercase">
        File đính kèm
      </span>
      {attachments.length > 0 ? (
        <ul className="m-0 flex list-none flex-wrap gap-2 p-0">
          {attachments.map((attachment) => (
            <li key={attachment.mediaId} className="flex items-center border border-line bg-surface-50">
              <a
                href={mediaContentUrl(attachment.mediaId)}
                download={attachment.fileName}
                className="flex min-h-11 items-center gap-2 px-3 text-sm text-ink no-underline hover:text-navy-700 hover:underline"
              >
                <Paperclip className="size-4 shrink-0" aria-hidden="true" />
                <span>{attachment.fileName}</span>
                <span className="text-xs text-ink-3">{fmtBytes(attachment.sizeBytes)}</span>
              </a>
              {editable ? (
                <IconButton
                  aria-label={`Gỡ file ${attachment.fileName}`}
                  disabled={busy}
                  onClick={() => void detach(attachment)}
                >
                  <X aria-hidden="true" />
                </IconButton>
              ) : null}
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-sm text-ink-3">Chưa có file kèm.</p>
      )}
      {editable ? (
        <>
          <input
            ref={inputRef}
            type="file"
            accept={DOCUMENT_ACCEPT}
            tabIndex={-1}
            aria-label="Chọn file đính kèm"
            className="hidden"
            onChange={(e) => void onPick(e)}
          />
          <div>
            <Button
              variant="outline"
              size="sm"
              disabled={busy}
              aria-busy={busy}
              aria-describedby={error ? errorId : undefined}
              onClick={() => inputRef.current?.click()}
            >
              <Upload aria-hidden="true" /> {upload ? `Đang tải ${upload.fileName}…` : "Tải file lên"}
            </Button>
          </div>
          {upload ? <Progress value={upload.pct} aria-label={`Đã tải lên ${String(upload.pct)}%`} /> : null}
          {error ? (
            <div id={errorId} role="alert" className="text-xs text-danger">
              {error}
            </div>
          ) : null}
        </>
      ) : null}
    </div>
  );
}
