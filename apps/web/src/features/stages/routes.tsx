import type { RouteObject } from "react-router";
import type { RouteHandle } from "@/shared/lib/route";

// Paths are relative to the area prefix added by app/router.tsx; see the contract there.
export const routes: RouteObject[] = [
  { path: "stages", lazy: () => import("./pages/stages-page"), handle: { title: "Chặng" } satisfies RouteHandle },
  {
    path: "stages/:stageId",
    lazy: () => import("./pages/stage-detail-page"),
    handle: { title: "Chặng" } satisfies RouteHandle,
  },
  // One page for both: the editor and its styles load only with this chunk.
  {
    path: "stages/:stageId/versions/:versionId/lessons/new",
    lazy: () => import("./pages/lesson-editor-page"),
    handle: { title: "Soạn học liệu" } satisfies RouteHandle,
  },
  {
    path: "stages/:stageId/versions/:versionId/lessons/:lessonId/edit",
    lazy: () => import("./pages/lesson-editor-page"),
    handle: { title: "Soạn học liệu" } satisfies RouteHandle,
  },
];
