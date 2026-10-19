import { Link } from "react-router";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card, CardHeader, CardTitle } from "@/shared/ui/card";
import { EmptyState } from "@/shared/ui/empty-state";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui/table";
import { TableWrap } from "@/shared/ui/table-wrap";
import { fmtAvgHours } from "../model/format-hours";
import type { TeacherBacklog } from "../model/schemas";

/** "Tồn đọng chấm bài": per teacher, the items waiting, those past three days and the average handling time. */
export function BacklogCard({ backlog }: { backlog: readonly TeacherBacklog[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Tồn đọng chấm bài</CardTitle>
        <Button asChild variant="ghost" size="sm">
          <Link to="/admin/grading">Hàng chờ chấm</Link>
        </Button>
      </CardHeader>
      {backlog.length === 0 ? (
        <EmptyState title="Chưa có giảng viên" />
      ) : (
        <TableWrap>
          <Table>
            <TableCaption>Tồn đọng chấm bài</TableCaption>
            <TableHeader>
              <TableRow>
                <TableHead>Giảng viên</TableHead>
                <TableHead className="text-right">Chờ chấm, xác nhận</TableHead>
                <TableHead className="text-right">Quá 3 ngày</TableHead>
                <TableHead className="text-right">TG chấm TB</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {backlog.map((row) => (
                <TableRow key={row.teacherId}>
                  <TableCell>{row.teacherName}</TableCell>
                  <TableCell className="text-right tabular-nums">{row.pending}</TableCell>
                  <TableCell className="text-right tabular-nums">
                    {row.overdue > 0 ? <Badge variant="danger">{row.overdue}</Badge> : 0}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">{fmtAvgHours(row.avgHours)}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableWrap>
      )}
    </Card>
  );
}
