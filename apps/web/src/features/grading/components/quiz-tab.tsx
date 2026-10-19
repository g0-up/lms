import { Plus } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router";
import { toast } from "sonner";
import { QUIZ_KIND_VI, quizReviewBadge } from "@/shared/domain";
import { isApiError } from "@/shared/api/errors";
import { rel } from "@/shared/lib/format";
import { Alert, AlertActions } from "@/shared/ui/alert";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { Card, CardHeader } from "@/shared/ui/card";
import { EmptyState } from "@/shared/ui/empty-state";
import { Skeleton } from "@/shared/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui/table";
import { useClassQuizzes } from "../hooks/use-quiz-review";
import { quizReviewHref, type GradingBase } from "../model/links";
import type { ClassQuizItem, MemberQuiz } from "../model/quiz-schemas";
import { canGrantAttempts, memberQuizAction, memberQuizBadge } from "../model/quiz-tab";
import { GrantDialog } from "./grant-dialog";
import { GrantHistory } from "./grant-history";
import { QuestionStats } from "./question-stats";

const NOT_ACTIVE_REASON = "Lớp không ở trạng thái đang chạy";

/** Where a quiz card sits: the class and whether it still runs (only a running class takes grants). */
interface QuizClass {
  id: string;
  active: boolean;
  base: GradingBase;
}

/** Line under the quiz title: its stage, what the quiz allows and how the teacher follows it. */
function settingsLine(item: ClassQuizItem): string {
  const rules =
    item.mode === "assessment"
      ? [
          item.maxAttempts === undefined ? null : `${String(item.maxAttempts)} lượt`,
          item.timeLimitMinutes === undefined ? null : `${String(item.timeLimitMinutes)} phút`,
          "giảng viên xác nhận từng lượt",
        ]
      : ["không giới hạn lượt", "nhận xét khi cần"];
  const threshold = item.passPercent === undefined ? null : `đạt từ ${String(item.passPercent)}%`;
  return [item.stage.name, ...rules, threshold].filter(Boolean).join(" · ");
}

function Latest({ member, item }: { member: MemberQuiz; item: ClassQuizItem }) {
  const { latest } = member;
  if (!latest) return <span className="text-ink-3">—</span>;
  const badge = latest.reviewStatus ? quizReviewBadge(item.mode, latest.reviewStatus) : null;
  return (
    <div className="flex flex-col gap-0.5">
      <span className="whitespace-nowrap tabular-nums">
        #{latest.no} · {latest.percent}% · {rel(latest.submittedAt)}
      </span>
      <span className="text-xs text-ink-3">
        {badge ? <Badge variant={badge.tone}>{badge.label}</Badge> : "Chưa nhận xét"}
      </span>
    </div>
  );
}

function MemberRow({ member, item, cls }: { member: MemberQuiz; item: ClassQuizItem; cls: QuizClass }) {
  const [granting, setGranting] = useState(false);
  const status = memberQuizBadge(member);
  const action = memberQuizAction(member, item.mode);
  const grantable = canGrantAttempts(member, item.mode);
  return (
    <TableRow>
      <TableCell className="font-medium text-ink">{member.name}</TableCell>
      <TableCell className="text-right tabular-nums">
        {member.used}
        {member.limit === null ? "" : `/${String(member.limit)}`}
      </TableCell>
      <TableCell className="text-right tabular-nums">
        {member.bestPercent === null ? "—" : `${String(member.bestPercent)}%`}
      </TableCell>
      <TableCell>
        <Badge variant={status.tone}>{status.label}</Badge>
      </TableCell>
      <TableCell>
        <Latest member={member} item={item} />
      </TableCell>
      <TableCell className="text-right">
        <div className="flex flex-wrap justify-end gap-1">
          {action && member.latest ? (
            <Button asChild size="sm" variant={action.primary ? "default" : "ghost"}>
              <Link
                to={quizReviewHref(cls.base, member.latest.attemptId, "quiz")}
                aria-label={`${action.verb} lượt #${String(member.latest.no)} của ${member.name}`}
              >
                {action.label}
              </Link>
            </Button>
          ) : null}
          {grantable ? (
            <Button
              size="sm"
              variant={member.status === "outOfAttempts" ? "outline" : "ghost"}
              aria-label={`Cấp thêm lượt cho ${member.name}`}
              aria-disabled={!cls.active || undefined}
              title={cls.active ? undefined : NOT_ACTIVE_REASON}
              onClick={() => {
                if (cls.active) setGranting(true);
                else toast.error(`${NOT_ACTIVE_REASON}, không cấp thêm lượt được.`);
              }}
            >
              <Plus aria-hidden="true" /> Cấp thêm lượt
            </Button>
          ) : null}
        </div>
        {grantable ? (
          <GrantDialog open={granting} onOpenChange={setGranting} classId={cls.id} member={member} item={item} />
        ) : null}
      </TableCell>
    </TableRow>
  );
}

