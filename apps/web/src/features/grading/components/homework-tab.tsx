import { CalendarClock } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router";
import { toast } from "sonner";
import { GRADING_SLA_DAYS, shortTitle } from "@/shared/domain";
import { cn } from "@/shared/lib/cn";
import { dueIn, fmtDateTime } from "@/shared/lib/format";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Badge, SubmissionBadge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card, CardHeader } from "@/shared/ui/card";
import { EmptyState } from "@/shared/ui/empty-state";
import { Skeleton } from "@/shared/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui/table";
import { useClassHomework } from "../hooks/use-class-homework";
import { isPastDeadline } from "../model/deadline-time";
import { gradeHref, type GradingBase } from "../model/links";
import type { ClassHomework, ClassHomeworkItem, MemberHomework } from "../model/schemas";
import { isOverdueWaiting } from "../model/wait";
import { DeadlineDialog } from "./deadline-dialog";

const ENDED_REASON = "Lớp đã kết thúc";

function MemberStatus({ member, pastDeadline }: { member: MemberHomework; pastDeadline: boolean }) {
  if (member.status === "notSubmitted") {
    return pastDeadline ? <Badge variant="danger">Quá hạn</Badge> : <Badge variant="muted">Chưa nộp</Badge>;
  }
  return <SubmissionBadge status={member.status} />;
}

function Latest({ latest }: { latest: NonNullable<MemberHomework["latest"]> }) {
  const parts = [latest.hasRepo ? "Repo" : "", latest.fileCount > 0 ? `${String(latest.fileCount)} file` : ""];
  const content = parts.filter(Boolean).join(" + ");
  return (
    <div className="flex flex-col gap-0.5">
      <span className="whitespace-nowrap tabular-nums">
        #{latest.no} · {fmtDateTime(latest.submittedAt)}
      </span>
      <span className="text-xs text-ink-3">
        {latest.isLate ? <span className="text-danger">Nộp muộn</span> : null}
        {latest.isLate && content ? " · " : null}
        {content}
      </span>
    </div>
  );
}

function MemberRow({ member, item, base }: { member: MemberHomework; item: ClassHomeworkItem; base: GradingBase }) {
  const { latest } = member;
  const pastDeadline = item.deadline !== null && isPastDeadline(item.deadline);
  const overdueGrading = member.status === "pending" && isOverdueWaiting(member.waitingDays);
  const pending = member.status === "pending";
  return (
    <TableRow className={cn(overdueGrading && "shadow-[inset_3px_0_0_var(--color-danger)] hover:bg-danger-bg!")}>
      <TableCell className="font-medium text-ink">{member.name}</TableCell>
      <TableCell>
        <span className="flex flex-wrap items-center gap-1.5">
          <MemberStatus member={member} pastDeadline={pastDeadline} />
          {overdueGrading ? <Badge variant="danger">Quá {GRADING_SLA_DAYS} ngày chưa chấm</Badge> : null}
        </span>
      </TableCell>
      <TableCell>{latest ? <Latest latest={latest} /> : <span className="text-ink-3">—</span>}</TableCell>
      <TableCell className="text-right tabular-nums">
        {latest?.totalPoints != null && latest.maxPoints != null
          ? `${String(latest.totalPoints)}/${String(latest.maxPoints)}`
          : "—"}
      </TableCell>
      <TableCell className="text-right">
        {latest ? (
          <Button asChild size="sm" variant={pending ? "default" : "ghost"}>
            <Link
              to={gradeHref(base, latest.submissionId)}
              aria-label={`${pending ? "Chấm" : "Xem"} bài của ${member.name}: ${shortTitle(item.lesson.title)}`}
            >
              {pending ? "Chấm" : "Xem"}
            </Link>
          </Button>
        ) : null}
      </TableCell>
    </TableRow>
  );
}

