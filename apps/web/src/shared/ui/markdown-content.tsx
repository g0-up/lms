import DOMPurify from "dompurify";
import { useMemo } from "react";
import { cn } from "@/shared/lib/cn";

let hookInstalled = false;

function ensureLinkHook(): void {
  if (hookInstalled) return;
  hookInstalled = true;
  DOMPurify.addHook("afterSanitizeAttributes", (node) => {
    if (node instanceof HTMLAnchorElement && node.getAttribute("target") === "_blank") {
      node.setAttribute("rel", "noopener noreferrer");
    }
  });
}

/** Sanitizes server-rendered lesson HTML; the only place raw HTML is rendered in the app. */
export function sanitizeHtml(html: string): string {
  ensureLinkHook();
  return DOMPurify.sanitize(html, { USE_PROFILES: { html: true }, ADD_ATTR: ["target"] });
}

/** Lesson typography, shared by the learner page, the editor and its preview so all three look alike. */
export const proseClass = [
  "max-w-[72ch] leading-[1.7] text-ink-2",
  "[&_h1]:mt-6 [&_h1]:mb-3 [&_h1]:font-sans [&_h1]:text-xl [&_h2]:mt-6 [&_h2]:mb-3 [&_h2]:text-lg [&_h3]:mt-6 [&_h3]:mb-3",
  "[&_p]:mb-3 [&_ul]:mb-3 [&_ul]:list-disc [&_ul]:pl-[22px] [&_ol]:mb-3 [&_ol]:list-decimal [&_ol]:pl-[22px]",
  "[&_pre]:mb-3 [&_pre]:overflow-x-auto [&_pre]:bg-surface-dark [&_pre]:p-4 [&_pre]:text-[13px] [&_pre]:leading-normal [&_pre]:text-on-accent",
  "[&_code]:bg-surface-blue-50 [&_code]:px-[5px] [&_code]:py-px [&_code]:text-navy-900",
  "[&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_pre_code]:text-inherit",
  "[&_blockquote]:mb-3 [&_blockquote]:border-l-3 [&_blockquote]:border-navy-700 [&_blockquote]:bg-surface-blue-50 [&_blockquote]:px-4 [&_blockquote]:py-2 [&_blockquote]:text-ink",
  "[&_a]:text-navy-700 [&_a]:underline [&_img]:h-auto [&_img]:max-w-full",
  "[&_h4]:mt-5 [&_h4]:mb-2 [&_h4]:font-semibold [&_del]:line-through [&_hr]:my-6 [&_hr]:border-line",
  "[&_table]:mb-3 [&_table]:block [&_table]:w-full [&_table]:overflow-x-auto [&_table]:border-collapse",
  "[&_th]:border [&_th]:border-line [&_th]:px-3 [&_th]:py-2 [&_th]:text-left [&_td]:border [&_td]:border-line [&_td]:px-3 [&_td]:py-2",
  "[&_img]:my-3 [&_img]:block [&_img]:rounded-md",
].join(" ");

export function MarkdownContent({ html, className }: { html: string; className?: string }) {
  const clean = useMemo(() => sanitizeHtml(html), [html]);
  return <div className={cn(proseClass, className)} dangerouslySetInnerHTML={{ __html: clean }} />;
}
