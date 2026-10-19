import type { StatusTone } from "@/shared/ui/badge";
import type { MatrixCell, StageSummary } from "./schemas";

export interface StageBadge {
  tone: StatusTone;
  label: string;
}

/** Badge of a stage on the student's summary: complete first, then what needs the student most. */
export function stageBadge(stage: Pick<StageSummary, "complete" | "overdue" | "rework">): StageBadge {
  if (stage.complete) return { tone: "ok", label: "Hoàn thành chặng" };
  if (stage.overdue) return { tone: "danger", label: "Có bài quá hạn" };
  if (stage.rework) return { tone: "warn", label: "Có bài cần làm lại" };
  return { tone: "info", label: "Đang học" };
}

/** Colour of a matrix cell, matching the legend; the cell's text always says the same in words. */
export type CellTone = "complete" | "overdue" | "rework" | "progress" | "todo";

export function cellTone(cell: MatrixCell): CellTone {
  if (cell.complete) return "complete";
  if (cell.overdue) return "overdue";
  if (cell.rework) return "rework";
  return cell.done > 0 ? "progress" : "todo";
}

/** Visible state of a cell: "Hoàn thành" or the required lessons done ("3/5 bắt buộc"). */
export function cellState(cell: MatrixCell): string {
  return cell.complete ? "Hoàn thành" : `${String(cell.done)}/${String(cell.required)} bắt buộc`;
}

/** What is still missing in an incomplete cell ("Còn 2 mục"), null once complete. */
export function cellMissing(cell: MatrixCell): string | null {
  return cell.complete ? null : `Còn ${String(cell.missing)} mục`;
}

/** Accessible name of a cell's button: member, stage, state in words, flags and review, then what it opens. */
export function cellLabel(memberName: string, stageName: string, cell: MatrixCell): string {
  const parts = [cell.complete ? "Hoàn thành" : `Chưa hoàn thành, ${cellState(cell)}, còn ${String(cell.missing)} mục`];
  if (cell.overdue) parts.push("có bài quá hạn");
  if (cell.rework) parts.push("có bài cần làm lại");
  parts.push(cell.hasReview ? "đã nhận xét" : "chưa nhận xét");
  return `${memberName}, ${stageName}: ${parts.join(", ")}. Mở tổng kết chặng`;
}
