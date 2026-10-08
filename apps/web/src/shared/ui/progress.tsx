import { Progress as ProgressPrimitive } from "radix-ui";
import type { ComponentProps } from "react";
import { cn } from "@/shared/lib/cn";

export type ProgressProps = ComponentProps<typeof ProgressPrimitive.Root> & { size?: "default" | "lg" };

/** Square bar: 6px track (10px with `size="lg"`) on blue-100, navy fill. */
export function Progress({ className, value, size = "default", ...props }: ProgressProps) {
  const pct = Math.min(100, Math.max(0, value ?? 0));
  return (
    <ProgressPrimitive.Root
      data-slot="progress"
      value={value}
      className={cn(
        "relative w-full overflow-hidden rounded-none bg-surface-blue-100",
        size === "lg" ? "h-2.5" : "h-1.5",
        className,
      )}
      {...props}
    >
      <ProgressPrimitive.Indicator
        data-slot="progress-indicator"
        className="h-full w-full bg-navy-700 transition-transform duration-300 ease-brand"
        style={{ transform: `translateX(-${String(100 - pct)}%)` }}
      />
    </ProgressPrimitive.Root>
  );
}
