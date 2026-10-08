import type { ReactNode } from "react";

/**
 * Secondary action on the left, the page's gradient submit on the right. When the row wraps on a
 * narrow screen the submit keeps to the right edge.
 */
export function FormActions({ children }: { children: ReactNode }) {
  return (
    <div className="mt-2 flex flex-wrap items-center justify-between gap-3 [&>:last-child]:ml-auto">{children}</div>
  );
}
