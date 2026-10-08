import { Link } from "react-router";
import { StatusBadge } from "@/shared/ui/badge";
import { ProgressBar } from "@/shared/ui/progress-bar";
import { StatusDot } from "@/shared/ui/status-dot";
import { fmtDay } from "../model/day";
import type { TeachClassItem } from "../model/schemas";

/** Whole-card link to the class report, with its average progress and stale learners. */
export function TeacherClassCard({ item }: { item: TeachClassItem }) {
  const stale = item.status === "active" ? item.inactiveOver7Days : 0;
  return (
    <Link
      to={`/teach/classes/${item.id}`}
      className="block bg-card p-6 text-ink no-underline shadow-card transition-shadow duration-[180ms] ease-brand hover:shadow-float focus-visible:outline-2 focus-visible:outline-navy-700 max-[720px]:px-4"
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2>{item.code}</h2>
        <StatusBadge status={item.status} />
      </div>
      <p className="mt-1 mb-4 text-sm text-ink-3">
        {item.name} · {item.courseName} v{item.courseVersionNo}
      </p>
      {item.status === "draft" ? (
        <p className="text-sm text-ink-3">Bắt đầu {fmtDay(item.startDate)}</p>
      ) : (
        <ProgressBar value={item.avgPercent ?? 0} />
      )}
      <div className="mt-3 flex flex-wrap items-center gap-3 text-sm text-ink-3">
        <span>{item.memberCount} học viên</span>
        {stale > 0 ? <StatusDot variant="invited" label={`${String(stale)} lâu không hoạt động`} /> : null}
      </div>
    </Link>
  );
}
