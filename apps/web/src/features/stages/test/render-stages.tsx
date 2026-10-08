/** Test-only helpers for the stage pages; nothing in the app imports this module. */
import type { QueryClient } from "@tanstack/react-query";
import type { RequestHandler } from "msw";
import "./dom-polyfills";
import { toast } from "sonner";
import { onTestFinished } from "vitest";
import { server, setSession, users } from "@/shared/test/msw";
import { renderRoutes } from "@/shared/test/render";
import { stagesHandlers, type StageWorld } from "../api/msw-handlers";
import { routes } from "../routes";

export const stageUrl = (stageId: string, vid?: string) => `/admin/stages/${stageId}${vid ? `?v=${vid}` : ""}`;

/**
 * Renders the stage area at `path` as a signed-in admin, against the golden API.
 * `overrides` take precedence over the default handlers.
 */
export function renderStages(
  path: string,
  { world, overrides = [], queryClient }: { world?: StageWorld; overrides?: RequestHandler[]; queryClient?: QueryClient } = {},
) {
  // Sonner keeps toasts in a module store across tests; a new Toaster would replay them.
  toast.dismiss();
  setSession(users.admin);
  server.use(...stagesHandlers(world));
  if (overrides.length > 0) server.use(...overrides);
  return renderRoutes([{ path: "/admin", children: routes }], { initialEntries: [path], withToaster: true, queryClient });
}

/** Lets a test hold a response until it has checked the loading state. */
export function gate() {
  let open: () => void = () => undefined;
  const opened = new Promise<void>((resolve) => {
    open = resolve;
  });
  return {
    wait: () => opened,
    open: () => {
      open();
    },
  };
}

export interface SeenRequest {
  method: string;
  path: string;
  headers: Headers;
  body: string;
}

/** Records every request the page sends during the current test (bodies as text). */
export function recordRequests() {
  const seen: SeenRequest[] = [];
  const listener = ({ request }: { request: Request }) => {
    const entry: SeenRequest = { method: request.method, path: new URL(request.url).pathname, headers: request.headers, body: "" };
    seen.push(entry);
    void request
      .clone()
      .text()
      .then((text) => {
        entry.body = text;
      })
      .catch(() => undefined);
  };
  server.events.on("request:start", listener);
  onTestFinished(() => {
    server.events.removeListener("request:start", listener);
  });
  return {
    seen,
    /** Requests matching a method and an `/api/v1`-relative path (or a full storage path). */
    of: (method: string, path: string) => seen.filter((r) => r.method === method && (r.path === `/api/v1${path}` || r.path === path)),
  };
}
