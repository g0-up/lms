import type { RouteObject } from "react-router";
import type { RouteHandle } from "@/shared/lib/route";

// Paths are relative to the area prefix added by app/router.tsx; see the contract there.
export const routes: RouteObject[] = [
  { path: "courses", lazy: () => import("./pages/courses-page"), handle: { title: "Khóa học" } satisfies RouteHandle },
  {
    path: "courses/:courseId",
    lazy: () => import("./pages/course-detail-page"),
    handle: { title: "Khóa học" } satisfies RouteHandle,
  },
];
