import { isApiError } from "@/shared/api/errors";
import { UPLOAD_FAILED_MESSAGE } from "../model/video-file";
import type { Media } from "../model/schemas";
import { mediaApi, type UploadRequest } from "./media-api";

/** Storage refused the PUT or the connection dropped; carries the user-facing message. */
export class UploadFailed extends Error {}

/** Message for a failed upload: the API's or storage's own text when known, else the generic one. */
export function uploadErrorMessage(error: unknown): string {
  return isApiError(error) || error instanceof UploadFailed ? error.message : UPLOAD_FAILED_MESSAGE;
}

/**
 * PUT straight to the presigned storage URL. Raw XHR for upload progress; only `Content-Type` is
 * set and no credentials are sent: the target is the bucket, not the API, and any extra header
 * would break the signature or the CORS preflight.
 */
export function putToStorage(
  url: string,
  file: File,
  onProgress: (pct: number) => void,
  signal: AbortSignal,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    const onAbort = () => {
      xhr.abort();
    };
    const done = (fn: () => void) => {
      signal.removeEventListener("abort", onAbort);
      fn();
    };
    xhr.open("PUT", url);
    xhr.setRequestHeader("Content-Type", file.type);
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable && event.total > 0) onProgress(Math.round((event.loaded / event.total) * 100));
    };
    xhr.onload = () => {
      done(() => {
        if (xhr.status >= 200 && xhr.status < 300) resolve();
        else reject(new UploadFailed(UPLOAD_FAILED_MESSAGE));
      });
    };
    xhr.onerror = () => {
      done(() => {
        reject(new UploadFailed(UPLOAD_FAILED_MESSAGE));
      });
    };
    xhr.onabort = () => {
      done(() => {
        reject(new DOMException("Upload cancelled", "AbortError"));
      });
    };
    signal.addEventListener("abort", onAbort);
    xhr.send(file);
  });
}

export interface UploadMediaOptions {
  onProgress?: (pct: number) => void;
  signal?: AbortSignal;
}

/**
 * Upload in three steps: reserve (`POST /media/uploads`), PUT to storage with progress, confirm
 * (`POST /media/uploads/{id}/complete`). Rejects with `ApiError`, `UploadFailed` or an abort error.
 */
export async function uploadMedia(
  kind: UploadRequest["kind"],
  file: File,
  { onProgress, signal = new AbortController().signal }: UploadMediaOptions = {},
): Promise<Media> {
  const ticket = await mediaApi.initUpload(
    { kind, fileName: file.name, contentType: file.type, sizeBytes: file.size },
    signal,
  );
  await putToStorage(
    ticket.uploadUrl,
    file,
    (pct) => {
      if (!signal.aborted) onProgress?.(pct);
    },
    signal,
  );
  return mediaApi.completeUpload(ticket.mediaId, signal);
}
