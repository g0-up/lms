/** Test-only helpers for the summary tab and the student's summary page; nothing in the app imports this module. */
import type { RequestHandler } from "msw";
import { beforeAll } from "vitest";
import { server, setSession, users } from "@/shared/test/msw";
import { renderRoutes } from "@/shared/test/render";
import { summaryHandlers } from "../api/msw-handlers";
import { SummaryTab } from "../components/summary-tab";
import type { MemberStage } from "../model/schemas";
import { routes } from "../routes";
import { fixtures } from "./golden";

export const classId = fixtures.matrix.class.id;
/** The tab without filter, as the admin class page passes it. */
export const TAB_PATH = `/admin/classes/${classId}?tab=summary`;
export const learnSummaryPath = (id = fixtures.learner.class.id) => `/learn/classes/${id}/summary`;

export interface RenderSummaryOptions {
  /** Take precedence over the golden handlers. */
  overrides?: RequestHandler[];
  /** What every drawer opens on; the reviewed golden by default. */
  memberStage?: MemberStage;
}

/** Renders the "Tổng kết" tab of an admin class page at `path`, against the golden API. */
export function renderSummaryTab(path = TAB_PATH, { overrides = [], memberStage }: RenderSummaryOptions = {}) {
  setSession(users.admin);
  server.use(...summaryHandlers({ memberStage }));
  // Later `use` calls win: overrides go last.
  if (overrides.length > 0) server.use(...overrides);
  return renderRoutes(
    [{ path: "/admin/classes/:classId", element: <SummaryTab classId={classId} basePath={TAB_PATH} /> }],
    { initialEntries: [path], withToaster: true },
  );
}

/** Renders the student's area at `path` with the summary page and a stand-in "Lớp của tôi". */
export function renderLearnSummary(path = learnSummaryPath(), { overrides = [] }: Pick<RenderSummaryOptions, "overrides"> = {}) {
  setSession(users.student);
  server.use(...summaryHandlers());
  if (overrides.length > 0) server.use(...overrides);
  return renderRoutes(
    [{ path: "/learn", children: [{ index: true, element: <h1>Lớp của tôi</h1> }, ...routes] }],
    { initialEntries: [path], withToaster: true },
  );
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
