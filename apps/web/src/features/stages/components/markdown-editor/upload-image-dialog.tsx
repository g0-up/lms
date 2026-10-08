import {
  closeImageDialog$,
  imageDialogState$,
  saveImage$,
  useCellValues,
  usePublisher,
  type EditingImageDialogState,
  type SaveImageParameters,
} from "@mdxeditor/editor";
import { useEffect, useRef, useState, type ChangeEvent, type SubmitEvent } from "react";
import { Alert } from "@/shared/ui/alert";
import { Button } from "@/shared/ui/button";
import { Dialog, DialogBody, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/shared/ui/dialog";
import { Field } from "@/shared/ui/field";
import { Input } from "@/shared/ui/input";
import { Progress } from "@/shared/ui/progress";
import { uploadErrorMessage, uploadMedia } from "../../api/upload-media";
import { altFromFileName, IMAGE_CONTENT_TYPES, imageFileError, mediaContentUrl } from "../../model/image-file";

const IMAGE_ALT_MAX = 200;
const IMAGE_ALT_REQUIRED = "Nhập mô tả ảnh.";
const IMAGE_FILE_REQUIRED = "Chọn một ảnh để tải lên.";

/**
 * The editor's image dialog, upload only: no URL field, so every image in a lesson is a media file
 * the API stores. Alt text is required. Editing an image changes its alt and keeps its file.
 */
export function UploadImageDialog() {
  const [state] = useCellValues(imageDialogState$);
  const saveImage = usePublisher(saveImage$);
  const close = usePublisher(closeImageDialog$);
  const open = state.type !== "inactive";

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) close();
      }}
    >
      {open ? (
        <DialogContent aria-describedby={undefined}>
          <ImageForm editing={state.type === "editing" ? state : null} onSave={saveImage} onCancel={close} />
        </DialogContent>
      ) : null}
    </Dialog>
  );
}

interface ImageFormProps {
  editing: EditingImageDialogState | null;
  onSave: (values: SaveImageParameters) => void;
  onCancel: () => void;
}

function ImageForm({ editing, onSave, onCancel }: ImageFormProps) {
  const [file, setFile] = useState<File | null>(null);
  const [alt, setAlt] = useState(editing?.initialValues.altText ?? "");
  const [altTyped, setAltTyped] = useState(Boolean(editing));
  const [fileError, setFileError] = useState<string>();
  const [altError, setAltError] = useState<string>();
  const [uploadError, setUploadError] = useState<string>();
  const [pct, setPct] = useState<number | null>(null);
  const controllerRef = useRef<AbortController | null>(null);

  useEffect(() => () => controllerRef.current?.abort(), []);

  function pickFile(event: ChangeEvent<HTMLInputElement>) {
    const picked = event.target.files?.[0] ?? null;
    setFile(picked);
    setUploadError(undefined);
    setFileError(picked ? (imageFileError(picked) ?? undefined) : undefined);
    if (picked && !altTyped) setAlt(altFromFileName(picked.name));
  }

  async function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    // The dialog is portalled, but React bubbles synthetic events along the component tree: without
    // this the lesson page's own form would save and leave the page.
    event.stopPropagation();
    if (pct !== null) return;
    const altText = alt.trim();
    const missingFile = !editing && !file ? IMAGE_FILE_REQUIRED : undefined;
    const badFile = file ? (imageFileError(file) ?? undefined) : undefined;
    setAltError(altText ? undefined : IMAGE_ALT_REQUIRED);
    setFileError(missingFile ?? badFile);
    if (!altText || missingFile || badFile) return;

    if (editing) {
      onSave({ ...editing.initialValues, altText });
      return;
    }
    if (!file) return;

    const controller = new AbortController();
    controllerRef.current = controller;
    setUploadError(undefined);
    setPct(0);
    try {
      const media = await uploadMedia("image", file, { onProgress: setPct, signal: controller.signal });
      if (!controller.signal.aborted) onSave({ src: mediaContentUrl(media.id), altText });
    } catch (error) {
      if (controller.signal.aborted) return;
      setUploadError(uploadErrorMessage(error));
      setPct(null);
    }
  }

  const uploading = pct !== null;
  let submitLabel = editing ? "Lưu" : "Chèn ảnh";
  if (uploading) submitLabel = `Đang tải lên ${String(pct)}%`;

  return (
    <form noValidate onSubmit={(e) => void submit(e)}>
      <DialogHeader>
        <DialogTitle>{editing ? "Sửa ảnh" : "Chèn ảnh"}</DialogTitle>
      </DialogHeader>
      <DialogBody>
        {editing ? null : (
          <Field label="File ảnh" required help="PNG, JPEG, WebP hoặc GIF, tối đa 10 MB." error={fileError}>
            <Input
              type="file"
              required
              accept={IMAGE_CONTENT_TYPES.join(",")}
              disabled={uploading}
              className="h-auto py-2"
              onChange={pickFile}
            />
          </Field>
        )}
        <Field
          label="Mô tả ảnh (alt)"
          required
          help="Mô tả ngắn cho người không xem được ảnh."
          error={altError}
        >
          <Input
            required
            maxLength={IMAGE_ALT_MAX}
            value={alt}
            disabled={uploading}
            onChange={(event) => {
              setAlt(event.target.value);
              setAltTyped(true);
            }}
          />
        </Field>
        {uploading ? <Progress value={pct} aria-label={`Đã tải lên ${String(pct)}%`} /> : null}
        {uploadError ? (
          <Alert variant="danger" role="alert">
            {uploadError}
          </Alert>
        ) : null}
      </DialogBody>
      <DialogFooter>
        <Button variant="outline" onClick={onCancel}>
          Hủy
        </Button>
        <Button type="submit" disabled={uploading} aria-busy={uploading}>
          {submitLabel}
        </Button>
      </DialogFooter>
    </form>
  );
}
