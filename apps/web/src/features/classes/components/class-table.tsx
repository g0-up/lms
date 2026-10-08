import type { ReactNode } from "react";
import { Link } from "react-router";
import { StatusBadge } from "@/shared/ui/badge";
import { ProgressBar } from "@/shared/ui/progress-bar";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui/table";
import { fmtDay } from "../model/day";
import type { ClassListItem } from "../model/schemas";

export interface ClassTableProps {
  classes: readonly ClassListItem[];
  /** Average progress by class id; a class missing from it shows "—". */
  progress: ReadonlyMap<string, number> | undefined;
  /** Shown in place of the rows when there are none. */
  empty: ReactNode;
}

/** Every class with its course version, teacher, active members and average progress. */
export function ClassTable({ classes, progress, empty }: ClassTableProps) {
  return (
    <Table>
      <TableCaption>Danh sách lớp học</TableCaption>
      <TableHeader>
        <TableRow>
          <TableHead>Lớp</TableHead>
          <TableHead>Khóa học</TableHead>
          <TableHead>Trạng thái</TableHead>
          <TableHead>Giảng viên</TableHead>
          <TableHead className="text-right">Học viên</TableHead>
          <TableHead>Tiến độ TB</TableHead>
          <TableHead className="text-right">Bắt đầu</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {classes.length === 0 ? (
          <TableRow>
            <TableCell colSpan={7}>{empty}</TableCell>
          </TableRow>
        ) : (
          classes.map((c) => {
            const avg = c.status === "draft" ? undefined : progress?.get(c.id);
            return (
              <TableRow key={c.id}>
                <TableCell>
                  <div className="flex flex-col gap-0.5">
                    <Link
                      to={`/admin/classes/${c.id}`}
                      className="inline-flex min-h-11 items-center self-start font-medium text-navy-700 hover:underline"
                    >
                      {c.code}
                    </Link>
                    <span className="-mt-2 text-xs text-ink-3">{c.name}</span>
                  </div>
                </TableCell>
                <TableCell>
                  <span className="inline-flex flex-wrap items-center gap-2">
                    {c.courseName}
                    <span className="inline-flex items-center rounded-pill border border-navy-700 px-2 text-xs font-semibold text-navy-700">
                      v{c.courseVersionNo}
                    </span>
                  </span>
                </TableCell>
                <TableCell>
                  <StatusBadge status={c.status} />
                </TableCell>
                <TableCell>{c.teacherName}</TableCell>
                <TableCell className="text-right tabular-nums">{c.memberCount}</TableCell>
                <TableCell>{avg === undefined ? <span className="text-ink-3">—</span> : <ProgressBar value={avg} />}</TableCell>
                <TableCell className="text-right tabular-nums">{fmtDay(c.startDate)}</TableCell>
              </TableRow>
            );
          })
        )}
      </TableBody>
    </Table>
  );
}
