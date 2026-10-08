import { z } from "zod";
import { isoDate } from "@/shared/api/schemas";

const id = z.string().min(1);
const versionNo = z.number().int().positive();
const count = z.number().int().nonnegative();

/**
 * A course version still on an older version of a stage. `currentVersionNo` and `latestPublishedNo` are the
 * stage's; `latestVersionId` is the stage version to apply.
 */
export const outdatedRowSchema = z.object({
  courseId: id,
  courseCode: z.string(),
  courseName: z.string(),
  courseVersionId: id,
  courseVersionNo: versionNo,
  stageId: id,
  stageCode: z.string(),
  stageName: z.string(),
  currentVersionNo: versionNo,
  latestVersionId: id,
  latestPublishedNo: versionNo,
});
export type OutdatedRow = z.infer<typeof outdatedRowSchema>;

export const dashboardClassSchema = z.object({
  id,
  code: z.string(),
  name: z.string(),
  status: z.enum(["draft", "active", "ended"]),
  courseName: z.string(),
  courseVersionNo: versionNo,
  memberCount: count,
  avgPercent: z.number().min(0).max(100),
});
export type DashboardClass = z.infer<typeof dashboardClassSchema>;

/** One audit log line; `actionLabel` is already translated by the API, `target.id` is null when detached. */
export const activitySchema = z.object({
  id,
  at: isoDate,
  actorName: z.string(),
  actionLabel: z.string(),
  summary: z.string(),
  target: z.object({ type: z.string(), id: id.nullable(), label: z.string() }),
});
export type Activity = z.infer<typeof activitySchema>;

/** `GET /dashboard`. */
export const dashboardSchema = z.object({
  kpis: z.object({ stages: count, courses: count, classes: count, students: count }),
  hints: z.object({ draftClasses: count, notLoggedIn: count, outdatedCourses: count, failedInvites: count }),
  outdated: z.array(outdatedRowSchema),
  classes: z.array(dashboardClassSchema),
  recentActivity: z.array(activitySchema),
});
export type Dashboard = z.infer<typeof dashboardSchema>;

/** The audit card shows the newest lines only. */
export const RECENT_ACTIVITY_LIMIT = 8;
