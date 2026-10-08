import { http } from "@/shared/api/http";
import {
  courseDetailSchema,
  courseListSchema,
  courseVersionSchema,
  deleteCourseVersionSchema,
  type CourseDetail,
  type CourseList,
  type CourseVersion,
} from "../model/schemas";

const enc = encodeURIComponent;
const versionPath = (vid: string) => `/course-versions/${enc(vid)}`;

export interface NewCourse {
  code: string;
  name: string;
}

export const coursesApi = {
  list: (signal?: AbortSignal): Promise<CourseList> => http("/courses", { schema: courseListSchema, signal }),
  /** Creates the course with an empty draft v1. */
  create: (body: NewCourse): Promise<CourseDetail> =>
    http("/courses", { method: "POST", body, schema: courseDetailSchema }),
  detail: (courseId: string, signal?: AbortSignal): Promise<CourseDetail> =>
    http(`/courses/${enc(courseId)}`, { schema: courseDetailSchema, signal }),
  version: (vid: string, signal?: AbortSignal): Promise<CourseVersion> =>
    http(versionPath(vid), { schema: courseVersionSchema, signal }),
  /** Replaces the draft's ordered stage list; every id must be a published stage version. */
  setStages: (vid: string, stageVersionIds: string[]): Promise<CourseVersion> =>
    http(`${versionPath(vid)}/stages`, { method: "PUT", body: { stageVersionIds }, schema: courseVersionSchema }),
  clone: (vid: string): Promise<CourseVersion> =>
    http(`${versionPath(vid)}/clone`, { method: "POST", schema: courseVersionSchema }),
  publish: (vid: string): Promise<CourseVersion> =>
    http(`${versionPath(vid)}/publish`, { method: "POST", schema: courseVersionSchema }),
  archive: (vid: string): Promise<CourseVersion> =>
    http(`${versionPath(vid)}/archive`, { method: "POST", schema: courseVersionSchema }),
  remove: (vid: string): Promise<{ courseDeleted: boolean }> =>
    http(versionPath(vid), { method: "DELETE", schema: deleteCourseVersionSchema }),
};

/** `version` is also the prefix of the stage feature's apply-preview key, so invalidating it refreshes both. */
export const courseKeys = {
  list: ["courses"] as const,
  detail: (courseId: string) => ["course", courseId] as const,
  version: (vid: string) => ["course-version", vid] as const,
};

/** Keys owned by the stages and dashboard features that course actions make stale (prefixes). */
export const relatedKeys = {
  dashboard: ["dashboard"] as const,
  stages: ["stages"] as const,
  stageDetails: ["stage"] as const,
  stageVersions: ["stage-version"] as const,
};
