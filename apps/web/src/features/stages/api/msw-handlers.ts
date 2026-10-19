/**
 * MSW handlers for the stage, lesson and media API, answering with the golden fixtures of
 * `test/golden.ts`. Test-only: nothing in the app imports this module.
 */
import { http, HttpResponse } from "msw";
import { api } from "@/shared/test/msw";
import type { Media } from "@/shared/api/upload-media";
import type { CriterionBody } from "../model/criterion-form";
import type { LessonBody, LessonPatch } from "../model/lesson-form";
import { applyLayout, layoutOf, type UnitLessons } from "../model/order";
import type { QuestionBody } from "../model/question-form";
import type { Attachment, Homework, Lesson, Question, Quiz, StageDetail, StageVersion, Unit } from "../model/schemas";
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

/** Media id the in-memory API gives the next uploaded document. */
export const DOCUMENT_MEDIA_ID = "0199ffff-0000-7000-8000-0000000000d1";

/** Signed content type of a document, from its extension as the API derives it. */
const DOCUMENT_TYPES: Record<string, string> = {
  pdf: "application/pdf",
  zip: "application/zip",
  md: "text/plain; charset=utf-8",
  txt: "text/plain; charset=utf-8",
  sql: "text/plain; charset=utf-8",
  png: "image/png",
  jpg: "image/jpeg",
  jpeg: "image/jpeg",
};

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

/** v1 published, v2 a draft split into two bài học with one lesson each. */
export const twoUnitWorld = (): StageWorld => ({
  detail: fixtures.stageDetailWithDraft,
  versions: [fixtures.stageVersion, fixtures.stageVersionDraftTwoUnits],
});

/** v1 published, v2 a draft whose only bài học also holds the golden assessment quiz and homework. */
export const authoringWorld = (): StageWorld => ({
  detail: fixtures.stageDetailWithDraft,
  versions: [fixtures.stageVersion, fixtures.stageVersionDraftAuthoring],
});

/** A new question as the server stores it: generated ids and key, options in the order sent. */
function storedQuestion(body: QuestionBody, id: string, questionKey: string): Question {
  return {
    id,
    questionKey,
    type: body.type,
    bodySource: body.bodySource,
    explanationSource: body.explanationSource,
    points: body.points,
    options: body.options.map((o, i) => ({
      id: o.id ?? `${id}-o${String(i + 1)}`,
      body: o.body,
      isCorrect: o.correct,
    })),
  };
}

const withQuestions = (quiz: Quiz, questions: Question[]): Quiz => ({ ...quiz, questions });
const withCriteria = (homework: Homework, criteria: Homework["criteria"]): Homework => ({
  ...homework,
  criteria,
  maxPoints: criteria.reduce((sum, c) => sum + c.maxPoints, 0),
});

/** A lesson as the server creates it from a body; the quiz and homework start empty. */
function createdLesson(body: LessonBody): Lesson {
  if (body.type === "quiz") {
    const assessment = body.quiz.mode === "assessment";
    return {
      ...fixtures.lessonQuiz,
      id: "0199ffff-0000-7000-8000-0000000000a1",
      title: body.title,
      unitId: body.unitId,
      required: body.required,
      quiz: {
        mode: body.quiz.mode,
        shuffle: body.quiz.shuffle,
        passPercent: assessment ? (body.quiz.passPercent ?? 70) : null,
        maxAttempts: assessment ? (body.quiz.maxAttempts ?? 2) : null,
        timeLimitMinutes: assessment ? (body.quiz.timeLimitMinutes ?? 20) : null,
        questions: [],
      },
    };
  }
  if (body.type === "homework") {
    return {
      ...fixtures.lessonHomework,
      id: "0199ffff-0000-7000-8000-0000000000a2",
      title: body.title,
      unitId: body.unitId,
      required: body.required,
      homework: {
        briefSource: "",
        passPercent: body.homework.passPercent ?? 70,
        maxPoints: 0,
        criteria: [],
        attachments: [],
      },
    };
  }
  return fixtures.lessonVideo;
}

/**
 * Happy-path admin content API over one stage. Unit changes and reorders are written back into
 * `world`, so the refetch that follows a mutation sees them as the server would.
 */
