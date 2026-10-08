/** Test-only helpers for the class pages; nothing in the app imports this module. */
import type { RequestHandler } from "msw";
import { beforeAll } from "vitest";
import { server, setSession, users } from "@/shared/test/msw";
import { renderRoutes } from "@/shared/test/render";
import { classesHandlers, seedDb, type ClassesDb } from "../api/msw-handlers";
import { routes } from "../routes";

export const classPath = (classId: string, tab?: string) =>
  `/admin/classes/${classId}${tab ? `?tab=${tab}` : ""}`;

/**
 * Renders the admin class pages at `path` as the signed-in admin, against an in-memory API seeded
 * from the golden fixtures. `overrides` take precedence over the default handlers; `db` exposes the
 * data so a test can arrange it before rendering or check what a mutation changed.
 */
export function renderClasses(
  path: string,
  { overrides = [], db = seedDb() }: { overrides?: RequestHandler[]; db?: ClassesDb } = {},
) {
  setSession(users.admin);
  server.use(...classesHandlers(db));
  if (overrides.length > 0) server.use(...overrides);
  const view = renderRoutes([{ path: "/admin", children: routes }], { initialEntries: [path], withToaster: true });
  return { ...view, db };
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
