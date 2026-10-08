import type { ComponentProps } from "react";
import { cn } from "@/shared/lib/cn";

/** Horizontal scroll container for tables; headers stay sticky inside it. */
export function TableWrap({ className, ...props }: ComponentProps<"div">) {
  return <div data-slot="table-wrap" className={cn("overflow-x-auto overscroll-x-contain", className)} {...props} />;
}
