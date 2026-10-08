/**
 * Fixtures for the stage tests, straight from the API's golden JSON
 * (`apps/api/internal/features/{stages,courses,media}/testdata`). Every fixture goes through the
 * same Zod schema the app uses, so a contract change on the API turns these tests red. Plain data,
 * no network: `model/` tests may use it. Test-only: nothing in the app imports this module.
 */
import applyResultsJson from "../../../../../api/internal/features/courses/testdata/apply_results.json";
import courseVersionJson from "../../../../../api/internal/features/courses/testdata/course_version.json";
import mediaJson from "../../../../../api/internal/features/media/testdata/media.json";
import notUploadedJson from "../../../../../api/internal/features/media/testdata/error_not_uploaded.json";
import uploadTicketJson from "../../../../../api/internal/features/media/testdata/upload_ticket.json";
import deleteVersionJson from "../../../../../api/internal/features/stages/testdata/delete_version.json";
import draftExistsJson from "../../../../../api/internal/features/stages/testdata/error_draft_exists.json";
import inUseJson from "../../../../../api/internal/features/stages/testdata/error_in_use.json";
import validationJson from "../../../../../api/internal/features/stages/testdata/error_validation.json";
import versionImmutableJson from "../../../../../api/internal/features/stages/testdata/error_version_immutable.json";
import lessonMarkdownJson from "../../../../../api/internal/features/stages/testdata/lesson_markdown.json";
import lessonVideoJson from "../../../../../api/internal/features/stages/testdata/lesson_video.json";
import markdownPreviewJson from "../../../../../api/internal/features/stages/testdata/markdown_preview.json";
import stageDetailJson from "../../../../../api/internal/features/stages/testdata/stage_detail.json";
import stageListJson from "../../../../../api/internal/features/stages/testdata/stage_list.json";
import stageVersionDraftJson from "../../../../../api/internal/features/stages/testdata/stage_version_draft.json";
import stageVersionJson from "../../../../../api/internal/features/stages/testdata/stage_version.json";
import seedMarkdownJson from "../../../../../api/internal/seed/testdata/seed_markdown.json";
import { z } from "zod";
import { errorEnvelopeSchema } from "@/shared/api/schemas";
import {
  applyResultsSchema,
  courseVersionSummarySchema,
  deleteStageVersionSchema,
  lessonSchema,
  markdownPreviewSchema,
  mediaSchema,
  stageDetailSchema,
  stageListSchema,
  stageVersionSchema,
  uploadTicketSchema,
  type StageDetail,
  type StageVersion,
} from "../model/schemas";

/** Raw golden payloads, for the contract test. */
export const golden = {
  stageList: stageListJson,
  stageDetail: stageDetailJson,
  stageVersion: stageVersionJson,
  stageVersionDraft: stageVersionDraftJson,
  lessonVideo: lessonVideoJson,
  lessonMarkdown: lessonMarkdownJson,
  deleteVersion: deleteVersionJson,
  applyResults: applyResultsJson,
  courseVersion: courseVersionJson,
  uploadTicket: uploadTicketJson,
  media: mediaJson,
  markdownPreview: markdownPreviewJson,
};

export const errors = {
  /** 409 DRAFT_EXISTS on clone, with `details.draftVersionNo`. */
  draftExists: errorEnvelopeSchema.parse(draftExistsJson),
  /** 409 IN_USE on deleting a referenced version. */
  inUse: errorEnvelopeSchema.parse(inUseJson),
  /** 422 VALIDATION_FAILED. */
  validation: errorEnvelopeSchema.parse(validationJson),
  /** 409 VERSION_IMMUTABLE on editing a published version. */
  versionImmutable: errorEnvelopeSchema.parse(versionImmutableJson),
  /** 422 on completing an upload that never reached storage. */
  notUploaded: errorEnvelopeSchema.parse(notUploadedJson),
};

const stageList = stageListSchema.parse(stageListJson);
/** Database with v1 and v2 published; three published courses still on v1. */
const stageDetail = stageDetailSchema.parse(stageDetailJson);
const stageVersion = stageVersionSchema.parse(stageVersionJson);
const stageVersionDraft = stageVersionSchema.parse(stageVersionDraftJson);

/** The golden detail is taken after v2 was published; its v2 has the draft's lessons and UPTODATE's reference. */
const stageVersionV2: StageVersion = {
  ...stageVersionDraft,
  status: "published",
  publishedAt: stageDetail.versions.find((v) => v.id === stageVersionDraft.id)?.publishedAt,
  usedBy: stageDetail.usedBy
    .filter((u) => u.stageVersionNo === stageVersionDraft.versionNo)
    .map(({ stageVersionNo: _no, outdated: _outdated, ...row }) => row),
};

/** The same stage as the golden list sees it: v1 published, v2 still a draft, nothing outdated yet. */
const stageDetailWithDraft: StageDetail = {
  ...stageDetail,
  versions: stageList.items[0].versions,
  usedBy: stageDetail.usedBy.filter((u) => u.stageVersionNo === stageVersion.versionNo && u.courseCode === "BASIC"),
  outdatedCourses: [],
};

export const fixtures = {
  stageList,
  stageDetail,
  stageDetailWithDraft,
  stageVersion,
  stageVersionV2,
  stageVersionDraft,
  lessonVideo: lessonSchema.parse(lessonVideoJson),
  lessonMarkdown: lessonSchema.parse(lessonMarkdownJson),
  deleteVersion: deleteStageVersionSchema.parse(deleteVersionJson),
  applyResults: applyResultsSchema.parse(applyResultsJson),
  courseVersion: courseVersionSummarySchema.parse(courseVersionJson),
  uploadTicket: uploadTicketSchema.parse(uploadTicketJson),
  media: mediaSchema.parse(mediaJson),
  markdownPreview: markdownPreviewSchema.parse(markdownPreviewJson),
};

/**
 * Markdown source of every seeded lesson (`apps/api/internal/seed/testdata`), the real content the
 * editor must open and save back byte for byte.
 */
export const seedMarkdown = z
  .array(z.object({ key: z.string().min(1), source: z.string().min(1) }))
  .min(1)
  .parse(seedMarkdownJson);

export const ids = {
  stage: stageDetail.id,
  v1: stageVersion.id,
  v2: stageVersionDraft.id,
  basic: "01990000-0000-7000-8000-000000000201",
  hasDraft: "0199ffff-0000-7000-8000-000000000001",
};
