import { cn } from "@/shared/lib/cn";

/** GoUp mark: a bar in the current text colour, a crimson pill and a blue-100 block. Decorative. */
export function BrandMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 28 28" aria-hidden="true" focusable="false" className={cn("block size-7 shrink-0", className)}>
      <rect width="12" height="28" fill="currentColor" />
      <rect x="16" y="4" width="12" height="8" rx="4" fill="var(--accent-crimson)" />
      <rect x="16" y="16" width="12" height="12" fill="var(--surface-blue-100)" />
    </svg>
  );
}
