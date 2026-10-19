import { Link } from "react-router";
import { cn } from "@/shared/lib/cn";

export interface TabBarItem<K extends string> {
  key: K;
  label: string;
  /** Optional count pill next to the label. */
  count?: number;
}

export interface TabBarProps<K extends string> {
  tabs: readonly TabBarItem<K>[];
  active: K;
  /** Link target of a tab, usually `?tab=<key>`: each tab can be shared and reloaded. */
  hrefFor: (key: K) => string;
  label?: string;
}

/** Sections of a page as links, the current one marked with `aria-current="page"`. */
export function TabBar<K extends string>({ tabs, active, hrefFor, label = "Mục của lớp" }: TabBarProps<K>) {
  return (
    <nav aria-label={label} className="mb-6 flex gap-1 overflow-x-auto border-b border-line">
      {tabs.map((tab) => (
        <Link
          key={tab.key}
          to={hrefFor(tab.key)}
          aria-current={tab.key === active ? "page" : undefined}
          className={cn(
            "-mb-px inline-flex min-h-11 items-center gap-2 border-b-2 border-transparent px-4 text-sm font-medium whitespace-nowrap text-ink-3 no-underline",
            "hover:text-navy-700 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-navy-700",
            "aria-[current=page]:border-navy-700 aria-[current=page]:text-navy-700",
          )}
        >
          {tab.label}
          {tab.count !== undefined ? (
            <span className="rounded-pill bg-surface-100 px-2 text-xs text-ink-2 tabular-nums">{tab.count}</span>
          ) : null}
        </Link>
      ))}
    </nav>
  );
}
