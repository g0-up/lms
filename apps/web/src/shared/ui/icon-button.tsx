import type { ComponentProps } from "react";
import { cn } from "@/shared/lib/cn";

export type IconButtonProps = Omit<ComponentProps<"button">, "aria-label"> & {
  /** Required: the button has no visible text. */
  "aria-label": string;
};

/** 44×44 icon-only button with a slate icon; pass a lucide icon as the child. */
export function IconButton({ className, type = "button", ...props }: IconButtonProps) {
  return (
    <button
      data-slot="icon-button"
      type={type}
      className={cn(
        "inline-grid size-11 shrink-0 cursor-pointer place-items-center rounded-sm border-0 bg-transparent text-slate-600",
        "transition-colors duration-[180ms] ease-brand hover:bg-surface-blue-50 hover:text-navy-700",
        "disabled:cursor-not-allowed disabled:opacity-35 [&_svg]:size-[18px]",
        className,
      )}
      {...props}
    />
  );
}
