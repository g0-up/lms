import type { RouteObject } from "react-router";
import type { RouteHandle } from "@/shared/lib/route";

// Paths are relative to the area prefix added by app/router.tsx; see the contract there.
export const routes: RouteObject[] = [
  { index: true, lazy: () => import("./pages/learn-page"), handle: { title: "Lớp của tôi" } satisfies RouteHandle },
  {
    path: "classes/:classId",
    lazy: () => import("./pages/learn-class-page"),
    handle: { title: "Lộ trình lớp" } satisfies RouteHandle,
  },
  {
    path: "classes/:classId/lessons/:lessonId",
    lazy: () => import("./pages/lesson-page"),
    handle: { title: "Học liệu" } satisfies RouteHandle,
  },
];
