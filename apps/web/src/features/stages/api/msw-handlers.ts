/**
 * MSW handlers for the stage, lesson and media API, answering with the golden fixtures of
 * `test/golden.ts`. Test-only: nothing in the app imports this module.
 */
import { http, HttpResponse } from "msw";
import { api } from "@/shared/test/msw";
import type { StageDetail, StageVersion } from "../model/schemas";
import { errors, fixtures } from "../test/golden";

export function errorResponse(status: number, envelope: { error: { code: string; message: string } }) {
  return HttpResponse.json(envelope, { status });
}

/** Presigned PUT target of the golden upload ticket (the in-memory storage of the API tests). */
export const STORAGE_PUT_URL = "https://storage.mem/put/*";

/** The bucket is cross-origin: jsdom preflights the PUT, so every storage answer carries CORS headers. */
const STORAGE_CORS = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Methods": "PUT",
  "Access-Control-Allow-Headers": "Content-Type",
};

export const storageResponse = (status = 200) => new HttpResponse(null, { status, headers: STORAGE_CORS });

export interface StageWorld {
  detail: StageDetail;
  versions: StageVersion[];
}

/** v1 and v2 published, BASIC and HASDRAFT still on v1. */
export const publishedWorld = (): StageWorld => ({
  detail: fixtures.stageDetail,
  versions: [fixtures.stageVersion, fixtures.stageVersionV2],
});

/** v1 published and used by BASIC, v2 a draft with two lessons. */
export const draftWorld = (): StageWorld => ({
  detail: fixtures.stageDetailWithDraft,
  versions: [fixtures.stageVersion, fixtures.stageVersionDraft],
});

/** Happy-path admin content API over one stage. */
export function stagesHandlers(world: StageWorld = publishedWorld()) {
  const findVersion = (vid: unknown) => world.versions.find((v) => v.id === String(vid));
  const notFound = () => errorResponse(404, { error: { code: "NOT_FOUND", message: "Không tìm thấy." } });
  return [
    http.get(api("/stages"), () => HttpResponse.json(fixtures.stageList)),
    http.post(api("/stages"), () => HttpResponse.json(world.detail, { status: 201 })),
    http.post(api("/stages/markdown-preview"), () => HttpResponse.json(fixtures.markdownPreview)),
    http.get(api("/stages/:stageId"), ({ params }) =>
      String(params.stageId) === world.detail.id ? HttpResponse.json(world.detail) : notFound(),
    ),
    http.get(api("/stage-versions/:vid"), ({ params }) => {
      const version = findVersion(params.vid);
      return version ? HttpResponse.json(version) : notFound();
    }),
    http.post(api("/stage-versions/:vid/publish"), ({ params }) => {
      const version = findVersion(params.vid);
      if (!version) return notFound();
      return HttpResponse.json({ ...version, status: "published", publishedAt: "2026-10-05T09:00:00Z" });
    }),
    http.post(api("/stage-versions/:vid/archive"), ({ params }) => {
      const version = findVersion(params.vid);
      return version ? HttpResponse.json({ ...version, status: "archived" }) : notFound();
    }),
    http.post(api("/stage-versions/:vid/clone"), () => errorResponse(409, errors.draftExists)),
    http.delete(api("/stage-versions/:vid"), () => HttpResponse.json(fixtures.deleteVersion)),
    http.post(api("/stage-versions/:vid/lessons"), () => HttpResponse.json(fixtures.lessonVideo, { status: 201 })),
    http.patch(api("/stage-versions/:vid/lessons/:lessonId"), () => HttpResponse.json(fixtures.lessonMarkdown)),
    http.delete(api("/stage-versions/:vid/lessons/:lessonId"), () => new HttpResponse(null, { status: 204 })),
    http.put(api("/stage-versions/:vid/lessons/order"), async ({ params, request }) => {
      const version = findVersion(params.vid);
      if (!version) return notFound();
      const { lessonIds } = (await request.json()) as { lessonIds: string[] };
      const lessons = lessonIds.flatMap((id) => version.lessons.filter((l) => l.id === id));
      return HttpResponse.json({ ...version, lessons });
    }),
    http.post(api("/stage-versions/:vid/apply"), async ({ request }) => {
      const { courseIds } = (await request.json()) as { courseIds: string[] };
      return HttpResponse.json({ results: fixtures.applyResults.results.filter((r) => courseIds.includes(r.courseId)) });
    }),
    http.get(api("/course-versions/:vid"), () => HttpResponse.json(fixtures.courseVersion)),
    http.post(api("/media/uploads"), () => HttpResponse.json(fixtures.uploadTicket, { status: 201 })),
    http.options(STORAGE_PUT_URL, () => storageResponse(204)),
    http.put(STORAGE_PUT_URL, () => storageResponse()),
    http.post(api("/media/uploads/:mediaId/complete"), () => HttpResponse.json(fixtures.media)),
  ];
}
