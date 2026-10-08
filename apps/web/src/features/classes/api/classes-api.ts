import { http } from "@/shared/api/http";
import {
  classDetailSchema,
  classListSchema,
  classProgressSchema,
  courseListSchema,
  inviteResultSchema,
  memberSchema,
  membersSchema,
  resendResultSchema,
  userSchema,
  usersSchema,
  type ClassDetail,
  type ClassListItem,
  type ClassProgress,
  type CourseList,
  type InviteResult,
  type Member,
  type UserItem,
} from "../model/schemas";

const classPath = (classId: string) => `/classes/${encodeURIComponent(classId)}`;
const memberPath = (classId: string, memberId: string) =>
  `${classPath(classId)}/members/${encodeURIComponent(memberId)}`;

export interface CreateClassBody {
  code: string;
  name: string;
  courseVersionId: string;
  startDate: string;
  endDate: string;
  teacherId: string;
}

/** PATCH body: only the fields that changed. */
export type UpdateClassBody = Partial<Omit<CreateClassBody, "code">>;

export interface InviteBody {
  email: string;
  fullName?: string;
}

export const classesApi = {
  list: async (signal?: AbortSignal): Promise<ClassListItem[]> =>
    (await http("/classes", { schema: classListSchema, signal })).items,
  get: (classId: string, signal?: AbortSignal): Promise<ClassDetail> =>
    http(classPath(classId), { schema: classDetailSchema, signal }),
  create: (body: CreateClassBody): Promise<ClassDetail> =>
    http("/classes", { method: "POST", body, schema: classDetailSchema }),
  update: (classId: string, body: UpdateClassBody): Promise<ClassDetail> =>
    http(classPath(classId), { method: "PATCH", body, schema: classDetailSchema }),
  activate: (classId: string): Promise<ClassDetail> =>
    http(`${classPath(classId)}/activate`, { method: "POST", schema: classDetailSchema }),
  end: (classId: string): Promise<ClassDetail> =>
    http(`${classPath(classId)}/end`, { method: "POST", schema: classDetailSchema }),
  /** Members who left are included: the students tab lists them greyed out. */
  members: async (classId: string, signal?: AbortSignal): Promise<Member[]> =>
    (await http(`${classPath(classId)}/members?includeDropped=true`, { schema: membersSchema, signal })).items,
  invite: (classId: string, body: InviteBody): Promise<InviteResult> =>
    http(`${classPath(classId)}/invitations`, { method: "POST", body, schema: inviteResultSchema }),
  resend: async (classId: string, memberId: string): Promise<Member> =>
    (await http(`${memberPath(classId, memberId)}/resend`, { method: "POST", schema: resendResultSchema })).member,
  removeMember: (classId: string, memberId: string): Promise<Member> =>
    http(memberPath(classId, memberId), { method: "DELETE", schema: memberSchema }),
  /** Teachers who can take a class: the API refuses a disabled one. */
  teachers: async (signal?: AbortSignal): Promise<UserItem[]> =>
    (await http("/users?role=teacher&status=active", { schema: usersSchema, signal })).items,
  disableUser: (userId: string): Promise<UserItem> =>
    http(`/users/${encodeURIComponent(userId)}/disable`, { method: "POST", schema: userSchema }),
  enableUser: (userId: string): Promise<UserItem> =>
    http(`/users/${encodeURIComponent(userId)}/enable`, { method: "POST", schema: userSchema }),
  courses: (signal?: AbortSignal): Promise<CourseList> => http("/courses", { schema: courseListSchema, signal }),
  /** Per-class average progress, taken from the admin dashboard. */
  progress: (signal?: AbortSignal): Promise<ClassProgress> =>
    http("/dashboard", { schema: classProgressSchema, signal }),
};

export const classesKeys = {
  list: ["classes"] as const,
  detail: (classId: string) => ["class", classId] as const,
  members: (classId: string) => ["members", classId] as const,
  teachers: ["teachers"] as const,
  publishedVersions: ["course-versions-published"] as const,
  /** Under the dashboard prefix so every dashboard invalidation refreshes it too. */
  progress: ["dashboard", "class-progress"] as const,
};
