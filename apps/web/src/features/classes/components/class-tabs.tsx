import { Link } from "react-router";
import { cn } from "@/shared/lib/cn";
import { CLASS_TABS, CLASS_TAB_LABEL, type ClassTab } from "../model/class-tabs";

export interface ClassTabsProps {
  current: ClassTab;
  /** Members still in the class, next to "Học viên". */
  studentCount: number;
}

/** Sections of the class page as links (`?tab=`): each tab can be shared and reloaded. */
export function ClassTabs({ current, studentCount }: ClassTabsProps) {
  return (
    <nav aria-label="Mục của lớp" className="mb-6 flex gap-1 overflow-x-auto border-b border-line">
      {CLASS_TABS.map((tab) => (
        <Link
          key={tab}
          to={{ search: `?tab=${tab}` }}
          aria-current={tab === current ? "page" : undefined}
          className={cn(
            "-mb-px inline-flex min-h-11 items-center gap-2 border-b-2 border-transparent px-4 text-sm font-medium whitespace-nowrap text-ink-3 no-underline",
            "hover:text-navy-700 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-navy-700",
            "aria-[current=page]:border-navy-700 aria-[current=page]:text-navy-700",
          )}
        >
          {CLASS_TAB_LABEL[tab]}
          {tab === "students" ? (
            <span className="rounded-pill bg-surface-100 px-2 text-xs text-ink-2 tabular-nums">{studentCount}</span>
          ) : null}
        </Link>
      ))}
    </nav>
  );
}
