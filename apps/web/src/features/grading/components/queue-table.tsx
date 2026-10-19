import type { ReactNode } from "react";
import { Link } from "react-router";
import { GRADING_SLA_DAYS, shortTitle } from "@/shared/domain";
import { cn } from "@/shared/lib/cn";
import { fmtDateTime } from "@/shared/lib/format";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui/table";
import { itemHref, type GradingBase } from "../model/links";
import type { QueueItem } from "../model/schemas";
import { fmtWait } from "../model/wait";

function Detail({ item }: { item: QueueItem }) {
  if (item.kind === "quiz") {
    return (
      <>
        Lượt #{item.no} · {item.scorePercent ?? 0}%{item.autoSubmitted ? " · tự nộp khi hết giờ" : ""}
      </>
    );
  }
  return (
    <>
      Lần nộp #{item.no}
      {item.isLate ? (
        <>
          {" · "}
          <span className="text-danger">nộp muộn</span>
        </>
      ) : null}
    </>
  );
}

export interface QueueTableProps {
  items: QueueItem[];
  base: GradingBase;
  /** Shown in place of the rows when there are none. */
  empty: ReactNode;
  /** A new filter is loading while the previous rows stay on screen. */
  busy?: boolean;
}

/** The grading queue, oldest first; items waiting past the SLA carry a danger rule and badge. */
export function QueueTable({ items, base, empty, busy = false }: QueueTableProps) {
  return (
    <Table aria-busy={busy || undefined}>
      <TableCaption>Hàng chờ chấm, cũ nhất ở trên</TableCaption>
      <TableHeader>
        <TableRow>
          <TableHead>Học viên</TableHead>
          <TableHead>Bài</TableHead>
          <TableHead>Lớp</TableHead>
          <TableHead>Nộp lúc</TableHead>
          <TableHead className="text-right">Đã chờ</TableHead>
          <TableHead>
            <span className="sr-only">Thao tác</span>
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {items.length === 0 ? (
          <TableRow>
            <TableCell colSpan={6}>{empty}</TableCell>
          </TableRow>
        ) : (
          items.map((item) => (
            <TableRow
              key={`${item.kind}-${item.itemId}`}
              className={cn(item.overdue && "shadow-[inset_3px_0_0_var(--color-danger)] hover:bg-danger-bg!")}
            >
              <TableCell>
                <div className="flex flex-col gap-0.5">
                  <span className="font-medium text-ink">{item.student.name}</span>
                  <span className="text-xs text-ink-3">
                    <Detail item={item} />
                  </span>
                </div>
              </TableCell>
              <TableCell>
                <div className="flex flex-col gap-0.5">
                  <span>{shortTitle(item.lesson.title)}</span>
                  <span className="flex flex-wrap items-center gap-1.5 text-xs text-ink-3">
                    {item.kind === "quiz" ? (
                      <Badge variant="info">Quiz bài tập</Badge>
                    ) : (
                      <Badge variant="muted">Bài tập</Badge>
                    )}
                    {item.stage.name}
                  </span>
                </div>
              </TableCell>
              <TableCell>{item.class.code}</TableCell>
              <TableCell className="whitespace-nowrap tabular-nums">{fmtDateTime(item.submittedAt)}</TableCell>
              <TableCell className="text-right whitespace-nowrap tabular-nums">
                {item.overdue ? (
                  <>
                    <Badge variant="danger">Quá {GRADING_SLA_DAYS} ngày</Badge>{" "}
                  </>
                ) : null}
                {fmtWait(item.waitedHours)}
              </TableCell>
              <TableCell className="text-right">
                <Button asChild size="sm">
                  <Link
                    to={itemHref(base, item.kind, item.itemId)}
                    aria-label={`${item.kind === "quiz" ? "Xác nhận" : "Chấm"} bài của ${item.student.name}: ${shortTitle(item.lesson.title)}`}
                  >
                    {item.kind === "quiz" ? "Xác nhận" : "Chấm"}
                  </Link>
                </Button>
              </TableCell>
            </TableRow>
          ))
        )}
      </TableBody>
    </Table>
  );
}
