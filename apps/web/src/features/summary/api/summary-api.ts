import { http } from "@/shared/api/http";
import {
  classMatrixSchema,
  learnerSummarySchema,
  memberStageSchema,
  reviewSchema,
  type ClassMatrix,
  type LearnerSummary,
  type MemberStage,
  type StageReview,
} from "../model/schemas";

const classPath = (classId: string) => `/classes/${encodeURIComponent(classId)}`;
const memberStagePath = (classId: string, memberId: string, stageVersionId: string) =>
  `${classPath(classId)}/members/${encodeURIComponent(memberId)}/stages/${encodeURIComponent(stageVersionId)}`;

/** One member's stage in a class, as a matrix cell addresses it. */
export interface StageTarget {
  memberId: string;
  stageVersionId: string;
}

export const summaryApi = {
  /** The signed-in student's own summary; 403 when they do not (or no longer) learn the class. */
  learner: (classId: string, signal?: AbortSignal): Promise<LearnerSummary> =>
    http(`/me/classes/${encodeURIComponent(classId)}/summary`, { schema: learnerSummarySchema, signal }),
  /** 403 for a teacher of another class, 404 when there is no such class. */
  matrix: (classId: string, signal?: AbortSignal): Promise<ClassMatrix> =>
    http(`${classPath(classId)}/summary`, { schema: classMatrixSchema, signal }),
  /** 404 when the member or the stage version is not part of the class. */
  memberStage: (classId: string, { memberId, stageVersionId }: StageTarget, signal?: AbortSignal): Promise<MemberStage> =>
    http(`${memberStagePath(classId, memberId, stageVersionId)}/summary`, { schema: memberStageSchema, signal }),
  /** Writes or replaces the review; 422 with the API's message on an empty or too long body. */
  saveReview: (classId: string, { memberId, stageVersionId }: StageTarget, body: string): Promise<StageReview> =>
    http(`${memberStagePath(classId, memberId, stageVersionId)}/review`, { method: "PUT", body: { body }, schema: reviewSchema }),
};

export const summaryKeys = {
  learner: (classId: string) => ["learner-summary", classId] as const,
  /** Prefix of the summary matrix of every class. */
  matrices: ["class-summary"] as const,
  matrix: (classId: string) => ["class-summary", classId] as const,
  /** Prefix of every member's stage summary, in every class. */
  memberStages: ["member-stage-summary"] as const,
  memberStage: (classId: string, { memberId, stageVersionId }: StageTarget) =>
    ["member-stage-summary", classId, memberId, stageVersionId] as const,
};
