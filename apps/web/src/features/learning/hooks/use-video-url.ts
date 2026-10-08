import { useQuery } from "@tanstack/react-query";
import { learningKeys } from "../api/learning-api";
import { mediaApi } from "../api/media-api";
import type { VideoContent } from "../model/schemas";

/** Renew a signed URL this long before it expires. */
const RENEW_MARGIN_MS = 60_000;

/**
 * Signed URL of a lesson video. Starts from the URL the lesson response already carries (no extra
 * request), counts as stale one minute before it expires and is re-signed through
 * `GET /media/{id}/url` on the next mount or when the player calls `refetch` after a load error.
 * Never refetched on focus: swapping `src` mid-playback would restart the video.
 */
export function useVideoUrl(lessonId: string, content: VideoContent) {
  return useQuery({
    queryKey: learningKeys.lessonVideo(lessonId),
    queryFn: ({ signal }) => mediaApi.signedUrl(content.mediaId, signal),
    initialData: { url: content.url, expiresAt: content.expiresAt },
    staleTime: (query) => {
      const expiresAt = query.state.data ? Date.parse(query.state.data.expiresAt) : 0;
      return Math.max(0, expiresAt - Date.now() - RENEW_MARGIN_MS);
    },
    gcTime: 0,
    refetchOnWindowFocus: false,
  });
}
