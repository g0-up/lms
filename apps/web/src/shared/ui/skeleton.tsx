import type { ComponentProps } from "react";
import { cn } from "@/shared/lib/cn";

export function Skeleton({ className, ...props }: ComponentProps<"div">) {
  return <div data-slot="skeleton" aria-hidden="true" className={cn("animate-pulse bg-surface-100", className)} {...props} />;
}
