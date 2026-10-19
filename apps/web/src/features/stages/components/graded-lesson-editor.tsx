import { Lock, Pencil } from "lucide-react";
import { useState } from "react";
import { Link, useNavigate } from "react-router";
import { Alert, AlertActions, AlertTitle } from "@/shared/ui/alert";
import { Badge, StatusBadge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import type { Crumb } from "@/shared/ui/crumbs";
import { EmptyState } from "@/shared/ui/empty-state";
import { PageHead } from "@/shared/ui/page-head";
import { useLessonMutations } from "../hooks/use-lesson-mutations";
import { isGraded, lessonMeta } from "../model/lesson-form";
import type { StageVersion } from "../model/schemas";
import { lessonPublishBlockers } from "../model/stage-rules";
import { CriteriaTable } from "./criteria-table";
import { HomeworkBriefCard, HomeworkSettingsCard } from "./homework-brief-card";
import { LessonFormDialog } from "./lesson-form-dialog";
import { QuestionList } from "./question-list";
import { QuizSettingsCard } from "./quiz-settings-card";

const TYPE_LABEL = { quiz: "Quiz", homework: "Bài tập" } as const;
const LOCKED_AS = { published: "đã phát hành", archived: "đã lưu trữ" } as const;

export interface GradedLessonEditorProps {
  /** Kept current: every question, criterion or settings save lands here. */
  version: StageVersion;
  lessonId: string;
  /** Chặng and version crumbs; the unit and the lesson are appended. */
  crumbs: Crumb[];
  backPath: string;
}

/**
 * Page of a quiz or homework: settings, then the questions or the brief, files and rubric. A draft is
 * edited in place; a published or archived version is read-only.
 */
export function GradedLessonEditor({ version, lessonId, crumbs, backPath }: GradedLessonEditorProps) {
  const navigate = useNavigate();
  const { save } = useLessonMutations(version.stageId, version.id);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const lesson = version.lessons.find((l) => l.id === lessonId);

  if (!lesson || !isGraded(lesson)) {
    return (
      <>
        <PageHead crumbs={crumbs} title="Không tìm thấy học liệu" />
        <EmptyState
          title="Học liệu không còn trong phiên bản này hoặc đã đổi loại."
          action={
            <Button asChild variant="outline" size="sm">
              <Link to={backPath}>Về chặng</Link>
            </Button>
          }
        />
      </>
    );
  }

  const editable = version.status === "draft";
  const blockers = editable ? lessonPublishBlockers(lesson) : [];
  const unit = version.units.find((u) => u.id === lesson.unitId);
  const trail: Crumb[] = [...crumbs, ...(unit ? [{ label: unit.title, to: backPath }] : []), { label: lesson.title }];
  const ids = { stageId: version.stageId, vid: version.id, lessonId: lesson.id };

  return (
    <>
      <PageHead
        crumbs={trail}
        title={lesson.title}
        badges={
          <>
            <Badge variant="subtle">{TYPE_LABEL[lesson.type as keyof typeof TYPE_LABEL]}</Badge>
            <StatusBadge status={version.status} />
            {lesson.required ? null : <Badge variant="subtle">Không bắt buộc</Badge>}
          </>
        }
        lede={lessonMeta(lesson)}
        actions={
          editable ? (
            <Button
              variant="outline"
              onClick={() => {
                setSettingsOpen(true);
              }}
            >
              <Pencil aria-hidden="true" /> Sửa thiết lập
            </Button>
          ) : undefined
        }
      />
      <div className="grid gap-6">
        {version.status === "draft" ? null : (
          <Alert icon={<Lock aria-hidden="true" />}>
            v{version.versionNo} {LOCKED_AS[version.status]} nên không sửa được. Nhân bản chặng thành bản nháp mới để
            sửa; câu hỏi và tiêu chí giữ nguyên key.
            <AlertActions>
              <Button asChild variant="outline" size="sm">
                <Link to={backPath}>Về chặng</Link>
              </Button>
            </AlertActions>
          </Alert>
        )}
        {blockers.length > 0 ? (
          <Alert variant="warn">
            <AlertTitle>Chưa đủ để phát hành:</AlertTitle>
            <ul className="mt-1 list-disc pl-5">
              {blockers.map((b) => (
                <li key={b}>{b}</li>
              ))}
            </ul>
          </Alert>
        ) : null}
        {lesson.quiz ? (
          <>
            <QuizSettingsCard quiz={lesson.quiz} />
            <QuestionList {...ids} quiz={lesson.quiz} editable={editable} />
          </>
        ) : null}
        {lesson.homework ? (
          <>
            <HomeworkSettingsCard homework={lesson.homework} />
            <HomeworkBriefCard {...ids} homework={lesson.homework} editable={editable} />
            <CriteriaTable {...ids} homework={lesson.homework} editable={editable} />
          </>
        ) : null}
      </div>
      <LessonFormDialog
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
        lesson={lesson}
        units={version.units}
        onSave={async (values) => {
          const saved = await save.mutateAsync({ lessonId: lesson.id, values, from: lesson.type });
          // A quiz or homework turned into a video or a reading has nothing left to write here.
          if (!isGraded(saved)) void navigate(backPath, { replace: true });
        }}
      />
    </>
  );
}
