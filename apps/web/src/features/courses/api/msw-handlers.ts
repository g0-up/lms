/**
 * MSW handlers for the course API (and the stage list it reads), answering with the golden
 * fixtures of `test/golden.ts`. Test-only: nothing in the app imports this module.
 */
import { http, HttpResponse } from "msw";
import type { StageList } from "@/features/stages";
import { api } from "@/shared/test/msw";
import type { CourseDetail, CourseList, CourseVersion } from "../model/schemas";
import { errors, fixtures } from "../test/golden";

export function errorResponse(status: number, envelope: { error: { code: string; message: string } }) {
  return HttpResponse.json(envelope, { status });
}

export interface CourseWorld {
  list: CourseList;
  detail: CourseDetail;
  versions: CourseVersion[];
  stageList: StageList;
}

/** BASIC with only v1 published, used by basic01. */
export const publishedWorld = (): CourseWorld => ({
  list: fixtures.courseList,
  detail: fixtures.courseDetail,
  versions: [fixtures.courseVersion],
  stageList: fixtures.stageList,
});

/** BASIC v1 published, v2 a draft; `draft` replaces the golden draft (same id). */
export const draftWorld = (draft: CourseVersion = fixtures.courseVersionDraft, stageList = fixtures.stageList): CourseWorld => ({
  list: fixtures.courseList,
  detail: fixtures.courseDetailWithDraft,
  versions: [fixtures.courseVersion, draft],
  stageList,
});

/** Happy-path course API over one course. */
export function coursesHandlers(world: CourseWorld = publishedWorld()) {
  const findVersion = (vid: unknown) => world.versions.find((v) => v.id === String(vid));
  const notFound = () => errorResponse(404, { error: { code: "NOT_FOUND", message: "Không tìm thấy." } });
  return [
    http.get(api("/stages"), () => HttpResponse.json(world.stageList)),
    http.get(api("/courses"), () => HttpResponse.json(world.list)),
    http.post(api("/courses"), () => HttpResponse.json(world.detail, { status: 201 })),
    http.get(api("/courses/:courseId"), ({ params }) =>
      String(params.courseId) === world.detail.id ? HttpResponse.json(world.detail) : notFound(),
    ),
    http.get(api("/course-versions/:vid"), ({ params }) => {
      const version = findVersion(params.vid);
      return version ? HttpResponse.json(version) : notFound();
    }),
    http.put(api("/course-versions/:vid/stages"), async ({ params, request }) => {
      const version = findVersion(params.vid);
      if (!version) return notFound();
      if (version.status !== "draft") return errorResponse(409, errors.versionImmutable);
      const { stageVersionIds } = (await request.json()) as { stageVersionIds: string[] };
      const rows = stageVersionIds.map((id) => fixtures.stageRows.find((r) => r.stageVersionId === id));
      if (rows.some((r) => r === undefined)) return errorResponse(422, errors.validation);
      const stages = rows.flatMap((r, i) => (r ? [{ ...r, position: i + 1 }] : []));
      return HttpResponse.json({ ...version, stages });
    }),
    http.post(api("/course-versions/:vid/clone"), () =>
      world.versions.some((v) => v.status === "draft")
        ? errorResponse(409, errors.draftExists)
        : HttpResponse.json(fixtures.courseVersionDraft, { status: 201 }),
    ),
    http.post(api("/course-versions/:vid/publish"), ({ params }) => {
      const version = findVersion(params.vid);
      if (!version) return notFound();
      return HttpResponse.json({ ...version, status: "published", publishedAt: "2026-10-05T09:00:00Z" });
    }),
    http.post(api("/course-versions/:vid/archive"), ({ params }) => {
      const version = findVersion(params.vid);
      return version ? HttpResponse.json({ ...version, status: "archived" }) : notFound();
    }),
    http.delete(api("/course-versions/:vid"), () => HttpResponse.json(fixtures.deleteVersion)),
  ];
}
