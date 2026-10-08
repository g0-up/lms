/**
 * Fixtures for the learning tests, straight from the API's golden JSON
 * (`apps/api/internal/features/{learning,media}/testdata`). Every fixture goes through the same Zod
 * schema the app uses, so a contract change on the API turns these tests red. Plain data, no
 * network: `model/` tests may use it. Test-only: nothing in the app imports this module.
 */
import completionJson from "../../../../../api/internal/features/learning/testdata/completion.json";
import completionUntickedJson from "../../../../../api/internal/features/learning/testdata/completion-unticked.json";
import classNotActiveJson from "../../../../../api/internal/features/learning/testdata/error-class-not-active.json";
import classNotStartedJson from "../../../../../api/internal/features/learning/testdata/error-class-not-started.json";
import lessonNotInCourseJson from "../../../../../api/internal/features/learning/testdata/error-lesson-not-in-course.json";
import notMemberJson from "../../../../../api/internal/features/learning/testdata/error-not-member.json";
import notOpenedJson from "../../../../../api/internal/features/learning/testdata/error-not-opened.json";
import lessonEndedJson from "../../../../../api/internal/features/learning/testdata/lesson-ended.json";
import lessonMarkdownJson from "../../../../../api/internal/features/learning/testdata/lesson-markdown.json";
import lessonVideoJson from "../../../../../api/internal/features/learning/testdata/lesson-video.json";
import myClassActiveJson from "../../../../../api/internal/features/learning/testdata/my-class-basic01.json";
import myClassDraftJson from "../../../../../api/internal/features/learning/testdata/my-class-draft.json";
import myClassesJson from "../../../../../api/internal/features/learning/testdata/my-classes.json";
import signedUrlJson from "../../../../../api/internal/features/media/testdata/signed_url.json";
import { errorEnvelopeSchema } from "@/shared/api/schemas";
import {
  completionSchema,
  lessonPageSchema,
  myClassesSchema,
  roadmapSchema,
  signedUrlSchema,
  type Roadmap,
} from "../model/schemas";

/** Raw golden payloads, for the contract test. */
export const golden = {
  myClasses: myClassesJson,
  myClassActive: myClassActiveJson,
  myClassDraft: myClassDraftJson,
  lessonVideo: lessonVideoJson,
  lessonMarkdown: lessonMarkdownJson,
  lessonEnded: lessonEndedJson,
  completion: completionJson,
  completionUnticked: completionUntickedJson,
  signedUrl: signedUrlJson,
};

export const errors = {
  /** 404: not (or no longer) an active member, or no such class. */
  notMember: errorEnvelopeSchema.parse(notMemberJson),
  /** 404: the lesson is not part of the class's course version. */
  lessonNotInCourse: errorEnvelopeSchema.parse(lessonNotInCourseJson),
  /** 409 CONFLICT: ticking a lesson never opened. */
  notOpened: errorEnvelopeSchema.parse(notOpenedJson),
  /** 409 INVALID_TRANSITION: progress write on a draft or ended class. */
  classNotActive: errorEnvelopeSchema.parse(classNotActiveJson),
  /** 409 INVALID_TRANSITION: opening a lesson of a draft class. */
  classNotStarted: errorEnvelopeSchema.parse(classNotStartedJson),
};

const myClasses = myClassesSchema.parse(myClassesJson);
const roadmapActive = roadmapSchema.parse(myClassActiveJson);

/** basic00 (ended) shares basic01's course version; the API has no golden roadmap for it. */
const roadmapEnded: Roadmap = {
  ...roadmapActive,
  class: { ...roadmapActive.class, id: "01990000-0000-7000-8000-000000000953", code: "basic00", name: "Lập trình cơ bản – khóa 0", status: "ended" },
  readOnly: true,
  readOnlyReason: "ended",
};

export const fixtures = {
  myClasses,
  roadmapActive,
  roadmapDraft: roadmapSchema.parse(myClassDraftJson),
  roadmapEnded,
  lessonVideo: lessonPageSchema.parse(lessonVideoJson),
  lessonMarkdown: lessonPageSchema.parse(lessonMarkdownJson),
  lessonEnded: lessonPageSchema.parse(lessonEndedJson),
  completion: completionSchema.parse(completionJson),
  completionUnticked: completionSchema.parse(completionUntickedJson),
  signedUrl: signedUrlSchema.parse(signedUrlJson),
};

export const ids = {
  active: fixtures.roadmapActive.class.id,
  draft: fixtures.roadmapDraft.class.id,
  ended: roadmapEnded.class.id,
  video: fixtures.lessonVideo.lesson.id,
  markdown: fixtures.lessonMarkdown.lesson.id,
  optional: fixtures.lessonEnded.lesson.id,
};

/** Golden signed URLs carry a fixed past expiry; tests need one that is still valid (API default TTL 2h). */
export function freshExpiry(ms = 2 * 60 * 60 * 1000): string {
  return new Date(Date.now() + ms).toISOString();
}
