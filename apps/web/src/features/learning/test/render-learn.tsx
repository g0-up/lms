/** Test-only helpers for the learning pages; nothing in the app imports this module. */
import type { QueryClient } from "@tanstack/react-query";
import type { RequestHandler } from "msw";
import { toast } from "sonner";
import { server, setSession, users } from "@/shared/test/msw";
import { renderRoutes } from "@/shared/test/render";
import { learningHandlers, quizHandlers, quizState, type QuizState } from "../api/msw-handlers";
import { routes } from "../routes";

export const classPath = (classId: string) => `/learn/classes/${classId}`;
export const lessonPath = (classId: string, lessonId: string) => `${classPath(classId)}/lessons/${lessonId}`;

// jsdom has no layout: moving between quiz questions scrolls the card into view.
const proto: Partial<Element> = Element.prototype;
proto.scrollIntoView ??= () => undefined;

/**
 * Renders the learning area at `path` as a signed-in student, against the golden API. The quiz API runs over
 * `quiz` (a fresh `quizState()` by default), which the result exposes for the test to inspect.
 * `overrides` take precedence over the default handlers.
 */
export function renderLearn(
  path: string,
  {
    overrides = [],
    queryClient,
    quiz = quizState(),
  }: { overrides?: RequestHandler[]; queryClient?: QueryClient; quiz?: QuizState } = {},
) {
  // Sonner keeps toasts in a module store across tests; a new Toaster would replay them.
  toast.dismiss();
  setSession(users.student);
  server.use(...learningHandlers, ...quizHandlers(quiz));
  if (overrides.length > 0) server.use(...overrides);
  const view = renderRoutes([{ path: "/learn", children: routes }], {
    initialEntries: [path],
    withToaster: true,
    queryClient,
  });
  return { ...view, quiz };
}

/** Lets a test hold a response until it has checked the loading state. */
export function gate() {
  let open: () => void = () => undefined;
  const opened = new Promise<void>((resolve) => {
    open = resolve;
  });
  return { wait: () => opened, open: () => { open(); } };
}
