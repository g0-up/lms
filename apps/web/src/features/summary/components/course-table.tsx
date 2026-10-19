import { shortTitle } from "@/shared/domain";
import { ToneBadge } from "@/shared/ui/badge";
import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from "@/shared/ui/table";
import { TableWrap } from "@/shared/ui/table-wrap";
import { assessmentScoreText, homeworkScoreText, missingText } from "../model/course-summary";
import type { CourseSummary, StageSummary } from "../model/schemas";
import { stageBadge } from "../model/stage-status";

/** One line per scored lesson; with several in a stage each line names its lesson. */
function ScoreLines<T extends { lessonId: string; title: string }>({
  scores,
  text,
}: {
  scores: readonly T[];
  text: (score: T) => string;
}) {
  if (scores.length === 0) return <span className="text-ink-3">—</span>;
  if (scores.length === 1) return <span className="whitespace-nowrap">{text(scores[0])}</span>;
  return (
    <ul className="grid gap-0.5">
      {scores.map((s) => (
        <li key={s.lessonId}>
          {shortTitle(s.title)}: <span className="whitespace-nowrap">{text(s)}</span>
        </li>
      ))}
    </ul>
  );
}

/**
 * Every stage of the course in one table: its state, what is still missing and the scores of its assessments and
 * homework. Stage by stage only; the course has no total score.
 */
export function CourseTable({ course, stages }: { course: CourseSummary; stages: readonly StageSummary[] }) {
  return (
    <TableWrap>
      <Table>
        <TableCaption>Tổng kết từng chặng</TableCaption>
        <TableHeader>
          <TableRow>
            <TableHead>Chặng</TableHead>
            <TableHead>Trạng thái</TableHead>
            <TableHead>Còn thiếu</TableHead>
            <TableHead>Quiz bài tập</TableHead>
            <TableHead>Homework</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {course.stages.map((cs, i) => {
            const stage = stages.find((s) => s.stageVersionId === cs.stageVersionId);
            const badge = stageBadge({
              complete: cs.complete,
              overdue: stage?.overdue ?? false,
              rework: stage?.rework ?? false,
            });
            return (
              <TableRow key={cs.stageVersionId}>
                <TableCell className="font-medium text-ink">
                  {i + 1}. {cs.name}
                </TableCell>
                <TableCell>
                  <ToneBadge tone={badge.tone}>{badge.label}</ToneBadge>
                </TableCell>
                <TableCell className="whitespace-nowrap">
                  {missingText(cs.missingCount) ?? <span className="text-ink-3">—</span>}
                </TableCell>
                <TableCell>
                  <ScoreLines scores={cs.assessments} text={assessmentScoreText} />
                </TableCell>
                <TableCell>
                  <ScoreLines scores={cs.homework} text={homeworkScoreText} />
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </TableWrap>
  );
}
