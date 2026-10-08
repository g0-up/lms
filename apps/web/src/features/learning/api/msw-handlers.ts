/**
 * MSW handlers for the student learning API, answering with the golden fixtures of
 * `test/golden.ts`. Test-only: nothing in the app imports this module.
 */
import { http, HttpResponse } from "msw";
import { api } from "@/shared/test/msw";
import type { LessonPage, Roadmap } from "../model/schemas";
import { errors, fixtures, freshExpiry, ids } from "../test/golden";

function withFreshVideoUrl(page: LessonPage): LessonPage {
  return page.content.type === "video" ? { ...page, content: { ...page.content, expiresAt: freshExpiry() } } : page;
}

export function errorResponse(status: number, envelope: { error: { code: string; message: string } }) {
  return HttpResponse.json(envelope, { status });
}

const roadmaps: Partial<Record<string, Roadmap>> = {
  [ids.active]: fixtures.roadmapActive,
  [ids.draft]: fixtures.roadmapDraft,
  [ids.ended]: fixtures.roadmapEnded,
};

const lessons: Partial<Record<string, Partial<Record<string, LessonPage>>>> = {
  [ids.active]: { [ids.video]: fixtures.lessonVideo, [ids.markdown]: fixtures.lessonMarkdown },
  [ids.ended]: { [ids.optional]: fixtures.lessonEnded },
};

/** Happy-path student API: basic01 active, basic03 draft, basic00 ended. */
export const learningHandlers = [
  http.get(api("/me/classes"), () => HttpResponse.json(fixtures.myClasses)),
  http.get(api("/me/classes/:classId"), ({ params }) => {
    const roadmap = roadmaps[String(params.classId)];
    return roadmap ? HttpResponse.json(roadmap) : errorResponse(404, errors.notMember);
  }),
  http.get(api("/me/classes/:classId/lessons/:lessonId"), ({ params }) => {
    const classId = String(params.classId);
    if (classId === ids.draft) return errorResponse(409, errors.classNotStarted);
    if (!(classId in roadmaps)) return errorResponse(404, errors.notMember);
    const page = lessons[classId]?.[String(params.lessonId)];
    return page ? HttpResponse.json(withFreshVideoUrl(page)) : errorResponse(404, errors.lessonNotInCourse);
  }),
  http.put(api("/me/classes/:classId/lessons/:lessonId/completion"), async ({ request }) => {
    const body = (await request.json()) as { completed?: boolean };
    return HttpResponse.json(body.completed ? fixtures.completion : fixtures.completionUnticked);
  }),
  http.get(api("/media/:mediaId/url"), () => HttpResponse.json({ ...fixtures.signedUrl, expiresAt: freshExpiry() })),
];
