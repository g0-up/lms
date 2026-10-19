import { EmptyState } from "@/shared/ui/empty-state";
import type { Lesson, StageVersion } from "../model/schemas";
import { UnitSection, type UnitActions } from "./unit-section";

export interface LessonListProps extends UnitActions {
  version: Pick<StageVersion, "units" | "lessons">;
  /** Page of a quiz or homework, where its questions or criteria are written or read. */
  editorPath: (lesson: Lesson) => string;
  /** Only a draft shows the add, reorder, edit and delete controls. */
  editable: boolean;
}

/** A version's lessons grouped by bài học, in order; lessons are flat in unit order, so each unit's slice is contiguous. */
export function LessonList({ version, editable, editorPath, ...actions }: LessonListProps) {
  const { units, lessons } = version;
  if (units.length === 0) {
    return (
      <EmptyState
        title="Chưa có bài học"
        text={
          editable
            ? "Tạo bài học trước, rồi thêm video, bài đọc, quiz hoặc bài tập vào từng bài học."
            : "Phiên bản này không có học liệu."
        }
      />
    );
  }
  return (
    <div>
      {units.map((unit, i) => {
        const own = lessons.filter((l) => l.unitId === unit.id);
        return (
          <UnitSection
            key={unit.id}
            unit={unit}
            index={i}
            unitCount={units.length}
            lessons={own}
            editable={editable}
            editorPath={editorPath}
            {...actions}
          />
        );
      })}
    </div>
  );
}
