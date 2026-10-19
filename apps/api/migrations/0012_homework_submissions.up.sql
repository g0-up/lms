-- Hạn nộp homework theo lớp, bài nộp, file nộp, kết quả chấm và điểm từng tiêu chí.
CREATE TABLE class_lesson_deadlines (
  class_id uuid NOT NULL CONSTRAINT fk_class_lesson_deadlines_class REFERENCES classes (id) ON DELETE RESTRICT,
  lesson_id uuid NOT NULL CONSTRAINT fk_class_lesson_deadlines_lesson REFERENCES lessons (id) ON DELETE RESTRICT,
  due_at timestamptz NOT NULL,
  set_by uuid NOT NULL CONSTRAINT fk_class_lesson_deadlines_set_by REFERENCES users (id) ON DELETE RESTRICT,
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT pk_class_lesson_deadlines PRIMARY KEY (class_id, lesson_id)
);

-- Mỗi lần nộp là một dòng bất biến; chỉ status đổi (pending → passed | rework | superseded).
CREATE TABLE submissions (
  id uuid CONSTRAINT pk_submissions PRIMARY KEY,
  class_member_id uuid NOT NULL CONSTRAINT fk_submissions_member REFERENCES class_members (id) ON DELETE RESTRICT,
  lesson_id uuid NOT NULL CONSTRAINT fk_submissions_lesson REFERENCES lessons (id) ON DELETE RESTRICT,
  attempt_no integer NOT NULL CONSTRAINT ck_submissions_attempt_no CHECK (attempt_no >= 1),
  repo_url text CONSTRAINT ck_submissions_repo_url CHECK (repo_url ~ '^https://'),
  commit_ref text,
  submitted_at timestamptz NOT NULL,
  is_late boolean NOT NULL DEFAULT false,
  status text NOT NULL CONSTRAINT ck_submissions_status CHECK (status IN ('pending', 'passed', 'rework', 'superseded')),
  CONSTRAINT ck_submissions_commit_needs_repo CHECK (commit_ref IS NULL OR repo_url IS NOT NULL),
  CONSTRAINT uq_submissions_member_lesson_no UNIQUE (class_member_id, lesson_id, attempt_no)
);
-- Mỗi (thành viên, học liệu) tối đa một bản chờ chấm; nộp lại chuyển bản cũ sang superseded trong cùng tx.
CREATE UNIQUE INDEX uq_submissions_one_pending ON submissions (class_member_id, lesson_id) WHERE status = 'pending';
-- Hàng chờ chấm: bản cũ nhất trước.
CREATE INDEX ix_submissions_pending ON submissions (submitted_at) WHERE status = 'pending';

CREATE TABLE submission_files (
  submission_id uuid NOT NULL CONSTRAINT fk_submission_files_submission REFERENCES submissions (id) ON DELETE RESTRICT,
  media_id uuid NOT NULL CONSTRAINT fk_submission_files_media REFERENCES media_files (id) ON DELETE RESTRICT,
  position integer NOT NULL,
  CONSTRAINT pk_submission_files PRIMARY KEY (submission_id, media_id),
  -- Một file chỉ thuộc một bản nộp.
  CONSTRAINT uq_submission_files_media UNIQUE (media_id),
  CONSTRAINT uq_submission_files_position UNIQUE (submission_id, position)
);

-- Append-only: mỗi lần chấm hoặc sửa điểm là một dòng; dòng mới nhất là kết quả hiện hành, các dòng trước là lịch sử.
CREATE TABLE submission_grades (
  id uuid CONSTRAINT pk_submission_grades PRIMARY KEY,
  submission_id uuid NOT NULL CONSTRAINT fk_submission_grades_submission REFERENCES submissions (id) ON DELETE RESTRICT,
  graded_by uuid NOT NULL CONSTRAINT fk_submission_grades_graded_by REFERENCES users (id) ON DELETE RESTRICT,
  total_points integer NOT NULL,
  max_points integer NOT NULL,
  result text NOT NULL CONSTRAINT ck_submission_grades_result CHECK (result IN ('passed', 'rework')),
  comment text NOT NULL CONSTRAINT ck_submission_grades_comment CHECK (btrim(comment) <> ''),
  graded_at timestamptz NOT NULL,
  CONSTRAINT ck_submission_grades_points CHECK (0 <= total_points AND total_points <= max_points)
);
CREATE INDEX ix_submission_grades_submission ON submission_grades (submission_id, graded_at DESC);

CREATE TABLE grade_criterion_scores (
  grade_id uuid NOT NULL CONSTRAINT fk_grade_criterion_scores_grade REFERENCES submission_grades (id) ON DELETE CASCADE,
  criterion_id uuid NOT NULL CONSTRAINT fk_grade_criterion_scores_criterion REFERENCES rubric_criteria (id) ON DELETE RESTRICT,
  points integer NOT NULL CONSTRAINT ck_grade_criterion_scores_points CHECK (points >= 0),
  comment text NOT NULL DEFAULT '',
  CONSTRAINT pk_grade_criterion_scores PRIMARY KEY (grade_id, criterion_id)
);
