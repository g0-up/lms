import { z } from "zod";
import { isoDate, roleSchema, userStatusSchema } from "@/shared/api/schemas";

/** Class start/end dates are calendar days ("2026-10-05"), not timestamps. */
const day = z.iso.date();
const count = z.number().int().nonnegative();
const id = z.string().min(1);

export const classStatusSchema = z.enum(["draft", "active", "ended"]);
export const memberStatusSchema = z.enum(["active", "dropped", "completed"]);
export const inviteStatusSchema = z.enum(["queued", "sent", "failed"]);
export const inviteKindSchema = z.enum(["invite", "added", "resend"]);

/** `GET /classes` item; `memberCount` counts active members only. */
export const classListItemSchema = z.object({
  id,
  code: z.string(),
  name: z.string(),
  status: classStatusSchema,
  courseName: z.string(),
  courseVersionNo: z.number().int().positive(),
  teacherName: z.string(),
  startDate: day,
  endDate: day,
  memberCount: count,
});
export const classListSchema = z.object({ items: z.array(classListItemSchema) });
export type ClassListItem = z.infer<typeof classListItemSchema>;

/** `GET /classes/{id}` and the result of create, update, activate and end. */
export const classDetailSchema = z.object({
  id,
  code: z.string(),
  name: z.string(),
  status: classStatusSchema,
  startDate: day,
  endDate: day,
  teacher: z.object({ id, name: z.string() }),
  courseVersion: z.object({
    id,
    courseId: id,
    courseName: z.string(),
    versionNo: z.number().int().positive(),
    stageCount: count,
    lessonCount: count,
  }),
  memberCount: count,
  activatedAt: isoDate.nullable(),
  endedAt: isoDate.nullable(),
});
export type ClassDetail = z.infer<typeof classDetailSchema>;

/**
 * Flat `MemberDTO`; `id` is `class_members.id`. The invite fields are absent while the member
 * has no invitation. Strict on purpose: a stray field such as a temporary password must fail the
 * parse rather than reach the UI.
 */
export const memberSchema = z.strictObject({
  id,
  userId: id,
  email: z.string(),
  fullName: z.string(),
  accountStatus: userStatusSchema,
  memberStatus: memberStatusSchema,
  tempPasswordExpiresAt: isoDate.nullish(),
  inviteStatus: inviteStatusSchema.nullish(),
  inviteKind: inviteKindSchema.nullish(),
  inviteAttempts: count.nullish(),
  inviteLastError: z.string().nullish(),
  invitedAt: isoDate.nullish(),
  lastLoginAt: isoDate.nullish(),
  lastActiveAt: isoDate.nullish(),
  joinedAt: isoDate,
  droppedAt: isoDate.nullish(),
});
export type Member = z.infer<typeof memberSchema>;

export const membersSchema = z.object({ items: z.array(memberSchema) });

/** `POST /classes/{id}/invitations`: `invited` created the account, `added` reused an existing one. */
export const inviteResultSchema = z.object({ kind: z.enum(["invited", "added"]), member: memberSchema });
export type InviteResult = z.infer<typeof inviteResultSchema>;

export const resendResultSchema = z.object({ member: memberSchema });

/** `UserDTO` of the admin user endpoints. */
export const userSchema = z.object({
  id,
  name: z.string(),
  email: z.string(),
  role: roleSchema,
  status: userStatusSchema,
});
export const usersSchema = z.object({ items: z.array(userSchema) });
export type UserItem = z.infer<typeof userSchema>;

/** `GET /courses`, only what the course version selects need. */
export const courseListSchema = z.object({
  items: z.array(
    z.object({
      id,
      code: z.string(),
      name: z.string(),
      versions: z.array(
        z.object({
          id,
          versionNo: z.number().int().positive(),
          status: z.enum(["draft", "published", "archived"]),
          publishedAt: isoDate.nullish(),
        }),
      ),
    }),
  ),
});
export type CourseList = z.infer<typeof courseListSchema>;

/** `GET /dashboard`, only the per-class average shown in the class list. */
export const classProgressSchema = z.object({
  classes: z.array(z.object({ id, avgPercent: z.number().int().min(0).max(100) })),
});
export type ClassProgress = z.infer<typeof classProgressSchema>;
