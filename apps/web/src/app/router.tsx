/**
 * Route tree of the web app.
 *
 * Contract for feature folders (`src/features/<feature>/routes.tsx`):
 * - Export `routes: RouteObject[]` (reports also exports `teachRoutes`) with paths RELATIVE to the
 *   area below; never repeat the `/admin`, `/teach` or `/learn` prefix.
 *     /admin  ← dashboard.routes, stages.routes, courses.routes, classes.routes, reports.routes
 *              (authMiddleware + requireRole('admin'), rendered in AppShell)
 *     /teach  ← reports.teachRoutes  (requireRole('teacher'), AppShell)
 *     /learn  ← learning.routes      (requireRole('student'), AppShell)
 *   The area root page is `{ index: true, ... }`.
 * - Load pages lazily. `lazy: () => import('./pages/x-page')` needs the module to export
 *   `Component`; for a `default` export use `lazy: lazyPage(() => import('./pages/x-page'))`
 *   from `@/shared/lib/route`.
 * - `handle: { title }` (string, or `(loaderData) => string`) sets "{title} · GoUp LMS" and is
 *   announced after navigation; a page with a data-driven title may call `useDocumentTitle`.
 * - Data comes from React Query inside the page; loaders are optional. Loaders/actions can read
 *   the signed-in user with `context.get(userContext)` from `@/shared/lib/route`.
 * - Unknown paths inside an area render NotFound within AppShell (a `*` child added here).
 *
 * File ownership: feature work only adds files under `src/features/<feature>/**` and edits that
 * feature's `routes.tsx`. `src/app`, `src/shared` and `src/styles` belong to the foundation; a
 * feature that needs a change there asks for it instead of editing in place.
 */
import type { QueryClient } from "@tanstack/react-query";
import type { RouteObject } from "react-router";
import * as auth from "@/features/auth";
import * as classes from "@/features/classes";
import * as courses from "@/features/courses";
import * as dashboard from "@/features/dashboard";
import * as learning from "@/features/learning";
import * as reports from "@/features/reports";
import * as stages from "@/features/stages";
import type { RouteHandle } from "@/shared/lib/route";
import { AppShell } from "./layouts/AppShell";
import { AuthLayout } from "./layouts/AuthLayout";
import { RootLayout } from "./layouts/RootLayout";
import { createMiddleware } from "./middleware";
import { NotFound } from "./not-found";
import { RouteError } from "./route-error";

const notFoundRoute: RouteObject = {
  path: "*",
  Component: NotFound,
  handle: { title: "Không tìm thấy trang" } satisfies RouteHandle,
};

export function buildRoutes(queryClient: QueryClient): RouteObject[] {
  const { authMiddleware, requireRole, requirePasswordChange, guestMiddleware, indexMiddleware } =
    createMiddleware(queryClient);

  return [
    {
      id: "root",
      Component: RootLayout,
      ErrorBoundary: RouteError,
      HydrateFallback: () => null,
      children: [
        { index: true, middleware: [indexMiddleware] },
        {
          Component: AuthLayout,
          children: [
            { middleware: [guestMiddleware], children: auth.routes },
            ...auth.publicRoutes,
            { middleware: [authMiddleware, requirePasswordChange], children: auth.sessionRoutes },
          ],
        },
        {
          path: "admin",
          middleware: [authMiddleware, requireRole("admin")],
          Component: AppShell,
          children: [
            ...dashboard.routes,
            ...stages.routes,
            ...courses.routes,
            ...classes.routes,
            ...reports.routes,
            notFoundRoute,
          ],
        },
        {
          path: "teach",
          middleware: [authMiddleware, requireRole("teacher")],
          Component: AppShell,
          children: [...reports.teachRoutes, notFoundRoute],
        },
        {
          path: "learn",
          middleware: [authMiddleware, requireRole("student")],
          Component: AppShell,
          children: [...learning.routes, notFoundRoute],
        },
        { Component: AuthLayout, children: [notFoundRoute] },
      ],
    },
  ];
}
