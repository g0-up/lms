import { cn } from "@/shared/lib/cn";

export interface ProgressBarProps {
  /** Whole percent 0–100. */
  value: number;
  size?: "default" | "lg";
  className?: string;
}

/** Progress with its number: one image to assistive technology ("{pct}% hoàn thành"). */
export function ProgressBar({ value, size = "default", className }: ProgressBarProps) {
  const pct = Math.min(100, Math.max(0, Math.round(value)));
  const lg = size === "lg";
  return (
    <div role="img" aria-label={`${String(pct)}% hoàn thành`} className={cn("flex min-w-[90px] items-center gap-2", className)}>
      <div className={cn("flex-1 overflow-hidden bg-surface-blue-100", lg ? "h-2.5" : "h-1.5")}>
        <i
          className="block h-full bg-navy-700 transition-[width] duration-300 ease-brand"
          style={{ width: `${String(pct)}%` }}
        />
      </div>
      <span
        aria-hidden="true"
        className={cn("text-right text-ink tabular-nums", lg ? "min-w-[52px] text-lg font-semibold" : "min-w-[38px] text-sm")}
      >
        {pct}%
      </span>
    </div>
  );
}
