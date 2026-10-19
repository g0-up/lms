import { percentOf } from "@/shared/domain";
import { cn } from "@/shared/lib/cn";
import { fmtDateTime } from "@/shared/lib/format";
import { Badge, SubmissionBadge } from "@/shared/ui/badge";
import { Card, CardBody, CardHeader, CardTitle } from "@/shared/ui/card";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui/table";
import { TableWrap } from "@/shared/ui/table-wrap";
import type { LatestResult } from "../model/submissions";

/** Result of the latest graded submission: total, score and note per criterion, the teacher's comment. */
export function SubmissionFeedback({ result }: { result: LatestResult }) {
  const { submission, grade, newerPending } = result;
  const passed = grade.result === "passed";
  return (
    <Card className={cn("border-l-4", passed ? "border-l-ok" : "border-l-warn")}>
      <CardHeader>
        <CardTitle>Kết quả lần nộp #{submission.no}</CardTitle>
        {newerPending ? (
          <Badge variant="info">Chờ chấm lần nộp mới</Badge>
        ) : (
          <SubmissionBadge status={grade.result} />
        )}
      </CardHeader>
      <CardBody className="grid gap-4">
        <p className="font-display text-2xl leading-tight font-bold text-ink tabular-nums">
          {grade.totalPoints}/{grade.maxPoints}{" "}
          <span className="font-sans text-md font-medium text-ink-3">
            ({percentOf(grade.totalPoints, grade.maxPoints)}%)
          </span>
        </p>
        <TableWrap>
          <Table>
            <TableCaption>Điểm từng tiêu chí của lần nộp #{submission.no}</TableCaption>
            <TableHeader>
              <TableRow>
                <TableHead>Tiêu chí</TableHead>
                <TableHead className="text-right">Điểm</TableHead>
                <TableHead>Ghi chú</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {grade.scores.map((score) => (
                <TableRow key={score.criterionId}>
                  <TableCell>{score.description}</TableCell>
                  <TableCell className="text-right tabular-nums">
                    {score.points}/{score.maxPoints}
                  </TableCell>
                  <TableCell>{score.comment || <span className="text-ink-3">—</span>}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableWrap>
        <div className="grid gap-1 border-l-4 border-navy-700 bg-surface-blue-50 px-4 py-3">
          <h3 className="text-xs font-semibold tracking-[.5px] text-ink-3 uppercase">Nhận xét của {grade.gradedBy}</h3>
          <p className="whitespace-pre-line text-ink">{grade.comment}</p>
          <p className="text-xs text-ink-3">{fmtDateTime(grade.gradedAt)}</p>
        </div>
      </CardBody>
    </Card>
  );
}
