import type { RouteObject } from "react-router";
import type { RouteHandle } from "@/shared/lib/route";

// Paths are relative to the area prefix added by app/router.tsx; see the contract there.
// Reports have no admin page of their own: the admin report is a tab of the class page.
export const routes: RouteObject[] = [];

/** Teacher area (`/teach`). */
export const teachRoutes: RouteObject[] = [
  { index: true, lazy: () => import("./pages/teach-classes-page"), handle: { title: "Lớp của tôi" } satisfies RouteHandle },
  {
    path: "classes/:classId",
    lazy: () => import("./pages/teach-class-page"),
    handle: { title: "Lớp" } satisfies RouteHandle,
  },
];
