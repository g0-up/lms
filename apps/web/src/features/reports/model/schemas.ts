import { z } from "zod";
import { isoDate, userStatusSchema } from "@/shared/api/schemas";

/** Class start/end dates are calendar days ("2026-10-05"), not timestamps. */
const day = z.iso.date();
const count = z.number().int().nonnegative();
const percent = z.number().int().min(0).max(100);

export const classStatusSchema = z.enum(["draft", "active", "ended"]);
export const memberStatusSchema = z.enum(["active", "dropped", "completed"]);
export const inviteStatusSchema = z.enum(["queued", "sent", "failed"]);
export const sortSchema = z.enum(["name", "pct", "pct-desc", "activity", "activity-asc"]);

export type ReportSort = z.infer<typeof sortSchema>;

const reportClassSchema = z.object({
  id: z.string().min(1),
  code: z.string(),
  name: z.string(),
  status: classStatusSchema,
  courseName: z.string(),
  courseVersionNo: z.number().int().positive(),
  teacher: z.object({ id: z.string().min(1), name: z.string() }),
});

/** Latest invitation of the member; `status` is null while none was ever queued. */
const reportInviteSchema = z.object({
  kind: z.string(),
  status: inviteStatusSchema.nullable(),
  attempts: count,
  lastError: z.string().nullable(),
});

/** One member's line of the FR-40 report; `memberId` is `class_members.id`, the drilldown key. */
export const reportRowSchema = z.object({
  memberId: z.string().min(1),
  userId: z.string().min(1),
  name: z.string(),
  email: z.string(),
  accountStatus: userStatusSchema,
  memberStatus: memberStatusSchema,
  mustChangePassword: z.boolean(),
  invite: reportInviteSchema.optional(),
  stagePercents: z.array(
    z.object({ stageId: z.string().min(1), percent, requiredDone: count, requiredTotal: count }),
  ),
  percent,
  requiredDone: count,
  requiredTotal: count,
  lastLoginAt: isoDate.nullable(),
  lastActivityAt: isoDate.nullable(),
});
export type ReportRow = z.infer<typeof reportRowSchema>;

export const reportStageSchema = z.object({
  stageId: z.string().min(1),
  code: z.string(),
  name: z.string(),
  versionNo: z.number().int().positive(),
  position: z.number().int(),
  requiredTotal: count,
});
export type ReportStage = z.infer<typeof reportStageSchema>;

/**
 * `GET /classes/{id}/report`. `summary` always counts active members only, even when dropped
 * rows are shown, so the "{rows}/{total}" head and the KPIs never move with the checkbox.
 */
export const classReportSchema = z.object({
  class: reportClassSchema,
  stages: z.array(reportStageSchema),
  rows: z.array(reportRowSchema),
  summary: z.object({
    memberCount: count,
    activeCount: count,
    avgPercent: percent,
    notLoggedInCount: count,
    inactiveCount: count,
    belowCount: count,
    inactiveDays: count,
    belowPercent: percent,
  }),
  filter: z.object({
    notLoggedIn: z.boolean(),
    inactiveDays: count.nullable(),
    belowPercent: percent.nullable(),
    includeDropped: z.boolean(),
    sort: sortSchema,
  }),
  selfReported: z.literal(true),
});
export type ClassReport = z.infer<typeof classReportSchema>;

export const lessonStateSchema = z.enum(["completed", "opened", "not_opened"]);

/** `GET /classes/{id}/report/members/{mid}`: one member, lesson by lesson. */
export const memberReportSchema = z.object({
  class: reportClassSchema,
  member: reportRowSchema,
  stages: z.array(
    z.object({
      stageId: z.string().min(1),
      code: z.string(),
      name: z.string(),
      versionNo: z.number().int().positive(),
      percent,
      lessons: z.array(
        z.object({
          id: z.string().min(1),
          title: z.string(),
          type: z.enum(["markdown", "video"]),
          required: z.boolean(),
          position: z.number().int(),
          state: lessonStateSchema,
          firstOpenedAt: isoDate.nullable(),
          completedAt: isoDate.nullable(),
        }),
      ),
    }),
  ),
  selfReported: z.literal(true),
});
export type MemberReport = z.infer<typeof memberReportSchema>;

/** `GET /teach/classes`: classes of the signed-in teacher; `avgPercent` is null without active members. */
export const teachClassesSchema = z.object({
  items: z.array(
    z.object({
      id: z.string().min(1),
      code: z.string(),
      name: z.string(),
      status: classStatusSchema,
      startDate: day,
      endDate: day,
      courseName: z.string(),
      courseVersionNo: z.number().int().positive(),
      memberCount: count,
      avgPercent: percent.nullable(),
      notLoggedIn: count,
      inactiveOver7Days: count,
    }),
  ),
});
export type TeachClasses = z.infer<typeof teachClassesSchema>;
export type TeachClassItem = TeachClasses["items"][number];

/** The class header of the teacher page (`GET /classes/{id}`), only the fields it shows. */
export const teachClassSchema = z.object({
  id: z.string().min(1),
  code: z.string(),
  name: z.string(),
  status: classStatusSchema,
  startDate: day,
  endDate: day,
  courseVersion: z.object({ courseName: z.string(), versionNo: z.number().int().positive() }),
});
export type TeachClass = z.infer<typeof teachClassSchema>;
