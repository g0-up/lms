import { useEffect, useRef } from "react";
import { useLocation, useMatches } from "react-router";
import { APP_NAME, formatDocumentTitle } from "@/shared/hooks/use-document-title";
import { titleFromMatches } from "@/shared/lib/route";

/**
 * Keeps `document.title` in sync with the deepest `handle.title` ("{title} · GoUp LMS") and, after
 * each client-side navigation, announces the new title and moves focus to the page's h1 (or to
 * `#main` while the page has none), so keyboard and screen-reader users start at the content.
 *
 * Rendered before the routed content so a page that sets a data-driven title with
 * `useDocumentTitle` runs its effect later and wins.
 */
export function RouteAnnouncer() {
  const matches = useMatches();
  const { pathname } = useLocation();
  const title = titleFromMatches(matches);
  const liveRef = useRef<HTMLParagraphElement>(null);
  const previousPath = useRef<string | null>(null);

  useEffect(() => {
    document.title = formatDocumentTitle(title);
  }, [title]);

  useEffect(() => {
    const isFirstRender = previousPath.current === null;
    const samePath = previousPath.current === pathname;
    previousPath.current = pathname;
    if (isFirstRender || samePath) return;

    if (liveRef.current) liveRef.current.textContent = title ?? APP_NAME;
    const target = document.querySelector<HTMLElement>("main h1") ?? document.getElementById("main");
    if (target) {
      if (!target.hasAttribute("tabindex")) target.tabIndex = -1;
      target.focus();
    }
  }, [pathname, title]);

  return <p ref={liveRef} className="sr-only" aria-live="polite" aria-atomic="true" />;
}
