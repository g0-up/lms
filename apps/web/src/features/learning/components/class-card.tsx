import { Link } from "react-router";
import { fmtDate } from "@/shared/lib/format";
import { StatusBadge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card } from "@/shared/ui/card";
import { ProgressBar } from "@/shared/ui/progress-bar";
import type { MyClassItem } from "../model/schemas";

/** Calendar day ("2026-10-05") read as local midnight, so no time zone can shift it a day. */
const fmtDay = (day: string) => fmtDate(`${day}T00:00:00`);

export interface ClassCardProps {
  item: MyClassItem;
  /** The page's single gradient button goes on the first running class. */
  primary?: boolean;
}

export function ClassCard({ item, primary = false }: ClassCardProps) {
  const roadmapHref = `/learn/classes/${item.id}`;
  return (
    <Card className="p-6 max-[720px]:px-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2>{item.name}</h2>
        <StatusBadge status={item.status} />
      </div>
      <p className="mt-1 mb-4 text-sm text-ink-3">
        {item.code} · {item.courseName} · {fmtDay(item.startDate)} → {fmtDay(item.endDate)}
      </p>
      <ProgressBar size="lg" value={item.percent} />
      <p className="mt-1.5 text-sm text-ink-3">
        {item.requiredDone}/{item.requiredTotal} học liệu bắt buộc
      </p>
      <div className="mt-4 flex flex-wrap items-center gap-2">
        {item.readOnlyReason === "draft" ? (
          <span className="text-sm text-ink-3">
            Lớp bắt đầu {fmtDay(item.startDate)}. Bạn sẽ vào học được khi lớp kích hoạt.
          </span>
        ) : (
          <>
            {item.nextLesson ? (
              <Button asChild variant={primary ? "gradient" : "default"}>
                <Link to={`${roadmapHref}/lessons/${item.nextLesson.lessonId}`}>Học tiếp</Link>
              </Button>
            ) : (
              <Button asChild>
                <Link to={roadmapHref}>Xem lộ trình</Link>
              </Button>
            )}
            <Button asChild variant="ghost">
              <Link to={roadmapHref}>Lộ trình</Link>
            </Button>
          </>
        )}
      </div>
    </Card>
  );
}
