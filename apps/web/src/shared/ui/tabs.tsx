import { Tabs as TabsPrimitive } from "radix-ui";
import type { ComponentProps } from "react";
import { cn } from "@/shared/lib/cn";

export const Tabs = TabsPrimitive.Root;

export function TabsList({ className, ...props }: ComponentProps<typeof TabsPrimitive.List>) {
  return (
    <TabsPrimitive.List
      data-slot="tabs-list"
      className={cn("flex items-end gap-1 border-b border-line", className)}
      {...props}
    />
  );
}

export function TabsTrigger({ className, ...props }: ComponentProps<typeof TabsPrimitive.Trigger>) {
  return (
    <TabsPrimitive.Trigger
      data-slot="tabs-trigger"
      className={cn(
        "-mb-px inline-flex min-h-11 cursor-pointer items-center border-b-2 border-transparent px-4 font-sans text-sm font-medium text-ink-2",
        "transition-colors duration-200 hover:text-navy-700",
        "data-[state=active]:border-navy-700 data-[state=active]:text-navy-700",
        "focus-visible:outline-2 focus-visible:outline-offset-3 focus-visible:outline-navy-700",
        "disabled:cursor-not-allowed disabled:opacity-45",
        className,
      )}
      {...props}
    />
  );
}

export function TabsContent({ className, ...props }: ComponentProps<typeof TabsPrimitive.Content>) {
  return (
    <TabsPrimitive.Content
      data-slot="tabs-content"
      className={cn("pt-4 focus-visible:outline-2 focus-visible:outline-offset-3 focus-visible:outline-navy-700", className)}
      {...props}
    />
  );
}