export function stagesHandlers(world: StageWorld = publishedWorld()) {
  const findVersion = (vid: unknown) => world.versions.find((v) => v.id === String(vid));
  const store = (version: StageVersion) => {
    world.versions = world.versions.map((v) => (v.id === version.id ? version : v));
    return version;
  };
  const notFound = () => errorResponse(404, { error: { code: "NOT_FOUND", message: "Không tìm thấy." } });
  const unitNotFound = () => errorResponse(404, { error: { code: "NOT_FOUND", message: "Không tìm thấy bài học." } });
  const lessonNotFound = () =>
    errorResponse(404, { error: { code: "NOT_FOUND", message: "Không tìm thấy học liệu." } });
  /** Documents uploaded during the test, by media id, so a homework can attach them. */
  const documents = new Map<string, Media>();
  let created = 0;
  /** Applies `change` to a stored lesson and answers with the result, as the content routes do. */
  const changeLesson = (vid: unknown, lessonId: unknown, change: (lesson: Lesson) => Lesson | null) => {
    const version = findVersion(vid);
    const lesson = version?.lessons.find((l) => l.id === String(lessonId));
    if (!version || !lesson) return lessonNotFound();
    const next = change(lesson);
    if (!next) return lessonNotFound();
    store({ ...version, lessons: version.lessons.map((l) => (l.id === lesson.id ? next : l)) });
    return HttpResponse.json(next);
  };
  const removeFromLesson = (vid: unknown, lessonId: unknown, change: (lesson: Lesson) => Lesson | null) => {
    const response = changeLesson(vid, lessonId, change);
    return response.status === 200 ? new HttpResponse(null, { status: 204 }) : response;
  };
  const attachmentOf = (lesson: Lesson, mediaId: string): Attachment => {
    const kept = lesson.homework?.attachments.find((a) => a.mediaId === mediaId);
    if (kept) return kept;
    const media = documents.get(mediaId);
    return { mediaId, fileName: media?.fileName ?? "file", sizeBytes: media?.sizeBytes ?? 0 };
  };
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
    http.post(api("/stage-versions/:vid/lessons"), async ({ request }) =>
      HttpResponse.json(createdLesson((await request.json()) as LessonBody), { status: 201 }),
    ),
    http.patch(api("/stage-versions/:vid/lessons/:lessonId"), async ({ params, request }) => {
      const body = (await request.json()) as LessonPatch;
      const { quiz, homework } = body;
      const lesson = findVersion(params.vid)?.lessons.find((l) => l.id === String(params.lessonId));
      const kept = (l: Lesson) => ({ ...l, title: body.title ?? l.title, required: body.required ?? l.required });
      // A quiz keeps its questions across a settings save, as the server does.
      if (quiz && !body.type && lesson?.type === "quiz") {
        return changeLesson(params.vid, params.lessonId, (l) =>
          l.quiz
            ? {
                ...kept(l),
                quiz: {
                  ...l.quiz,
                  mode: quiz.mode,
                  shuffle: quiz.shuffle,
                  passPercent: quiz.passPercent ?? null,
                  maxAttempts: quiz.maxAttempts ?? null,
                  timeLimitMinutes: quiz.timeLimitMinutes ?? null,
                },
              }
            : null,
        );
      }
      // A homework's brief and files are kept, so the editor sees them after its refetch.
      if (!homework || body.type || lesson?.type !== "homework") return HttpResponse.json(fixtures.lessonMarkdown);
      return changeLesson(params.vid, params.lessonId, (l) =>
        l.homework
          ? {
              ...kept(l),
              homework: {
                ...l.homework,
                briefSource: homework.briefSource?.trim() ?? l.homework.briefSource,
                passPercent: homework.passPercent ?? l.homework.passPercent,
                attachments: homework.attachmentMediaIds?.map((id) => attachmentOf(l, id)) ?? l.homework.attachments,
              },
            }
          : null,
      );
    }),
    http.post(api("/stage-versions/:vid/lessons/:lessonId/questions"), async ({ params, request }) => {
      const body = (await request.json()) as QuestionBody;
      created += 1;
      const question = storedQuestion(body, `question-${String(created)}`, `cau-${String(created)}`);
      const response = changeLesson(params.vid, params.lessonId, (l) =>
        l.quiz ? { ...l, quiz: withQuestions(l.quiz, [...l.quiz.questions, question]) } : null,
      );
      return response.status === 200
        ? new HttpResponse(response.body, { status: 201, headers: response.headers })
        : response;
    }),
    http.put(api("/stage-versions/:vid/lessons/:lessonId/questions/order"), async ({ params, request }) => {
      const { questionIds } = (await request.json()) as { questionIds: string[] };
      return changeLesson(params.vid, params.lessonId, (l) =>
        l.quiz
          ? {
              ...l,
              quiz: withQuestions(
                l.quiz,
                questionIds.flatMap((id) => l.quiz?.questions.find((q) => q.id === id) ?? []),
              ),
            }
          : null,
      );
    }),
    http.patch(api("/stage-versions/:vid/lessons/:lessonId/questions/:questionId"), async ({ params, request }) => {
      const body = (await request.json()) as QuestionBody;
      const id = String(params.questionId);
      return changeLesson(params.vid, params.lessonId, (l) => {
        const old = l.quiz?.questions.find((q) => q.id === id);
        if (!l.quiz || !old) return null;
        const next = storedQuestion(body, id, old.questionKey);
        return {
          ...l,
          quiz: withQuestions(
            l.quiz,
            l.quiz.questions.map((q) => (q.id === id ? next : q)),
          ),
        };
      });
    }),
    http.delete(api("/stage-versions/:vid/lessons/:lessonId/questions/:questionId"), ({ params }) =>
      removeFromLesson(params.vid, params.lessonId, (l) =>
        l.quiz
          ? {
              ...l,
              quiz: withQuestions(
                l.quiz,
                l.quiz.questions.filter((q) => q.id !== String(params.questionId)),
              ),
            }
          : null,
      ),
    ),
    http.post(api("/stage-versions/:vid/lessons/:lessonId/criteria"), async ({ params, request }) => {
      const body = (await request.json()) as CriterionBody;
      created += 1;
      const criterion = {
        id: `criterion-${String(created)}`,
        description: body.description,
        maxPoints: body.maxPoints,
      };
      const response = changeLesson(params.vid, params.lessonId, (l) =>
        l.homework ? { ...l, homework: withCriteria(l.homework, [...l.homework.criteria, criterion]) } : null,
      );
      return response.status === 200
        ? new HttpResponse(response.body, { status: 201, headers: response.headers })
        : response;
    }),
    http.put(api("/stage-versions/:vid/lessons/:lessonId/criteria/order"), async ({ params, request }) => {
      const { criterionIds } = (await request.json()) as { criterionIds: string[] };
      return changeLesson(params.vid, params.lessonId, (l) =>
        l.homework
          ? {
              ...l,
              homework: withCriteria(
                l.homework,
                criterionIds.flatMap((id) => l.homework?.criteria.find((c) => c.id === id) ?? []),
              ),
            }
          : null,
      );
    }),
    http.patch(api("/stage-versions/:vid/lessons/:lessonId/criteria/:criterionId"), async ({ params, request }) => {
      const body = (await request.json()) as CriterionBody;
      const id = String(params.criterionId);
      return changeLesson(params.vid, params.lessonId, (l) =>
        l.homework?.criteria.some((c) => c.id === id)
          ? {
              ...l,
              homework: withCriteria(
                l.homework,
                l.homework.criteria.map((c) =>
                  c.id === id ? { id, description: body.description, maxPoints: body.maxPoints } : c,
                ),
              ),
            }
          : null,
      );
    }),
    http.delete(api("/stage-versions/:vid/lessons/:lessonId/criteria/:criterionId"), ({ params }) =>
      removeFromLesson(params.vid, params.lessonId, (l) =>
        l.homework
          ? {
              ...l,
              homework: withCriteria(
                l.homework,
                l.homework.criteria.filter((c) => c.id !== String(params.criterionId)),
              ),
            }
          : null,
      ),
    ),
    http.delete(api("/stage-versions/:vid/lessons/:lessonId"), () => new HttpResponse(null, { status: 204 })),
    http.put(api("/stage-versions/:vid/lessons/order"), async ({ params, request }) => {
      const version = findVersion(params.vid);
      if (!version) return notFound();
      const { units } = (await request.json()) as { units: UnitLessons[] };
      return HttpResponse.json(store(applyLayout(version, units)));
    }),
    http.post(api("/stage-versions/:vid/units"), async ({ params, request }) => {
      const version = findVersion(params.vid);
      if (!version) return notFound();
      const { title } = (await request.json()) as { title: string };
      const position = version.units.length + 1;
      const unit: Unit = {
        id: `unit-${String(position)}`,
        unitKey: `bai-hoc-${String(position)}`,
        title: title.trim(),
        position,
        lessonCount: 0,
        requiredCount: 0,
      };
      store({ ...version, units: [...version.units, unit] });
      return HttpResponse.json(unit, { status: 201 });
    }),
    http.patch(api("/stage-versions/:vid/units/:unitId"), async ({ params, request }) => {
      const version = findVersion(params.vid);
      const unit = version?.units.find((u) => u.id === String(params.unitId));
      if (!version || !unit) return unitNotFound();
      const { title } = (await request.json()) as { title: string };
      const renamed = { ...unit, title: title.trim() };
      store({ ...version, units: version.units.map((u) => (u.id === unit.id ? renamed : u)) });
      return HttpResponse.json(renamed);
    }),
    http.delete(api("/stage-versions/:vid/units/:unitId"), ({ params }) => {
      const version = findVersion(params.vid);
      if (!version?.units.some((u) => u.id === String(params.unitId))) return unitNotFound();
      store(
        applyLayout(
          version,
          layoutOf(version).filter((u) => u.unitId !== String(params.unitId)),
        ),
      );
      return new HttpResponse(null, { status: 204 });
    }),
    http.put(api("/stage-versions/:vid/units/order"), async ({ params, request }) => {
      const version = findVersion(params.vid);
      if (!version) return notFound();
      const { unitIds } = (await request.json()) as { unitIds: string[] };
      const layout = layoutOf(version);
      return HttpResponse.json(
        store(
          applyLayout(
            version,
            unitIds.flatMap((id) => layout.filter((u) => u.unitId === id)),
          ),
        ),
      );
    }),
    http.post(api("/stage-versions/:vid/apply"), async ({ request }) => {
      const { courseIds } = (await request.json()) as { courseIds: string[] };
      return HttpResponse.json({
        results: fixtures.applyResults.results.filter((r) => courseIds.includes(r.courseId)),
      });
    }),
    http.get(api("/course-versions/:vid"), () => HttpResponse.json(fixtures.courseVersion)),
    http.post(api("/media/uploads"), async ({ request }) => {
      const body = (await request.json()) as { kind: string; fileName: string; sizeBytes: number };
      if (body.kind !== "document") return HttpResponse.json(fixtures.uploadTicket, { status: 201 });
      const ext = body.fileName.split(".").pop()?.toLowerCase() ?? "";
      const contentType = DOCUMENT_TYPES[ext];
      if (!contentType) {
        return errorResponse(422, { error: { code: "VALIDATION_FAILED", message: "Định dạng không hỗ trợ." } });
      }
      documents.set(DOCUMENT_MEDIA_ID, {
        id: DOCUMENT_MEDIA_ID,
        kind: "document",
        fileName: body.fileName,
        contentType,
        sizeBytes: body.sizeBytes,
        status: "ready",
      });
      return HttpResponse.json(
        {
          mediaId: DOCUMENT_MEDIA_ID,
          uploadUrl: `https://storage.mem/put/media%2Fdocument%2F${DOCUMENT_MEDIA_ID}.${ext}?X-Amz-Signature=mem`,
          expiresAt: fixtures.uploadTicket.expiresAt,
          headers: { "Content-Type": contentType },
        },
        { status: 201 },
      );
    }),
    http.options(STORAGE_PUT_URL, () => storageResponse(204)),
    http.put(STORAGE_PUT_URL, () => storageResponse()),
    http.post(api("/media/uploads/:mediaId/complete"), ({ params }) =>
      HttpResponse.json(documents.get(String(params.mediaId)) ?? fixtures.media),
    ),
  ];
}
