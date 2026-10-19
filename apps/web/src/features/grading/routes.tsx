import type { RouteObject } from "react-router";
import type { RouteHandle } from "@/shared/lib/route";

// Paths are relative to the area prefix added by app/router.tsx; see the contract there.
// The same pages serve both areas: links stay in the area they were opened from.
const gradingRoutes: RouteObject[] = [
  { path: "grading", lazy: () => import("./pages/grading-page"), handle: { title: "Chấm bài" } satisfies RouteHandle },
  {
    path: "grading/:submissionId",
    lazy: () => import("./pages/grade-submission-page"),
    handle: { title: "Chấm bài" } satisfies RouteHandle,
  },
  {
    path: "grading/quiz/:attemptId",
    lazy: () => import("./pages/review-attempt-page"),
    handle: { title: "Chấm bài" } satisfies RouteHandle,
  },
];

/** Admin area (`/admin`). */
export const routes: RouteObject[] = gradingRoutes;

/** Teacher area (`/teach`). */
export const teachRoutes: RouteObject[] = gradingRoutes;
