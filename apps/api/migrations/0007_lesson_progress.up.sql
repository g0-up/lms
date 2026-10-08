-- Tiến độ theo cặp (thành viên lớp, học liệu); phần trăm luôn tính, không lưu.
CREATE TABLE lesson_progress (
  class_member_id uuid NOT NULL CONSTRAINT fk_lesson_progress_member REFERENCES class_members (id) ON DELETE RESTRICT,
  lesson_id uuid NOT NULL CONSTRAINT fk_lesson_progress_lesson REFERENCES lessons (id) ON DELETE RESTRICT,
  first_opened_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT pk_lesson_progress PRIMARY KEY (class_member_id, lesson_id),
  CONSTRAINT ck_lesson_progress_order CHECK (completed_at IS NULL OR completed_at >= first_opened_at)
);
CREATE INDEX ix_lesson_progress_lesson_completed ON lesson_progress (lesson_id) WHERE completed_at IS NOT NULL;
