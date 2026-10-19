import { CircleCheck, MessageSquare } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "@/shared/lib/cn";
import { versionLabel } from "@/shared/lib/format";
import { Badge } from "@/shared/ui/badge";
import { ProgressBar } from "@/shared/ui/progress-bar";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui/table";
import type { StageTarget } from "../api/summary-api";
import type { ClassMatrix, MatrixCell, MatrixRow } from "../model/schemas";
import { cellLabel, cellMissing, cellState, cellTone, type CellTone } from "../model/stage-status";

// Left border and fill of a cell, the same in the legend; the cell also says its state in words.
const TONE_CLASS: Record<CellTone, string> = {
  complete: "border-l-ok bg-ok-bg",
  progress: "border-l-navy-700 bg-white",
  rework: "border-l-warn bg-warn-bg",
  overdue: "border-l-danger bg-danger-bg",
  todo: "border-l-gray-300 bg-white text-ink-3",
};

const LEGEND: { tone: CellTone; label: string }[] = [
  { tone: "complete", label: "Hoàn thành" },
  { tone: "progress", label: "Đang học" },
  { tone: "rework", label: "Cần làm lại" },
  { tone: "overdue", label: "Quá hạn" },
];

/** What the colours of the cells mean. */
export function MatrixLegend() {
  return (
    <ul aria-label="Chú giải màu ô" className="flex flex-wrap gap-x-4 gap-y-2 text-sm text-ink-3">
      {LEGEND.map(({ tone, label }) => (
        <li key={tone} className="inline-flex items-center gap-1.5">
          <span aria-hidden="true" className={cn("size-3 border border-l-[3px] border-line", TONE_CLASS[tone])} />
          {label}
        </li>
      ))}
    </ul>
  );
}

function Cell({ row, stageName, cell, onOpen }: { row: MatrixRow; stageName: string; cell: MatrixCell; onOpen: () => void }) {
  const missing = cellMissing(cell);
  return (
    <TableCell className="min-w-[132px] p-1">
      <button
        type="button"
        aria-label={cellLabel(row.name, stageName, cell)}
        onClick={onOpen}
        className={cn(
          "grid min-h-14 w-full cursor-pointer content-center gap-1 border border-l-[3px] border-line px-3 py-2 text-left text-sm text-ink transition-colors hover:border-navy-700 hover:bg-surface-blue-50 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-navy-700",
          TONE_CLASS[cellTone(cell)],
        )}
      >
        <span className="inline-flex items-center gap-1.5 font-medium tabular-nums">
          {cell.complete ? <CircleCheck aria-hidden="true" className="size-4 text-ok" /> : null}
          {cellState(cell)}
        </span>
        {missing || cell.overdue || cell.rework || cell.hasReview ? (
          <span className="flex flex-wrap items-center gap-1 text-xs">
            {missing ? <span>{missing}</span> : null}
            {cell.overdue ? <Badge variant="danger">Quá hạn</Badge> : null}
            {cell.rework ? <Badge variant="warn">Làm lại</Badge> : null}
            {cell.hasReview ? (
              <MessageSquare aria-hidden="true" className="ml-auto size-3.5 text-navy-700" />
            ) : null}
          </span>
        ) : null}
      </button>
    </TableCell>
  );
}

export interface SummaryMatrixProps {
  matrix: ClassMatrix;
  /** The rows left by the filter, in the API's order (by name). */
  rows: MatrixRow[];
  onOpen: (target: StageTarget) => void;
  /** Shown in place of the rows when there are none. */
  empty: ReactNode;
}

/** FR-82 matrix: one row per active member, one cell per stage that opens its summary, then the course percent. */
export function SummaryMatrix({ matrix, rows, onOpen, empty }: SummaryMatrixProps) {
  return (
    <Table>
      <TableCaption>Tổng kết theo chặng lớp {matrix.class.code}</TableCaption>
      <TableHeader>
        <TableRow>
          <TableHead scope="col">Học viên</TableHead>
          {matrix.stages.map((stage) => (
            <TableHead key={stage.stageVersionId} scope="col" title={`${stage.name} ${versionLabel({ no: stage.versionNo })}`}>
              {stage.name}
            </TableHead>
          ))}
          <TableHead scope="col" className="text-right">
            Toàn khóa
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.length === 0 ? (
          <TableRow>
            <TableCell colSpan={matrix.stages.length + 2}>{empty}</TableCell>
          </TableRow>
        ) : (
          rows.map((row) => (
            <TableRow key={row.memberId}>
              <th scope="row" className="min-w-[180px] border-b border-line px-4 py-3 text-left align-middle font-normal">
                <span className="flex flex-col gap-0.5">
                  <span className="font-medium text-ink">{row.name}</span>
                  <span className="text-xs text-ink-3">{row.email}</span>
                </span>
              </th>
              {matrix.stages.map((stage) => {
                const cell = row.cells.find((c) => c.stageVersionId === stage.stageVersionId);
                return cell ? (
                  <Cell
                    key={stage.stageVersionId}
                    row={row}
                    stageName={stage.name}
                    cell={cell}
                    onOpen={() => {
                      onOpen({ memberId: row.memberId, stageVersionId: stage.stageVersionId });
                    }}
                  />
                ) : (
                  <TableCell key={stage.stageVersionId}>—</TableCell>
                );
              })}
              <TableCell className="text-right">
                <ProgressBar value={row.coursePercent} />
              </TableCell>
            </TableRow>
          ))
        )}
      </TableBody>
    </Table>
  );
}
