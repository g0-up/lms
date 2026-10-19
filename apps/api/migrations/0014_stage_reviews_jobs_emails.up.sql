-- Nhận xét chặng của giảng viên, khóa khử trùng lặp việc định kỳ và các mẫu email mới.
CREATE TABLE stage_reviews (
  id uuid CONSTRAINT pk_stage_reviews PRIMARY KEY,
  class_member_id uuid NOT NULL CONSTRAINT fk_stage_reviews_member REFERENCES class_members (id) ON DELETE RESTRICT,
  stage_version_id uuid NOT NULL CONSTRAINT fk_stage_reviews_stage_version REFERENCES stage_versions (id) ON DELETE RESTRICT,
  body text NOT NULL CONSTRAINT ck_stage_reviews_body CHECK (btrim(body) <> ''),
  author_id uuid NOT NULL CONSTRAINT fk_stage_reviews_author REFERENCES users (id) ON DELETE RESTRICT,
  -- Số lần lưu, cho nhãn "đã sửa n lần"; nội dung cũ nằm ở audit_logs.before.
  revision integer NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_stage_reviews_member_version UNIQUE (class_member_id, stage_version_id)
);

-- Việc định kỳ chèn khóa bằng INSERT ... ON CONFLICT DO NOTHING; chỉ bên chèn được mới enqueue email, nên mỗi khóa
-- (vd. grading_digest:<teacherId>:<yyyy-mm-dd>, deadline_reminder:<memberId>:<lessonId>:<dueAt>) chạy một lần.
CREATE TABLE scheduled_job_runs (
  run_key text CONSTRAINT pk_scheduled_job_runs PRIMARY KEY,
  ran_at timestamptz NOT NULL
);

ALTER TABLE email_outbox
  DROP CONSTRAINT ck_email_outbox_template,
  ADD CONSTRAINT ck_email_outbox_template CHECK (template IN (
    'invite', 'added', 'resend', 'password_reset',
    'homework_graded', 'quiz_reviewed', 'grading_digest', 'deadline_reminder'));
