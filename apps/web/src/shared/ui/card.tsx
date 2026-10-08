import type { ComponentProps } from "react";
import { cn } from "@/shared/lib/cn";

export function Card({ className, ...props }: ComponentProps<"div">) {
  return <div data-slot="card" className={cn("bg-card text-card-foreground shadow-card", className)} {...props} />;
}

export function CardHeader({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      data-slot="card-header"
      className={cn(
        "flex flex-wrap items-center justify-between gap-3 border-b border-line px-6 py-4 max-[720px]:px-4 [&_h2]:text-md",
        className,
      )}
      {...props}
    />
  );
}

export function CardTitle({ className, ...props }: ComponentProps<"h2">) {
  return <h2 data-slot="card-title" className={cn("text-md", className)} {...props} />;
}

export function CardBody({ className, ...props }: ComponentProps<"div">) {
  return <div data-slot="card-body" className={cn("p-6 max-[720px]:px-4", className)} {...props} />;
}
