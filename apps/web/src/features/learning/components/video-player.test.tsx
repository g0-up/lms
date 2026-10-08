import { QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api, server } from "@/shared/test/msw";
import { createTestQueryClient } from "@/shared/test/render";
import { fixtures, freshExpiry } from "../test/golden";
import type { VideoContent } from "../model/schemas";
import { MAX_URL_RECOVERIES, VideoPlayer } from "./video-player";

const golden = fixtures.lessonVideo.content as VideoContent;
const content = (expiresAt = freshExpiry()): VideoContent => ({ ...golden, url: "https://media.test/v?sig=first", expiresAt });

/** jsdom has no media pipeline: stub what the player reads and writes on every `<video>`. */
const media = { time: 0, paused: true, play: vi.fn<() => Promise<void>>() };
const descriptors = {
  currentTime: Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, "currentTime"),
  paused: Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, "paused"),
  play: Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, "play"),
};

beforeEach(() => {
  media.time = 0;
  media.paused = true;
  media.play = vi.fn(() => Promise.resolve());
  Object.defineProperties(HTMLMediaElement.prototype, {
    currentTime: {
      configurable: true,
      get: () => media.time,
      set: (t: number) => {
        media.time = t;
      },
    },
    paused: { configurable: true, get: () => media.paused },
    play: { configurable: true, value: () => media.play() },
  });
});

afterEach(() => {
  for (const [key, descriptor] of Object.entries(descriptors)) {
    if (descriptor) Object.defineProperty(HTMLMediaElement.prototype, key, descriptor);
  }
});

function signedUrls() {
  const served: string[] = [];
  server.use(
    http.get(api("/media/:mediaId/url"), ({ params }) => {
      expect(params.mediaId).toBe(golden.mediaId);
      const url = `https://media.test/v?sig=renewed-${String(served.length + 1)}`;
      served.push(url);
      return HttpResponse.json({ url, expiresAt: freshExpiry() });
    }),
  );
  return served;
}

function renderPlayer(videoContent: VideoContent) {
  return render(
    <QueryClientProvider client={createTestQueryClient()}>
      <VideoPlayer lessonId={fixtures.lessonVideo.lesson.id} content={videoContent} />
    </QueryClientProvider>,
  );
}

function video(): HTMLVideoElement {
  const el = document.querySelector("video");
  if (!el) throw new Error("no <video> rendered");
  return el;
}

describe("VideoPlayer", () => {
  it("plays the URL from the lesson response without another request", () => {
    const served = signedUrls();
    renderPlayer(content());

    const el = video();
    expect(el).toHaveAttribute("src", "https://media.test/v?sig=first");
    expect(el).toHaveAttribute("controls");
    expect(el).toHaveAttribute("preload", "metadata");
    expect(el).not.toHaveAttribute("autoplay");
    expect(el).not.toHaveAttribute("crossorigin");
    expect(served).toHaveLength(0);
  });

  it("re-signs a URL that expires within a minute before playing it", async () => {
    const served = signedUrls();
    renderPlayer(content(freshExpiry(30_000)));

    await waitFor(() => {
      expect(video()).toHaveAttribute("src", "https://media.test/v?sig=renewed-1");
    });
    expect(served).toHaveLength(1);
  });

  it("re-signs after a load error and resumes from the same position", async () => {
    const served = signedUrls();
    renderPlayer(content());
    media.time = 42;
    media.paused = false;

    fireEvent.error(video());
    await waitFor(() => {
      expect(video()).toHaveAttribute("src", "https://media.test/v?sig=renewed-1");
    });
    media.time = 0;
    fireEvent.loadedMetadata(video());

    expect(media.time).toBe(42);
    expect(media.play).toHaveBeenCalledTimes(1);
    expect(served).toHaveLength(1);
  });

  it("keeps the position when the recovery load fails as well", async () => {
    signedUrls();
    renderPlayer(content());
    media.time = 42;
    media.paused = false;

    fireEvent.error(video());
    await waitFor(() => {
      expect(video()).toHaveAttribute("src", "https://media.test/v?sig=renewed-1");
    });
    // The replacement element never loaded: it reports position 0 and paused.
    media.time = 0;
    media.paused = true;
    fireEvent.error(video());
    await waitFor(() => {
      expect(video()).toHaveAttribute("src", "https://media.test/v?sig=renewed-2");
    });
    fireEvent.loadedMetadata(video());

    expect(media.time).toBe(42);
    expect(media.play).toHaveBeenCalledTimes(1);
  });

  it("does not start a paused video when it recovers", async () => {
    signedUrls();
    renderPlayer(content());
    media.time = 7;

    fireEvent.error(video());
    await waitFor(() => {
      expect(video()).toHaveAttribute("src", "https://media.test/v?sig=renewed-1");
    });
    fireEvent.loadedMetadata(video());

    expect(media.time).toBe(7);
    expect(media.play).not.toHaveBeenCalled();
  });

  it(`gives up with an error after ${String(MAX_URL_RECOVERIES)} recoveries`, async () => {
    const served = signedUrls();
    renderPlayer(content());

    for (let attempt = 1; attempt <= MAX_URL_RECOVERIES; attempt += 1) {
      fireEvent.error(video());
      await waitFor(() => {
        expect(video()).toHaveAttribute("src", `https://media.test/v?sig=renewed-${String(attempt)}`);
      });
    }
    fireEvent.error(video());

    expect(await screen.findByRole("alert")).toHaveTextContent("Không phát được video. Tải lại trang để thử lại.");
    expect(document.querySelector("video")).toBeNull();
    expect(served).toHaveLength(MAX_URL_RECOVERIES);
  });
});
