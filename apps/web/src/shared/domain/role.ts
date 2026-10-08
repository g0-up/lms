export const ROLES = ["admin", "teacher", "student"] as const;

export type Role = (typeof ROLES)[number];

const HOME: Record<Role, string> = { admin: "/admin", teacher: "/teach", student: "/learn" };

export const ROLE_VI: Record<Role, string> = { admin: "Admin", teacher: "Giảng viên", student: "Học viên" };

/** Landing path of each role; one role per user. */
export function homeOf(role: Role): string {
  return HOME[role];
}
