/** Test-only helpers for the grading pages and the class homework and quiz tabs; nothing in the app imports this module. */
import type { QueryClient } from "@tanstack/react-query";
import type { RequestHandler } from "msw";
import { beforeAll } from "vitest";
import { server, setSession, users } from "@/shared/test/msw";
import { renderRoutes } from "@/shared/test/render";
import { gradingHandlers, homeworkTabHandlers, quizReviewHandlers } from "../api/msw-handlers";
import { routes, teachRoutes } from "../routes";

export interface RenderGradingOptions {
  /** Grader signed in; their area (`/teach` or `/admin`) is the one routed. Defaults to the teacher. */
  role?: "teacher" | "admin";
  /** Take precedence over the golden handlers. */
  overrides?: RequestHandler[];
  queryClient?: QueryClient;
}

/** Renders the grader's area at `path` with the grading pages, against the golden API. */
export function renderGrading(path: string, { role = "teacher", overrides = [], queryClient }: RenderGradingOptions = {}) {
  setSession(role === "admin" ? users.admin : users.teacher);
  server.use(...gradingHandlers, ...homeworkTabHandlers, ...quizReviewHandlers);
  if (overrides.length > 0) server.use(...overrides);
  const area = role === "admin" ? { path: "/admin", children: routes } : { path: "/teach", children: teachRoutes };
  return renderRoutes([area], { initialEntries: [path], withToaster: true, queryClient });
}

/**
 * jsdom lacks the pointer-capture, scroll and resize-observer APIs Radix Select and Checkbox
 * call; register once per test file that renders them.
 */
export function polyfillRadix() {
  beforeAll(() => {
    const proto: Partial<Pick<HTMLElement, "hasPointerCapture" | "releasePointerCapture" | "scrollIntoView">> =
      window.HTMLElement.prototype;
    proto.hasPointerCapture ??= () => false;
    proto.releasePointerCapture ??= () => undefined;
    proto.scrollIntoView ??= () => undefined;
    const win: { ResizeObserver?: typeof ResizeObserver } = window;
    win.ResizeObserver ??= class {
      observe() { /* jsdom has no layout to observe */ }
      unobserve() { /* idem */ }
      disconnect() { /* idem */ }
    };
  });
}
