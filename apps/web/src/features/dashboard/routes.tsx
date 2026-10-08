import type { RouteObject } from "react-router";
import type { RouteHandle } from "@/shared/lib/route";

// Paths are relative to the area prefix added by app/router.tsx; see the contract there.
export const routes: RouteObject[] = [
  {
    index: true,
    lazy: () => import("./pages/dashboard-page"),
    handle: { title: "Tổng quan" } satisfies RouteHandle,
  },
];
