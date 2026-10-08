import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";
import type { KeyboardEvent, ReactNode } from "react";
import { cn } from "@/shared/lib/cn";
import { fmtDateTime, rel } from "@/shared/lib/format";
import { Badge } from "@/shared/ui/badge";
import { ProgressBar } from "@/shared/ui/progress-bar";
import { StatusDot } from "@/shared/ui/status-dot";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui/table";
import type { ClassReport, ReportRow, ReportSort } from "../model/schemas";
import { ariaSort, toggleSort, type SortColumn } from "../model/sort";
import { StageHeader } from "./stage-header";

interface SortableHeadProps {
  label: string;
  column: SortColumn;
  sort: ReportSort;
  onSort: (sort: ReportSort) => void;
  className?: string;
}

function SortableHead({ label, column, sort, onSort, className }: SortableHeadProps) {
  const direction = ariaSort(sort, column);
  const Icon = direction === "ascending" ? ArrowUp : direction === "descending" ? ArrowDown : ArrowUpDown;
  return (
    <TableHead aria-sort={direction} className={className}>
      <button
        type="button"
        onClick={() => { onSort(toggleSort(sort, column)); }}
        className="-my-3 inline-flex min-h-11 cursor-pointer items-center gap-1 uppercase hover:text-navy-700 focus-visible:outline-2 focus-visible:outline-navy-700"
      >
        {label}
        <Icon aria-hidden="true" className="size-3.5" />
      </button>
    </TableHead>
  );
}

function LastActivity({ at }: { at: string | null }) {
  if (!at) return <StatusDot variant="invited" label="Chưa đăng nhập" />;
  return (
    <div className="flex flex-col gap-0.5">
      <span>{rel(at)}</span>
      <span className="text-xs text-ink-3">{fmtDateTime(at)}</span>
    </div>
  );
}

function StageCell({ row, stageId }: { row: ReportRow; stageId: string }) {
  const stage = row.stagePercents.find((s) => s.stageId === stageId);
  return (
    <TableCell className="text-right tabular-nums">
      {stage && stage.requiredTotal > 0 ? (
        <span className={cn(stage.percent === 100 && "text-ok")}>{stage.percent}%</span>
      ) : (
        "—"
      )}
    </TableCell>
  );
}

export interface ReportTableProps {
  report: ClassReport;
  sort: ReportSort;
  onSort: (sort: ReportSort) => void;
  /** Opens the drilldown of a member (`class_members.id`). */
  onOpen: (memberId: string) => void;
  /** Shown in place of the rows when there are none. */
  empty: ReactNode;
  /** A new filter is loading while the previous rows stay on screen. */
  busy?: boolean;
}

/** FR-40 table: one row per member, one column per stage; rows open the drilldown. */
export function ReportTable({ report, sort, onSort, onOpen, empty, busy = false }: ReportTableProps) {
  const onRowKey = (event: KeyboardEvent<HTMLTableRowElement>, memberId: string) => {
    if (event.key !== "Enter" && event.key !== " ") return;
    event.preventDefault();
    onOpen(memberId);
  };

  return (
    <Table aria-busy={busy || undefined}>
      <TableCaption>Tiến độ học viên lớp {report.class.code}</TableCaption>
      <TableHeader>
        <TableRow>
          <TableHead>Học viên</TableHead>
          <TableHead>Lời mời</TableHead>
          {report.stages.map((stage) => (
            <StageHeader key={stage.stageId} stage={stage} />
          ))}
          <SortableHead label="Toàn khóa" column="pct" sort={sort} onSort={onSort} className="text-right" />
          <SortableHead label="Hoạt động cuối" column="activity" sort={sort} onSort={onSort} />
        </TableRow>
      </TableHeader>
      <TableBody>
        {report.rows.length === 0 ? (
          <TableRow>
            <TableCell colSpan={4 + report.stages.length}>{empty}</TableCell>
          </TableRow>
        ) : (
          report.rows.map((row) => {
            const dropped = row.memberStatus === "dropped";
            return (
              <TableRow
                key={row.memberId}
                tabIndex={0}
                role="button"
                aria-label={`Xem chi tiết ${row.name}`}
                onClick={() => { onOpen(row.memberId); }}
                onKeyDown={(e) => { onRowKey(e, row.memberId); }}
                className={cn(
                  "cursor-pointer focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-navy-700",
                  // Tinted rather than faded: lowering opacity takes the grey secondary text below 4.5:1.
                  dropped && "bg-surface-100",
                )}
              >
                <TableCell>
                  <div className="flex flex-col gap-0.5">
                    <span className="flex flex-wrap items-center gap-2 font-medium text-ink">
                      {row.name}
                      {dropped ? <Badge variant="muted">Đã rời lớp</Badge> : null}
                    </span>
                    <span className="text-xs text-ink-3">{row.email}</span>
                  </div>
                </TableCell>
                <TableCell>{row.invite?.status ? <StatusDot variant={row.invite.status} /> : "—"}</TableCell>
                {report.stages.map((stage) => (
                  <StageCell key={stage.stageId} row={row} stageId={stage.stageId} />
                ))}
                <TableCell className="text-right">
                  <ProgressBar value={row.percent} />
                </TableCell>
                <TableCell>
                  <LastActivity at={row.lastActivityAt} />
                </TableCell>
              </TableRow>
            );
          })
        )}
      </TableBody>
    </Table>
  );
}
