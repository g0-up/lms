import { useRef, useState } from "react";
import { Alert } from "@/shared/ui/alert";
import { Skeleton } from "@/shared/ui/skeleton";
import { useVideoUrl } from "../hooks/use-video-url";
import type { VideoContent } from "../model/schemas";

/** Load errors recovered by re-signing the URL before giving up. */
export const MAX_URL_RECOVERIES = 2;

export interface VideoPlayerProps {
  lessonId: string;
  content: VideoContent;
}

/**
 * Plays a lesson video from its short-lived signed URL.
 *
 * A `MediaError` carries no HTTP status, so an expired URL looks like any other load failure: on
 * error the player remembers the position and whether it was playing, re-signs the URL and resumes
 * in `loadedmetadata` (the only point where iOS Safari honours `currentTime`). After
 * `MAX_URL_RECOVERIES` attempts it shows an error instead of retrying forever.
 */
export function VideoPlayer({ lessonId, content }: VideoPlayerProps) {
  const { data, isFetching, refetch } = useVideoUrl(lessonId, content);
  const resume = useRef<{ time: number; playing: boolean } | null>(null);
  const [failures, setFailures] = useState(0);

  if (failures > MAX_URL_RECOVERIES) {
    return (
      <Alert variant="danger" role="alert" className="m-6 max-[720px]:mx-4">
        Không phát được video. Tải lại trang để thử lại.
      </Alert>
    );
  }

  if (isFetching) {
    return (
      <div aria-busy="true" className="grid aspect-video w-full place-items-center bg-(image:--gradient-navy) p-6">
        <Skeleton className="h-16 w-16 rounded-full bg-white/20" />
        <span className="sr-only">Đang tải video</span>
      </div>
    );
  }

  return (
    // No `crossOrigin`: it would make the bucket answer CORS for a plain playback request.
    // `controlsList="nodownload"` only hides a menu item; it protects nothing (the URL is in the DOM).
    <video
      src={data.url}
      controls
      preload="metadata"
      playsInline
      controlsList="nodownload"
      className="block aspect-video w-full bg-navy-deep"
      onError={(event) => {
        const video = event.currentTarget;
        // Keep a pending position: a replacement element that fails before loading reports 0 and paused.
        resume.current ??= { time: video.currentTime, playing: !video.paused };
        setFailures((n) => n + 1);
        if (failures < MAX_URL_RECOVERIES) void refetch();
      }}
      onLoadedMetadata={(event) => {
        const position = resume.current;
        if (!position) return;
        resume.current = null;
        const video = event.currentTarget;
        video.currentTime = position.time;
        // Autoplay policies may refuse play(); the controls stay usable either way.
        if (position.playing) video.play().catch(() => undefined);
      }}
    />
  );
}
