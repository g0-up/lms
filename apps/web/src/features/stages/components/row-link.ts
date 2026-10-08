import type { MouseEvent } from "react";

/**
 * Click handler body for a clickable table row: follows the row's link unless the click landed
 * on a control of its own (a link, button or input) or the user is selecting text.
 */
export function navigateFromRow(event: MouseEvent<HTMLElement>, go: () => void): void {
  const target = event.target as HTMLElement;
  if (target.closest("a, button, input, select, textarea, [role=button]")) return;
  if (window.getSelection()?.toString()) return;
  go();
}
