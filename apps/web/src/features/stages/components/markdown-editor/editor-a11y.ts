/**
 * Accessible names for editor internals MDXEditor renders without one and exposes no props for:
 * the nested editor in each table cell, CodeMirror's editable area, and the table's add-row and
 * add-column buttons. Only fills a missing `aria-label`, so an upstream fix wins.
 */
const LABELS: readonly (readonly [selector: string, label: string])[] = [
  [".mdxeditor-root-contenteditable td [contenteditable], .mdxeditor-root-contenteditable th [contenteditable]", "Ô bảng"],
  [".mdxeditor-root-contenteditable .cm-content", "Mã trong khối mã"],
  [".cm-content", "Mã nguồn markdown"],
  // CSS-module class names of the pinned version, prefix-matched so a rebuilt hash still matches.
  ['button[class*="_addRowButton_"]', "Thêm hàng"],
  ['button[class*="_addColumnButton_"]', "Thêm cột"],
];

export function labelEditorInternals(container: HTMLElement): void {
  for (const [selector, label] of LABELS) {
    for (const el of container.querySelectorAll<HTMLElement>(selector)) {
      if (!el.hasAttribute("aria-label")) el.setAttribute("aria-label", label);
    }
  }
}
