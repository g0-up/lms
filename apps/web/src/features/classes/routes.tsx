import type { RouteObject } from "react-router";
import type { RouteHandle } from "@/shared/lib/route";

// Paths are relative to the area prefix added by app/router.tsx; see the contract there.
export const routes: RouteObject[] = [
  { path: "classes", lazy: () => import("./pages/classes-page"), handle: { title: "Lớp học" } satisfies RouteHandle },
  {
    path: "classes/:classId",
    lazy: () => import("./pages/class-detail-page"),
    handle: { title: "Lớp" } satisfies RouteHandle,
  },
];
