import type { RouteObject } from "react-router";
import type { RouteHandle } from "@/shared/lib/route";

// Paths are relative to the area prefix added by app/router.tsx; see the contract there.
export const routes: RouteObject[] = [
  {
    path: "classes/:classId/summary",
    lazy: () => import("./pages/learn-summary-page"),
    handle: { title: "Tổng kết khóa học" } satisfies RouteHandle,
  },
];
