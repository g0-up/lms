import type { RouteObject } from "react-router";
import type { RouteHandle } from "@/shared/lib/route";

/** Guest-only pages; the app sends a signed-in user to their home instead. */
export const routes: RouteObject[] = [
  { path: "login", lazy: () => import("./pages/login-page"), handle: { title: "Đăng nhập" } satisfies RouteHandle },
  { path: "forgot", lazy: () => import("./pages/forgot-page"), handle: { title: "Quên mật khẩu" } satisfies RouteHandle },
];

/** Reachable whether or not someone is signed in: the token, not the session, authorises it. */
export const publicRoutes: RouteObject[] = [
  {
    path: "reset-password",
    lazy: () => import("./pages/reset-password-page"),
    handle: { title: "Đặt lại mật khẩu" } satisfies RouteHandle,
  },
];

/** Needs a session that still has to replace its temporary password. */
export const sessionRoutes: RouteObject[] = [
  {
    path: "first-login",
    lazy: () => import("./pages/first-login-page"),
    handle: { title: "Đặt mật khẩu mới" } satisfies RouteHandle,
  },
];
