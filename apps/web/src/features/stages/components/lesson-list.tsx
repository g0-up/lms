import { Pencil, Trash2 } from "lucide-react";
import { Badge } from "@/shared/ui/badge";
import { EmptyState } from "@/shared/ui/empty-state";
import { IconButton } from "@/shared/ui/icon-button";
import { TypeIcon } from "@/shared/ui/type-icon";
import { lessonMeta } from "../model/lesson-form";
import type { Lesson } from "../model/schemas";
import { ItemBody, ItemMeta, ItemTitle, List, ListItem, Ord } from "./list";
import { OrderButtons } from "./order-buttons";

export interface LessonListProps {
  lessons: readonly Lesson[];
  /** Only a draft shows the reorder, edit and delete controls. */
  editable: boolean;
  onMove: (index: number, delta: -1 | 1) => void;
  onEdit: (lesson: Lesson) => void;
  onDelete: (lesson: Lesson) => void;
}

export function LessonList({ lessons, editable, onMove, onEdit, onDelete }: LessonListProps) {
  if (lessons.length === 0) {
    return (
      <EmptyState
        title="Chưa có học liệu"
        text={editable ? "Thêm video hoặc bài markdown để phát hành chặng này." : "Phiên bản này không có học liệu."}
      />
    );
  }
  return (
    <List>
      {lessons.map((lesson, i) => (
        <ListItem key={lesson.id}>
          <Ord n={i + 1} />
          <TypeIcon type={lesson.type} />
          <ItemBody>
            <ItemTitle>{lesson.title}</ItemTitle>
            <ItemMeta>
              <span>{lessonMeta(lesson)}</span>
              {lesson.required ? null : <Badge variant="subtle">Không bắt buộc</Badge>}
              <span>key {lesson.lessonKey}</span>
            </ItemMeta>
          </ItemBody>
          {editable ? (
            <div className="flex shrink-0 gap-0.5">
              <OrderButtons
                index={i}
                count={lessons.length}
                onMove={(delta) => {
                  onMove(i, delta);
                }}
              />
              <IconButton
                aria-label="Sửa"
                onClick={() => {
                  onEdit(lesson);
                }}
              >
                <Pencil aria-hidden="true" />
              </IconButton>
              <IconButton
                aria-label="Xóa"
                onClick={() => {
                  onDelete(lesson);
                }}
              >
                <Trash2 aria-hidden="true" />
              </IconButton>
            </div>
          ) : null}
        </ListItem>
      ))}
    </List>
  );
}
