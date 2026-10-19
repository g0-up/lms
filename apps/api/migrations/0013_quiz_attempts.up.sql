-- Lượt làm quiz, câu trả lời, xác nhận và nhận xét của giảng viên, lượt cấp thêm.
CREATE TABLE quiz_attempts (
  id uuid CONSTRAINT pk_quiz_attempts PRIMARY KEY,
  class_member_id uuid NOT NULL CONSTRAINT fk_quiz_attempts_member REFERENCES class_members (id) ON DELETE RESTRICT,
  lesson_id uuid NOT NULL CONSTRAINT fk_quiz_attempts_lesson REFERENCES lessons (id) ON DELETE RESTRICT,
  attempt_no integer NOT NULL,
  -- Thứ tự câu và phương án học viên thấy, chốt lúc bắt đầu lượt (trộn một lần): option_order là
  -- {"<question_id>": ["<option_id>", ...]}.
  question_order uuid[] NOT NULL,
  option_order jsonb NOT NULL,
  started_at timestamptz NOT NULL,
  -- NULL khi quiz không giới hạn thời gian.
  deadline_at timestamptz,
  submitted_at timestamptz,
  auto_submitted boolean NOT NULL DEFAULT false,
  score integer,
  max_score integer NOT NULL,
  passed boolean,
  -- NULL: không cần xác nhận (luyện tập chưa nhận xét). pending chỉ có ở lượt đánh giá đã nộp.
  review_status text CONSTRAINT ck_quiz_attempts_review_status CHECK (review_status IN ('pending', 'confirmed', 'superseded')),
  CONSTRAINT uq_quiz_attempts_member_lesson_no UNIQUE (class_member_id, lesson_id, attempt_no),
  CONSTRAINT ck_quiz_attempts_scored CHECK ((submitted_at IS NULL) = (score IS NULL)),
  CONSTRAINT ck_quiz_attempts_review_after_submit CHECK (review_status IS NULL OR submitted_at IS NOT NULL)
);
-- Mỗi (thành viên, quiz) tối đa một lượt đang làm và một lượt chờ xác nhận.
CREATE UNIQUE INDEX uq_quiz_attempts_one_open ON quiz_attempts (class_member_id, lesson_id) WHERE submitted_at IS NULL;
CREATE UNIQUE INDEX uq_quiz_attempts_one_pending ON quiz_attempts (class_member_id, lesson_id) WHERE review_status = 'pending';
-- Worker tự nộp lượt quá giờ và hàng chờ xác nhận.
CREATE INDEX ix_quiz_attempts_open_deadline ON quiz_attempts (deadline_at) WHERE submitted_at IS NULL;
CREATE INDEX ix_quiz_attempts_pending ON quiz_attempts (submitted_at) WHERE review_status = 'pending';

CREATE TABLE quiz_attempt_answers (
  attempt_id uuid NOT NULL CONSTRAINT fk_quiz_attempt_answers_attempt REFERENCES quiz_attempts (id) ON DELETE CASCADE,
  question_id uuid NOT NULL CONSTRAINT fk_quiz_attempt_answers_question REFERENCES questions (id) ON DELETE RESTRICT,
  selected_option_ids uuid[] NOT NULL,
  -- NULL tới khi lượt được nộp và chấm.
  is_correct boolean,
  points_awarded integer,
  saved_at timestamptz NOT NULL,
  CONSTRAINT pk_quiz_attempt_answers PRIMARY KEY (attempt_id, question_id)
);

-- Append-only: mỗi lần lưu nhận xét là một dòng; dòng mới nhất là nhận xét hiện hành, các dòng trước là lịch sử.
CREATE TABLE quiz_attempt_reviews (
  id uuid CONSTRAINT pk_quiz_attempt_reviews PRIMARY KEY,
  attempt_id uuid NOT NULL CONSTRAINT fk_quiz_attempt_reviews_attempt REFERENCES quiz_attempts (id) ON DELETE RESTRICT,
  reviewer_id uuid NOT NULL CONSTRAINT fk_quiz_attempt_reviews_reviewer REFERENCES users (id) ON DELETE RESTRICT,
  comment text NOT NULL CONSTRAINT ck_quiz_attempt_reviews_comment CHECK (btrim(comment) <> ''),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_quiz_attempt_reviews_attempt ON quiz_attempt_reviews (attempt_id, created_at DESC);

CREATE TABLE quiz_review_notes (
  review_id uuid NOT NULL CONSTRAINT fk_quiz_review_notes_review REFERENCES quiz_attempt_reviews (id) ON DELETE CASCADE,
  question_id uuid NOT NULL CONSTRAINT fk_quiz_review_notes_question REFERENCES questions (id) ON DELETE RESTRICT,
  note text NOT NULL CONSTRAINT ck_quiz_review_notes_note CHECK (btrim(note) <> ''),
  CONSTRAINT uq_quiz_review_notes_review_question UNIQUE (review_id, question_id)
);

CREATE TABLE quiz_attempt_grants (
  id uuid CONSTRAINT pk_quiz_attempt_grants PRIMARY KEY,
  class_member_id uuid NOT NULL CONSTRAINT fk_quiz_attempt_grants_member REFERENCES class_members (id) ON DELETE RESTRICT,
  lesson_id uuid NOT NULL CONSTRAINT fk_quiz_attempt_grants_lesson REFERENCES lessons (id) ON DELETE RESTRICT,
  extra_attempts integer NOT NULL CONSTRAINT ck_quiz_attempt_grants_extra CHECK (extra_attempts BETWEEN 1 AND 3),
  reason text NOT NULL CONSTRAINT ck_quiz_attempt_grants_reason CHECK (btrim(reason) <> ''),
  granted_by uuid NOT NULL CONSTRAINT fk_quiz_attempt_grants_granted_by REFERENCES users (id) ON DELETE RESTRICT,
  granted_at timestamptz NOT NULL
);
CREATE INDEX ix_quiz_attempt_grants_member_lesson ON quiz_attempt_grants (class_member_id, lesson_id);
