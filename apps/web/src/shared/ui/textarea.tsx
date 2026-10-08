import type { ComponentProps } from "react";
import { cn } from "@/shared/lib/cn";
import { controlClass } from "./input";

export function Textarea({ className, ...props }: ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(controlClass, "min-h-35 resize-y py-3 font-mono text-[14px] leading-[1.5]", className)}
      {...props}
    />
  );
}
