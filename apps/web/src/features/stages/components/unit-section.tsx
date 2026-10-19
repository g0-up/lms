import { Pencil, Plus, Trash2, TriangleAlert } from "lucide-react";
import { Link } from "react-router";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import { IconButton } from "@/shared/ui/icon-button";
import { TypeIcon } from "@/shared/ui/type-icon";
import { lessonMeta, unitMeta } from "../model/lesson-form";
import type { Lesson, Unit } from "../model/schemas";
import { lessonPublishBlockers } from "../model/stage-rules";
import { ItemBody, ItemMeta, ItemTitle, ItemTitleLink, List, ListItem, Ord } from "./list";
import { OrderButtons } from "./order-buttons";

export interface UnitActions {
  onAddLesson: (unit: Unit) => void;
  onMoveUnit: (unit: Unit, delta: -1 | 1) => void;
  onRenameUnit: (unit: Unit) => void;
  onDeleteUnit: (unit: Unit) => void;
  onMoveLesson: (lesson: Lesson, delta: -1 | 1) => void;
  onEditLesson: (lesson: Lesson) => void;
  onDeleteLesson: (lesson: Lesson) => void;
}

export interface UnitSectionProps extends UnitActions {
  unit: Unit;
  /** 0-based place among the version's units. */
  index: number;
  unitCount: number;
  lessons: readonly Lesson[];
  /** Only a draft shows the add, reorder, rename and delete controls. */
  editable: boolean;
  /** Page of a quiz or homework, where its questions or criteria are written or read. */
  editorPath: (lesson: Lesson) => string;
}

/** Quizzes and homework have their own page; the ghost link names what is done there. */
function editorLinkLabel(lesson: Lesson, editable: boolean): string {
  if (!editable) return "Xem chi tiết";
  return lesson.type === "quiz" ? "Soạn câu hỏi" : "Soạn tiêu chí";
}

/**
 * One bài học of the stage editor: its head with the unit actions, then its lessons numbered "i.j".
 * A lesson moves across unit edges, even into an empty bài học, so its buttons are disabled only on the
 * first lesson of the first bài học and the last lesson of the last one.
 */
export function UnitSection({
  unit,
  index,
  unitCount,
  lessons,
  editable,
  editorPath,
  ...actions
}: UnitSectionProps) {
  const ord = index + 1;
  const headingId = `unit-${unit.id}`;
  return (
    <section aria-labelledby={headingId} className="border-b border-line last:border-b-0">
      <div className="flex flex-wrap items-center gap-3 border-b border-line bg-surface-50 px-6 py-3 max-[720px]:px-4">
        <Ord n={ord} />
        <div className="min-w-0 flex-1">
          <h3 id={headingId} className="text-md font-semibold text-navy-900">
            {unit.title}
          </h3>
          <div className="mt-0.5 flex flex-wrap gap-2 text-xs text-ink-3">
            <span>{unitMeta(unit)}</span>
            <span>key {unit.unitKey}</span>
          </div>
        </div>
        {editable ? (
          <div className="flex flex-wrap items-center justify-end gap-0.5">
            <Button
              variant="outline"
              size="sm"
              className="mr-1"
              onClick={() => {
                actions.onAddLesson(unit);
              }}
            >
              <Plus aria-hidden="true" /> Thêm học liệu
            </Button>
            <OrderButtons
              index={index}
              count={unitCount}
              labels={{ up: `Chuyển bài học lên: ${unit.title}`, down: `Chuyển bài học xuống: ${unit.title}` }}
              onMove={(delta) => {
                actions.onMoveUnit(unit, delta);
              }}
            />
            <IconButton
              aria-label={`Đổi tên bài học: ${unit.title}`}
              onClick={() => {
                actions.onRenameUnit(unit);
              }}
            >
              <Pencil aria-hidden="true" />
            </IconButton>
            <IconButton
              aria-label={`Xóa bài học: ${unit.title}`}
              onClick={() => {
                actions.onDeleteUnit(unit);
              }}
            >
              <Trash2 aria-hidden="true" />
            </IconButton>
          </div>
        ) : null}
      </div>
      <List>
        {lessons.length === 0 ? (
          <ListItem className="text-sm text-ink-3">
            {editable
              ? 'Bài học chưa có học liệu. Bấm "Thêm học liệu" để thêm video, bài đọc, quiz hoặc bài tập.'
              : "Bài học không có học liệu."}
          </ListItem>
        ) : (
          lessons.map((lesson, j) => {
            const graded = lesson.type === "quiz" || lesson.type === "homework";
            const incomplete = editable && lessonPublishBlockers(lesson).length > 0;
            return (
              <ListItem key={lesson.id} className="max-[720px]:flex-wrap">
                <Ord n={`${String(ord)}.${String(j + 1)}`} />
                <TypeIcon type={lesson.type} />
                <ItemBody>
                  <ItemTitle>
                    {graded ? <ItemTitleLink to={editorPath(lesson)}>{lesson.title}</ItemTitleLink> : lesson.title}
                  </ItemTitle>
                  <ItemMeta>
                    <span>{lessonMeta(lesson)}</span>
                    {lesson.required ? null : <Badge variant="subtle">Không bắt buộc</Badge>}
                    {incomplete ? (
                      <Badge variant="warn">
                        <TriangleAlert className="size-3" aria-hidden="true" /> Chưa đủ để phát hành
                      </Badge>
                    ) : null}
                    <span>key {lesson.lessonKey}</span>
                  </ItemMeta>
                </ItemBody>
                {graded ? (
                  <Button asChild variant="ghost" size="sm" className="shrink-0">
                    <Link to={editorPath(lesson)} aria-label={`${editorLinkLabel(lesson, editable)}: ${lesson.title}`}>
                      {editorLinkLabel(lesson, editable)}
                    </Link>
                  </Button>
                ) : null}
                {editable ? (
                  <div className="flex shrink-0 gap-0.5 max-[720px]:flex-[1_1_100%] max-[720px]:justify-end">
                    <OrderButtons
                      first={index === 0 && j === 0}
                      last={index === unitCount - 1 && j === lessons.length - 1}
                      labels={{ up: `Chuyển lên: ${lesson.title}`, down: `Chuyển xuống: ${lesson.title}` }}
                      onMove={(delta) => {
                        actions.onMoveLesson(lesson, delta);
                      }}
                    />
                    <IconButton
                      aria-label={`Sửa: ${lesson.title}`}
                      onClick={() => {
                        actions.onEditLesson(lesson);
                      }}
                    >
                      <Pencil aria-hidden="true" />
                    </IconButton>
                    <IconButton
                      aria-label={`Xóa: ${lesson.title}`}
                      onClick={() => {
                        actions.onDeleteLesson(lesson);
                      }}
                    >
                      <Trash2 aria-hidden="true" />
                    </IconButton>
                  </div>
                ) : null}
              </ListItem>
            );
          })
        )}
      </List>
    </section>
  );
}
