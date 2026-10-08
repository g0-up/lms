import { http } from "@/shared/api/http";
import { signedUrlSchema, type SignedUrl } from "../model/schemas";

export const mediaApi = {
  /** Fresh short-lived signed URL for a stored file; never build storage URLs on the client. */
  signedUrl: (mediaId: string, signal?: AbortSignal): Promise<SignedUrl> =>
    http(`/media/${encodeURIComponent(mediaId)}/url`, { schema: signedUrlSchema, signal }),
};
