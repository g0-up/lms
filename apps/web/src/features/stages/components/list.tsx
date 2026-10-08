import type { ComponentProps } from "react";
import { Link, type LinkProps } from "react-router";
import { cn } from "@/shared/lib/cn";

/** Plain list inside a card: one bordered row per item. */
export function List({ className, ...props }: ComponentProps<"ul">) {
  return <ul data-slot="list" className={cn("m-0 list-none p-0", className)} {...props} />;
}

export function ListItem({ className, ...props }: ComponentProps<"li">) {
  return (
    <li
      data-slot="list-item"
      className={cn("flex items-center gap-3 border-b border-line px-6 py-3 last:border-b-0 max-[720px]:px-4", className)}
      {...props}
    />
  );
}

/** Position number of an ordered row (28px square). */
export function Ord({ n }: { n: number }) {
  return (
    <span className="inline-grid size-7 shrink-0 place-items-center bg-surface-blue-50 text-xs font-semibold text-navy-700 tabular-nums">
      {n}
    </span>
  );
}

/** Growing middle column holding the title and the meta line. */
export function ItemBody({ className, ...props }: ComponentProps<"div">) {
  return <div className={cn("min-w-0 flex-1", className)} {...props} />;
}

export function ItemTitle({ className, ...props }: ComponentProps<"div">) {
  return <div className={cn("font-medium text-ink", className)} {...props} />;
}

/** Title link; the vertical padding widens the hit area without moving the text. */
export function ItemTitleLink({ className, ...props }: LinkProps) {
  return (
    <Link
      className={cn("-my-[11px] inline-block py-[11px] font-medium text-ink no-underline hover:text-navy-700 hover:underline", className)}
      {...props}
    />
  );
}

export function ItemMeta({ className, ...props }: ComponentProps<"div">) {
  return <div className={cn("flex flex-wrap items-center gap-2 text-xs text-ink-3", className)} {...props} />;
}
