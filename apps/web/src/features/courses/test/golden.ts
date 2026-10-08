/**
 * Fixtures for the course tests, straight from the API's golden JSON
 * (`apps/api/internal/features/{courses,stages}/testdata`). Every fixture goes through the same Zod
 * schema the app uses, so a contract change on the API turns these tests red. Plain data, no
 * network: `model/` tests may use it. Test-only: nothing in the app imports this module.
 */
import courseDetailJson from "../../../../../api/internal/features/courses/testdata/course_detail.json";
import courseListJson from "../../../../../api/internal/features/courses/testdata/course_list.json";
import courseVersionDraftJson from "../../../../../api/internal/features/courses/testdata/course_version_draft.json";
import courseVersionJson from "../../../../../api/internal/features/courses/testdata/course_version.json";
import deleteVersionJson from "../../../../../api/internal/features/courses/testdata/delete_version.json";
import draftExistsJson from "../../../../../api/internal/features/courses/testdata/error_draft_exists.json";
import inUseJson from "../../../../../api/internal/features/courses/testdata/error_in_use.json";
import validationJson from "../../../../../api/internal/features/courses/testdata/error_validation.json";
import versionImmutableJson from "../../../../../api/internal/features/courses/testdata/error_version_immutable.json";
import stageDetailJson from "../../../../../api/internal/features/stages/testdata/stage_detail.json";
import stageListJson from "../../../../../api/internal/features/stages/testdata/stage_list.json";
import { stageListSchema, type StageList } from "@/features/stages";
import { errorEnvelopeSchema } from "@/shared/api/schemas";
import {
  courseDetailSchema,
  courseListSchema,
  courseVersionSchema,
  deleteCourseVersionSchema,
  type CourseDetail,
  type CourseStage,
  type CourseVersion,
} from "../model/schemas";

/** Raw golden payloads, for the contract test. */
export const golden = {
  courseList: courseListJson,
  courseDetail: courseDetailJson,
  courseVersion: courseVersionJson,
  courseVersionDraft: courseVersionDraftJson,
  deleteVersion: deleteVersionJson,
};

export const errors = {
  /** 409 DRAFT_EXISTS on clone. */
  draftExists: errorEnvelopeSchema.parse(draftExistsJson),
  /** 409 IN_USE on deleting a version a class is attached to. */
  inUse: errorEnvelopeSchema.parse(inUseJson),
  /** 422 VALIDATION_FAILED on attaching an unpublished stage version. */
  validation: errorEnvelopeSchema.parse(validationJson),
  /** 409 VERSION_IMMUTABLE on editing a published version's stages. */
  versionImmutable: errorEnvelopeSchema.parse(versionImmutableJson),
};

const courseList = courseListSchema.parse(courseListJson);
/** BASIC with only v1 published (class basic01 attached, Database v1 outdated). */
const courseDetail = courseDetailSchema.parse(courseDetailJson);
const courseVersion = courseVersionSchema.parse(courseVersionJson);
/** BASIC v2, a draft cloned from v1: Database v1, no classes. */
const courseVersionDraft = courseVersionSchema.parse(courseVersionDraftJson);

/** The stage list as the course golden sees it: Database v1 published, v2 a draft. */
const stageList = stageListSchema.parse(stageListJson);

/** The same list after Database v2 was published (the versions of the stage golden detail). */
const stageListV2Published: StageList = stageListSchema.parse({
  items: stageListJson.items.map(({ draftVersionId: _draft, ...stage }) => ({
    ...stage,
    latestPublishedNo: 2,
    versions: stageDetailJson.versions,
  })),
});

const databaseV1 = courseVersionDraft.stages[0];
const databaseV2Id = stageList.items[0].versions.find((v) => v.versionNo === 2)?.id ?? "";
/** Database v2 as a course stage row. */
const databaseV2: CourseStage = { ...databaseV1, stageVersionId: databaseV2Id, stageVersionNo: 2, latestPublishedNo: 2 };

/** BASIC with v1 published and v2 a draft. */
const courseDetailWithDraft: CourseDetail = {
  ...courseDetail,
  versions: [
    {
      id: courseVersionDraft.id,
      versionNo: courseVersionDraft.versionNo,
      status: "draft",
      stageCount: courseVersionDraft.stages.length,
      outdatedStageCount: 0,
      classCount: 0,
      clonedFromVersionNo: courseVersionDraft.clonedFromVersionNo,
    },
    ...courseDetail.versions,
  ],
};

/** The draft with every stage removed. */
const courseVersionDraftEmpty: CourseVersion = { ...courseVersionDraft, stages: [] };
/** The draft still on Database v1 after v2 was published. */
const courseVersionDraftOutdated: CourseVersion = {
  ...courseVersionDraft,
  stages: [{ ...databaseV1, outdated: true, latestPublishedNo: 2 }],
};
/** The draft pointing at Database v2 while v2 is still a draft (the server refuses to publish it). */
const courseVersionDraftUnpublishedStage: CourseVersion = { ...courseVersionDraft, stages: [{ ...databaseV2, latestPublishedNo: 1 }] };

export const fixtures = {
  courseList,
  courseDetail,
  courseDetailWithDraft,
  courseVersion,
  courseVersionDraft,
  courseVersionDraftEmpty,
  courseVersionDraftOutdated,
  courseVersionDraftUnpublishedStage,
  deleteVersion: deleteCourseVersionSchema.parse(deleteVersionJson),
  stageList,
  stageListV2Published,
  /** Course stage rows the stage-list PUT can resolve, by stage version id. */
  stageRows: [databaseV1, databaseV2],
};

export const ids = {
  course: courseDetail.id,
  v1: courseVersion.id,
  v2: courseVersionDraft.id,
  database: databaseV1.stageId,
  databaseV1: databaseV1.stageVersionId,
  databaseV2: databaseV2Id,
  basic01: courseVersion.classes[0].classId,
};
