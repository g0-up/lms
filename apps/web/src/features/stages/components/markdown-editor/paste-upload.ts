import { toast } from "sonner";
import { uploadMedia } from "../../api/upload-media";
import { altFromFileName, imageFileError, mediaContentUrl } from "../../model/image-file";
import { UPLOAD_FAILED_MESSAGE } from "../../model/video-file";

/** Most pasted or dropped images one editor queues at once. */
export const MAX_PENDING_PASTES = 5;
export const PASTE_LIMIT_MESSAGE = "Chỉ tải tối đa 5 ảnh mỗi lần.";

export interface PasteUploader {
  /** MDXEditor `imageUploadHandler`: never rejects; `""` means "no image" and the guard drops the node. */
  uploadPasted: (file: File) => Promise<string>;
  /** Alt text recorded for an uploaded src, from its file name. */
  altFor: (src: string) => string | undefined;
  /** Aborts the uploads in flight or queued; later pastes upload normally. */
  cancelAll: () => void;
}

/**
 * Upload handler for images pasted or dropped into one editor. Uploads run one at a time so a burst
 * of pastes cannot exhaust the `/media` rate limit; every failure becomes a toast instead of an
 * unhandled rejection inside the editor.
 */
export function createPasteUploader(onPendingChange?: (count: number) => void): PasteUploader {
  const alts = new Map<string, string>();
  let controller = new AbortController();
  let pending = 0;
  let tail: Promise<unknown> = Promise.resolve();

  const setPending = (next: number) => {
    pending = next;
    onPendingChange?.(pending);
  };

  async function upload(file: File, signal: AbortSignal): Promise<string> {
    try {
      if (signal.aborted) return "";
      const media = await uploadMedia("image", file, { signal });
      const src = mediaContentUrl(media.id);
      alts.set(src, altFromFileName(file.name));
      return src;
    } catch {
      if (!signal.aborted) toast.error(UPLOAD_FAILED_MESSAGE, { id: "paste-upload" });
      return "";
    } finally {
      setPending(pending - 1);
    }
  }

  return {
    uploadPasted(file) {
      const invalid = imageFileError(file);
      if (invalid) {
        toast.error(invalid);
        return Promise.resolve("");
      }
      if (pending >= MAX_PENDING_PASTES) {
        toast.error(PASTE_LIMIT_MESSAGE, { id: "paste-limit" });
        return Promise.resolve("");
      }
      setPending(pending + 1);
      const { signal } = controller;
      const result = tail.then(() => upload(file, signal));
      tail = result;
      return result;
    },
    altFor: (src) => alts.get(src),
    cancelAll() {
      controller.abort();
      controller = new AbortController();
    },
  };
}
