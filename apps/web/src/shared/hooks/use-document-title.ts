import { useEffect } from "react";

export const APP_NAME = "GoUp LMS";

export function formatDocumentTitle(title: string | null | undefined): string {
  return title ? `${title} · ${APP_NAME}` : APP_NAME;
}

/** Sets `document.title` to "{title} · GoUp LMS" while mounted. */
export function useDocumentTitle(title: string | null | undefined): void {
  useEffect(() => {
    document.title = formatDocumentTitle(title);
  }, [title]);
}
