import { z } from "zod";
import { isApiError } from "./errors";
import { http } from "./http";
import { isoDate } from "./schemas";

export const UPLOAD_FAILED_MESSAGE = "Tải file lên thất bại. Thử lại.";

/**
 * `POST /media/uploads` (201). The upload URL is presigned: never log or display it. `headers` are the
 * ones the URL was signed with (the server's canonical content type), to be sent as-is on the PUT.
 */
export const uploadTicketSchema = z.object({
  mediaId: z.string().min(1),
  uploadUrl: z.string().min(1),
  expiresAt: isoDate,
  headers: z.record(z.string(), z.string()),
});
export type UploadTicket = z.infer<typeof uploadTicketSchema>;

/** `POST /media/uploads/{id}/complete`. */
export const mediaSchema = z.object({
  id: z.string().min(1),
  kind: z.string(),
  fileName: z.string(),
  contentType: z.string(),
  sizeBytes: z.number().int().nonnegative(),
  status: z.string(),
});
export type Media = z.infer<typeof mediaSchema>;

export type UploadKind = "video" | "image" | "document";

export interface UploadRequest {
  kind: UploadKind;
  fileName: string;
  contentType: string;
  sizeBytes: number;
}

/**
 * Upload endpoints: `/media/uploads` for content authors, `/me/uploads` for a student's homework files
 * (documents only). Both take the same body and return the same ticket and media.
 */
export type UploadPath = "/media/uploads" | "/me/uploads";

export const mediaUploadApi = {
  /** Reserves a media id and returns a short-lived presigned PUT URL for the storage bucket. */
  initUpload: (body: UploadRequest, signal?: AbortSignal, path: UploadPath = "/media/uploads"): Promise<UploadTicket> =>
    http(path, { method: "POST", body, schema: uploadTicketSchema, signal }),
  /** Confirms the object landed in storage; 422 "Chưa nhận được file" when it did not. */
  completeUpload: (mediaId: string, signal?: AbortSignal, path: UploadPath = "/media/uploads"): Promise<Media> =>
    http(`${path}/${encodeURIComponent(mediaId)}/complete`, { method: "POST", schema: mediaSchema, signal }),
};

/** Storage refused the PUT or the connection dropped; carries the user-facing message. */
export class UploadFailed extends Error {}

/** Message for a failed upload: the API's or storage's own text when known, else the generic one. */
export function uploadErrorMessage(error: unknown): string {
  return isApiError(error) || error instanceof UploadFailed ? error.message : UPLOAD_FAILED_MESSAGE;
}

/**
 * PUT straight to the presigned storage URL. Raw XHR for upload progress; only the ticket's signed
 * headers are set and no credentials are sent: the target is the bucket, not the API, and any other
 * header (or the browser's guess at the file type) would break the signature or the CORS preflight.
 */
export function putToStorage(
  ticket: Pick<UploadTicket, "uploadUrl" | "headers">,
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
    xhr.open("PUT", ticket.uploadUrl);
    for (const [name, value] of Object.entries(ticket.headers)) xhr.setRequestHeader(name, value);
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
  /** Endpoint to reserve and confirm through; the authoring one by default. */
  path?: UploadPath;
}

/**
 * Upload in three steps: reserve (`POST {path}`), PUT to storage with progress, confirm
 * (`POST {path}/{id}/complete`). Rejects with `ApiError`, `UploadFailed` or an abort error.
 */
export async function uploadMedia(
  kind: UploadKind,
  file: File,
  { onProgress, signal = new AbortController().signal, path = "/media/uploads" }: UploadMediaOptions = {},
): Promise<Media> {
  const ticket = await mediaUploadApi.initUpload(
    { kind, fileName: file.name, contentType: file.type, sizeBytes: file.size },
    signal,
    path,
  );
  await putToStorage(
    ticket,
    file,
    (pct) => {
      if (!signal.aborted) onProgress?.(pct);
    },
    signal,
  );
  return mediaUploadApi.completeUpload(ticket.mediaId, signal, path);
}
