import type { ComponentType } from "react";
import { createContext, type LazyRouteFunction, type RouteObject, type UIMatch } from "react-router";
import type { User } from "@/shared/api/schemas";

/** Signed-in user, set by the app's auth middleware; loaders/actions read it with `context.get(userContext)`. */
export const userContext = createContext<User>();

/**
 * `handle` shape read by the app's RouteAnnouncer: `title` becomes "{title} · GoUp LMS" and is
 * announced to screen readers after navigation. A function receives the route's loader data.
 */
export interface RouteHandle {
  title?: string | ((loaderData: unknown) => string);
}

/** Title of the deepest matched route that declares one. */
export function titleFromMatches(matches: readonly UIMatch[]): string | undefined {
  for (const match of [...matches].reverse()) {
    const title = (match.handle as RouteHandle | undefined)?.title;
    if (typeof title === "string") return title;
    if (typeof title === "function") return title(match.loaderData);
  }
  return undefined;
}

/**
 * Lazy route for a page module with a `default` export. React Router's `lazy` only reads named
 * route properties (`Component`, `loader`, ...), so a bare `() => import(page)` whose module
 * exports only `default` would render nothing.
 */
export function lazyPage(load: () => Promise<{ default: ComponentType }>): LazyRouteFunction<RouteObject> {
  return async () => ({ Component: (await load()).default });
}