function QuizCard({ item, cls }: { item: ClassQuizItem; cls: QuizClass }) {
  const { stats, members } = item;
  const assess = item.mode === "assessment";
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
            <span className="text-sm text-ink-3">{settingsLine(item)}</span>
          </div>
          <Badge variant={assess ? "info" : "subtle"}>{QUIZ_KIND_VI[item.mode]}</Badge>
        </CardHeader>
        <div className="flex flex-wrap gap-x-5 gap-y-1 border-b border-line px-6 py-3 text-sm text-ink-3 max-[720px]:px-4">
          <span>
            <b className="font-semibold text-ink tabular-nums">{stats.attempted}</b>/{stats.members} đã làm
          </span>
          {assess ? stat(stats.passed, "đạt") : null}
          {assess ? stat(stats.pending, "chờ xác nhận") : null}
          <span>
            Điểm TB lượt tốt nhất{" "}
            <b className="font-semibold text-ink tabular-nums">
              {stats.avgBestPercent === null ? "—" : `${String(stats.avgBestPercent)}%`}
            </b>
          </span>
          {stat(stats.submittedAttempts, "lượt đã nộp")}
        </div>
        <QuestionStats stats={item.questionStats} />
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Học viên</TableHead>
              <TableHead className="text-right">Lượt đã dùng</TableHead>
              <TableHead className="text-right">Tốt nhất</TableHead>
              <TableHead>Trạng thái</TableHead>
              <TableHead>Lượt gần nhất</TableHead>
              <TableHead className="text-right">Thao tác</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {members.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6}>
                  <EmptyState title="Chưa có học viên" text="Mời học viên vào lớp trước." />
                </TableCell>
              </TableRow>
            ) : (
              members.map((m) => <MemberRow key={m.memberId} member={m} item={item} cls={cls} />)
            )}
          </TableBody>
        </Table>
        <GrantHistory grants={item.grants} />
      </Card>
    </section>
  );
}

export interface QuizTabProps {
  classId: string;
  /** Area the class page lives in; review links stay in it. */
  basePath: GradingBase;
}

/**
 * The "Quiz" tab of a class: per quiz its settings, counts, correct rate by question and each member's attempts and
 * latest review state; on an assessment, extra attempts for members who have not passed and the grants made so far.
 */
export function QuizTab({ classId, basePath }: QuizTabProps) {
  const { data, error, isPending, refetch } = useClassQuizzes(classId);
  if (isPending) {
    return (
      <Card className="grid gap-3 p-6" aria-busy="true">
        <span className="sr-only">Đang tải quiz của lớp</span>
        {Array.from({ length: 3 }, (_, i) => (
          <Skeleton key={i} className="h-10 w-full" />
        ))}
      </Card>
    );
  }
  if (error) {
    // A class outside the grader's reach does not come back on retry.
    const final = isApiError(error) && (error.status === 403 || error.status === 404);
    return (
      <Alert variant="danger" role="alert">
        {isApiError(error) ? error.message : "Không tải được quiz của lớp."}
        {final ? null : (
          <AlertActions>
            <Button variant="outline" size="sm" onClick={() => void refetch()}>
              Thử lại
            </Button>
          </AlertActions>
        )}
      </Alert>
    );
  }
  if (data.items.length === 0) {
    return (
      <Card>
        <EmptyState title="Khóa học không có quiz" text="Phiên bản khóa học của lớp này không có học liệu loại quiz." />
      </Card>
    );
  }
  const cls: QuizClass = { id: data.class.id, active: data.class.status === "active", base: basePath };
  return (
    <div className="grid gap-5">
      {data.items.map((item) => (
        <QuizCard key={item.lesson.id} item={item} cls={cls} />
      ))}
    </div>
  );
}
