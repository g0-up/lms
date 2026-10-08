import type { ComponentProps } from "react";
import { cn } from "@/shared/lib/cn";

/** Bare table; wrap it in `TableWrap` so wide data scrolls inside the card, never the page. */
export function Table({ className, ...props }: ComponentProps<"table">) {
  return <table data-slot="table" className={cn("w-full border-collapse text-sm", className)} {...props} />;
}

export function TableHeader({ className, ...props }: ComponentProps<"thead">) {
  return <thead data-slot="table-header" className={cn(className)} {...props} />;
}

export function TableBody({ className, ...props }: ComponentProps<"tbody">) {
  return (
    <tbody
      data-slot="table-body"
      className={cn("[&_tr]:transition-colors [&_tr:hover]:bg-surface-50 [&_tr:last-child>td]:border-b-0", className)}
      {...props}
    />
  );
}

export function TableRow({ className, ...props }: ComponentProps<"tr">) {
  return <tr data-slot="table-row" className={cn(className)} {...props} />;
}

export function TableHead({ className, ...props }: ComponentProps<"th">) {
  return (
    <th
      data-slot="table-head"
      className={cn(
        "sticky top-0 border-b border-line bg-white px-4 py-3 text-left align-middle text-xs font-semibold tracking-[.5px] whitespace-nowrap text-ink uppercase",
        className,
      )}
      {...props}
    />
  );
}

export function TableCell({ className, ...props }: ComponentProps<"td">) {
  return <td data-slot="table-cell" className={cn("border-b border-line px-4 py-3 align-middle", className)} {...props} />;
}

export function TableCaption({ className, ...props }: ComponentProps<"caption">) {
  return <caption data-slot="table-caption" className={cn("sr-only", className)} {...props} />;
}
