import type { ComponentProps } from "react";
import { cn } from "@/shared/lib/cn";

/** Shared control look: 40px, 2px radius, navy border and 3px blue-100 ring on focus. */
export const controlClass = [
  "w-full min-w-0 rounded-xs border border-input bg-white px-3 text-[16px] text-ink",
  "transition-[border-color,box-shadow] duration-[180ms] ease-brand placeholder:text-gray-500",
  "focus:border-navy-700 focus:ring-3 focus:ring-surface-blue-100 focus:outline-none",
  "aria-invalid:border-destructive disabled:cursor-not-allowed disabled:bg-surface-100 disabled:text-ink-3",
].join(" ");

export function Input({ className, type = "text", ...props }: ComponentProps<"input">) {
  return (
    <input
      data-slot="input"
      type={type}
      className={cn(controlClass, "h-10 max-[720px]:min-h-11", className)}
      {...props}
    />
  );
}
