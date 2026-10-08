import { homeOf, type Role } from "@/shared/domain";

export interface NavItem {
  to: string;
  label: string;
}

export const NAV: Record<Role, readonly NavItem[]> = {
  admin: [
    { to: "/admin", label: "Tổng quan" },
    { to: "/admin/stages", label: "Chặng" },
    { to: "/admin/courses", label: "Khóa học" },
    { to: "/admin/classes", label: "Lớp học" },
  ],
  teacher: [{ to: "/teach", label: "Lớp của tôi" }],
  student: [{ to: "/learn", label: "Lớp của tôi" }],
};

/**
 * A nav item is current on its own path and below it; the role's home matches only exactly,
 * so "Tổng quan" is not current on /admin/stages.
 */
export function isNavCurrent(role: Role, itemTo: string, pathname: string): boolean {
  const path = pathname.length > 1 ? pathname.replace(/\/+$/, "") : pathname;
  if (path === itemTo) return true;
  return itemTo !== homeOf(role) && path.startsWith(`${itemTo}/`);
}
