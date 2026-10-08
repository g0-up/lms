/** Test-only helpers for the report pages and panel; nothing in the app imports this module. */
import type { QueryClient } from "@tanstack/react-query";
import type { RequestHandler } from "msw";
import { beforeAll } from "vitest";
import { server, setSession, users } from "@/shared/test/msw";
import { renderRoutes } from "@/shared/test/render";
import { reportsHandlers, teachHandlers } from "../api/msw-handlers";
import { teachRoutes } from "../routes";

export const teachClassPath = (classId: string) => `/teach/classes/${classId}`;

/**
 * Renders the teacher area at `path` as the signed-in teacher, against the golden API.
 * `overrides` take precedence over the default handlers.
 */
export function renderTeach(
  path: string,
  { overrides = [], queryClient }: { overrides?: RequestHandler[]; queryClient?: QueryClient } = {},
) {
  setSession(users.teacher);
  server.use(...reportsHandlers, ...teachHandlers);
  if (overrides.length > 0) server.use(...overrides);
  return renderRoutes([{ path: "/teach", children: teachRoutes }], {
    initialEntries: [path],
    withToaster: true,
    queryClient,
  });
}

/** Lets a test hold a response until it has checked the loading state. */
export function gate() {
  let open: () => void = () => undefined;
  const opened = new Promise<void>((resolve) => {
    open = resolve;
  });
  return { wait: () => opened, open: () => { open(); } };
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
