import { http } from "@/shared/api/http";
import { mediaSchema, uploadTicketSchema, type Media, type UploadTicket } from "../model/schemas";

export interface UploadRequest {
  kind: "video" | "image";
  fileName: string;
  contentType: string;
  sizeBytes: number;
}

export const mediaApi = {
  /** Reserves a media id and returns a short-lived presigned PUT URL for the storage bucket. */
  initUpload: (body: UploadRequest, signal?: AbortSignal): Promise<UploadTicket> =>
    http("/media/uploads", { method: "POST", body, schema: uploadTicketSchema, signal }),
  /** Confirms the object landed in storage; 422 "Chưa nhận được file" when it did not. */
  completeUpload: (mediaId: string, signal?: AbortSignal): Promise<Media> =>
    http(`/media/uploads/${encodeURIComponent(mediaId)}/complete`, { method: "POST", schema: mediaSchema, signal }),
};
