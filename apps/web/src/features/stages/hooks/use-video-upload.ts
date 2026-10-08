import { useCallback, useEffect, useRef, useState } from "react";
import { uploadErrorMessage, uploadMedia } from "../api/upload-media";
import { videoFileError } from "../model/video-file";

export type VideoUploadState =
  | { status: "idle" }
  | { status: "uploading"; fileName: string; sizeBytes: number; pct: number }
  | { status: "done"; fileName: string; sizeBytes: number; mediaId: string }
  | { status: "error"; fileName: string; sizeBytes: number; message: string };

/**
 * Video upload in three steps: reserve (`POST /media/uploads`), PUT to storage with progress,
 * confirm (`POST /media/uploads/{id}/complete`). Starting again or `cancel()` aborts the upload in
 * flight; unmounting does too.
 */
export function useVideoUpload() {
  const [state, setState] = useState<VideoUploadState>({ status: "idle" });
  const controllerRef = useRef<AbortController | null>(null);

  const cancel = useCallback(() => {
    controllerRef.current?.abort();
    controllerRef.current = null;
    setState({ status: "idle" });
  }, []);

  useEffect(() => () => controllerRef.current?.abort(), []);

  /** Resolves to the media id once stored, or `undefined` when refused, failed or cancelled. */
  const start = useCallback(async (file: File): Promise<string | undefined> => {
    controllerRef.current?.abort();
    const meta = { fileName: file.name, sizeBytes: file.size };
    const invalid = videoFileError(file);
    if (invalid) {
      controllerRef.current = null;
      setState({ status: "error", ...meta, message: invalid });
      return undefined;
    }

    const controller = new AbortController();
    controllerRef.current = controller;
    const { signal } = controller;
    setState({ status: "uploading", ...meta, pct: 0 });
    try {
      const media = await uploadMedia("video", file, {
        onProgress: (pct) => {
          setState({ status: "uploading", ...meta, pct });
        },
        signal,
      });
      if (signal.aborted) return undefined;
      setState({ status: "done", ...meta, mediaId: media.id });
      return media.id;
    } catch (error) {
      if (signal.aborted) return undefined;
      setState({ status: "error", ...meta, message: uploadErrorMessage(error) });
      return undefined;
    } finally {
      if (controllerRef.current === controller) controllerRef.current = null;
    }
  }, []);

  return { state, start, cancel };
}
