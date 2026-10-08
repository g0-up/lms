import { Fragment } from "react";
import { Link } from "react-router";
import { cn } from "@/shared/lib/cn";

export interface Crumb {
  label: string;
  /** Omit on the last item: it is the current page. */
  to?: string;
}

const itemClass = "inline-flex items-center max-[720px]:min-h-11";

/** Breadcrumb trail; the last item is the current page (`aria-current="page"`). */
export function Crumbs({ items, className }: { items: Crumb[]; className?: string }) {
  return (
    <nav
      aria-label="Đường dẫn"
      className={cn("mb-3 flex flex-wrap items-center gap-1 text-sm text-ink-3 max-[720px]:mb-0", className)}
    >
      {items.map((item, i) => {
        const last = i === items.length - 1;
        return (
          <Fragment key={`${String(i)}-${item.label}`}>
            {i > 0 ? (
              <span aria-hidden="true" className="mx-1 text-gray-300">
                /
              </span>
            ) : null}
            {last || !item.to ? (
              <span className={itemClass} aria-current={last ? "page" : undefined}>
                {item.label}
              </span>
            ) : (
              <Link
                to={item.to}
                className={cn(
                  itemClass,
                  "text-ink-3 no-underline hover:text-navy-700 hover:underline max-[720px]:min-w-11 max-[720px]:justify-center max-[720px]:px-1",
                )}
              >
                {item.label}
              </Link>
            )}
          </Fragment>
        );
      })}
    </nav>
  );
}
