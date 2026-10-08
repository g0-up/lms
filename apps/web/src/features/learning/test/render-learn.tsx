/** Test-only helpers for the learning pages; nothing in the app imports this module. */
import type { QueryClient } from "@tanstack/react-query";
import type { RequestHandler } from "msw";
import { server, setSession, users } from "@/shared/test/msw";
import { renderRoutes } from "@/shared/test/render";
import { learningHandlers } from "../api/msw-handlers";
import { routes } from "../routes";

export const classPath = (classId: string) => `/learn/classes/${classId}`;
export const lessonPath = (classId: string, lessonId: string) => `${classPath(classId)}/lessons/${lessonId}`;

/**
 * Renders the learning area at `path` as a signed-in student, against the golden API.
 * `overrides` take precedence over the default handlers.
 */
export function renderLearn(
  path: string,
  { overrides = [], queryClient }: { overrides?: RequestHandler[]; queryClient?: QueryClient } = {},
) {
  setSession(users.student);
  server.use(...learningHandlers);
  if (overrides.length > 0) server.use(...overrides);
  return renderRoutes([{ path: "/learn", children: routes }], { initialEntries: [path], withToaster: true, queryClient });
}

/** Lets a test hold a response until it has checked the loading state. */
export function gate() {
  let open: () => void = () => undefined;
  const opened = new Promise<void>((resolve) => {
    open = resolve;
  });
  return { wait: () => opened, open: () => { open(); } };
}