function HomeworkCard({ cls, item, base }: { cls: ClassHomework["class"]; item: ClassHomeworkItem; base: GradingBase }) {
  const [editing, setEditing] = useState(false);
  const ended = cls.status === "ended";
  const { deadline, stats, members } = item;
  const pastDeadline = deadline !== null && isPastDeadline(deadline);
  const stat = (n: number, label: string) => (
    <span>
      <b className="font-semibold text-ink tabular-nums">{n}</b> {label}
    </span>
  );
  return (
    <section aria-label={item.lesson.title}>
      <Card>
        <CardHeader>
          <div className="grid gap-0.5">
            <h2>{item.lesson.title}</h2>
            <span className="text-sm text-ink-3">
              {item.stage.name} · {item.maxPoints} điểm · đạt từ {item.passPercent}%
            </span>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <span className={cn("inline-flex items-center gap-1.5 text-sm", pastDeadline ? "text-danger" : "text-ink-3")}>
              <CalendarClock aria-hidden="true" className="size-4" />
              {deadline ? `Hạn ${fmtDateTime(deadline)} · ${dueIn(deadline)}` : "Chưa đặt hạn nộp"}
            </span>
            <Button
              variant="outline"
              size="sm"
              aria-disabled={ended || undefined}
              title={ended ? ENDED_REASON : undefined}
              onClick={() => {
                if (ended) toast.error(`${ENDED_REASON}, không đổi hạn nộp được.`);
                else setEditing(true);
              }}
            >
              {deadline ? "Đổi hạn nộp" : "Đặt hạn nộp"}
            </Button>
          </div>
        </CardHeader>
        <div className="flex flex-wrap gap-x-5 gap-y-1 border-b border-line px-6 py-3 text-sm text-ink-3 max-[720px]:px-4">
          <span>
            <b className="font-semibold text-ink tabular-nums">{stats.submitted}</b>/{members.length} đã nộp
          </span>
          {stat(stats.pending, "chờ chấm")}
          {stat(stats.passed, "đạt")}
          {stat(stats.rework, "cần làm lại")}
          {stat(stats.overdueNotSubmitted, "quá hạn chưa nộp")}
        </div>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Học viên</TableHead>
              <TableHead>Trạng thái</TableHead>
              <TableHead>Lần nộp gần nhất</TableHead>
              <TableHead className="text-right">Điểm</TableHead>
              <TableHead className="text-right">Thao tác</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {members.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5}>
                  <EmptyState title="Chưa có học viên" text="Mời học viên vào lớp trước." />
                </TableCell>
              </TableRow>
            ) : (
              members.map((m) => <MemberRow key={m.memberId} member={m} item={item} base={base} />)
            )}
          </TableBody>
        </Table>
      </Card>
      <DeadlineDialog open={editing} onOpenChange={setEditing} cls={cls} item={item} />
    </section>
  );
}

export interface HomeworkTabProps {
  classId: string;
  /** Area the class page lives in; grading links stay in it. */
  basePath: GradingBase;
}

/** The "Bài tập" tab of a class: per homework its deadline, counts and each member's latest submission. */
export function HomeworkTab({ classId, basePath }: HomeworkTabProps) {
  const { data, error, isPending, refetch } = useClassHomework(classId);
  if (isPending) {
    return (
      <Card className="grid gap-3 p-6" aria-busy="true">
        <span className="sr-only">Đang tải bài tập của lớp</span>
        {Array.from({ length: 3 }, (_, i) => (
          <Skeleton key={i} className="h-10 w-full" />
        ))}
      </Card>
    );
  }
  if (error) {
    return (
      <Alert variant="danger" role="alert">
        Không tải được bài tập của lớp.
        <AlertActions>
          <Button variant="outline" size="sm" onClick={() => void refetch()}>
            Thử lại
          </Button>
        </AlertActions>
      </Alert>
    );
  }
  if (data.items.length === 0) {
    return (
      <Card>
        <EmptyState
          title="Khóa học không có bài tập"
          text="Phiên bản khóa học của lớp này không có học liệu loại bài tập."
        />
      </Card>
    );
  }
  return (
    <div className="grid gap-5">
      {data.items.map((item) => (
        <HomeworkCard key={item.lesson.id} cls={data.class} item={item} base={basePath} />
      ))}
    </div>
  );
}
