-- Học liệu quiz và homework: loại học liệu mới, cấu hình quiz, câu hỏi, phương án, đề homework, tiêu chí chấm, file kèm.
-- Nội dung quiz/homework nằm ở bảng con nên hai loại mới không dùng cột video hay markdown của lessons.
ALTER TABLE lessons
  DROP CONSTRAINT ck_lessons_type,
  ADD CONSTRAINT ck_lessons_type CHECK (type IN ('video', 'markdown', 'quiz', 'homework')),
  DROP CONSTRAINT ck_lessons_type_content,
  ADD CONSTRAINT ck_lessons_type_content CHECK (
    (type = 'video' AND video_media_id IS NOT NULL AND markdown_source IS NULL)
    OR (type = 'markdown' AND markdown_source IS NOT NULL AND video_media_id IS NULL)
    OR (type IN ('quiz', 'homework') AND video_media_id IS NULL AND markdown_source IS NULL)
  );

-- document: file kèm đề homework và file học viên nộp.
ALTER TABLE media_files
  DROP CONSTRAINT ck_media_files_kind,
  ADD CONSTRAINT ck_media_files_kind CHECK (kind IN ('video', 'image', 'document'));

-- Một dòng mỗi học liệu quiz. Việc học viên có thấy đáp án hay không suy ra từ mode, không lưu cột riêng.
CREATE TABLE quiz_settings (
  lesson_id uuid CONSTRAINT pk_quiz_settings PRIMARY KEY
    CONSTRAINT fk_quiz_settings_lesson REFERENCES lessons (id) ON DELETE CASCADE,
  mode text NOT NULL CONSTRAINT ck_quiz_settings_mode CHECK (mode IN ('practice', 'assessment')),
  max_attempts integer,
  time_limit_seconds integer,
  shuffle boolean NOT NULL DEFAULT true,
  pass_percent integer,
  -- Luyện tập: không giới hạn lượt, không giờ, không ngưỡng. Đánh giá: cả ba bắt buộc (IS NOT NULL vì CHECK coi
  -- NULL là đạt).
  CONSTRAINT ck_quiz_settings_rules CHECK (
    (mode = 'practice' AND max_attempts IS NULL AND time_limit_seconds IS NULL AND pass_percent IS NULL)
    OR (mode = 'assessment'
        AND max_attempts IS NOT NULL AND max_attempts BETWEEN 1 AND 10
        AND time_limit_seconds IS NOT NULL AND time_limit_seconds BETWEEN 60 AND 10800
        AND pass_percent IS NOT NULL AND pass_percent BETWEEN 1 AND 100)
  )
);

CREATE TABLE questions (
  id uuid CONSTRAINT pk_questions PRIMARY KEY,
  lesson_id uuid NOT NULL CONSTRAINT fk_questions_lesson REFERENCES lessons (id) ON DELETE CASCADE,
  question_key text NOT NULL CONSTRAINT ck_questions_key CHECK (question_key ~ '^[a-z0-9][a-z0-9-]{1,39}$'),
  type text NOT NULL CONSTRAINT ck_questions_type CHECK (type IN ('single', 'multi')),
  body_source text NOT NULL,
  body_html text,
  explanation_source text NOT NULL DEFAULT '',
  explanation_html text,
  points integer NOT NULL DEFAULT 1 CONSTRAINT ck_questions_points CHECK (points BETWEEN 1 AND 100),
  position integer NOT NULL,
  CONSTRAINT uq_questions_lesson_key UNIQUE (lesson_id, question_key),
  CONSTRAINT uq_questions_lesson_position UNIQUE (lesson_id, position) DEFERRABLE INITIALLY IMMEDIATE
);

CREATE TABLE question_options (
  id uuid CONSTRAINT pk_question_options PRIMARY KEY,
  question_id uuid NOT NULL CONSTRAINT fk_question_options_question REFERENCES questions (id) ON DELETE CASCADE,
  body text NOT NULL CONSTRAINT ck_question_options_body CHECK (char_length(btrim(body)) BETWEEN 1 AND 500),
  is_correct boolean NOT NULL DEFAULT false,
  position integer NOT NULL,
  CONSTRAINT uq_question_options_question_position UNIQUE (question_id, position) DEFERRABLE INITIALLY IMMEDIATE
);

-- Một dòng mỗi học liệu homework.
CREATE TABLE homework_settings (
  lesson_id uuid CONSTRAINT pk_homework_settings PRIMARY KEY
    CONSTRAINT fk_homework_settings_lesson REFERENCES lessons (id) ON DELETE CASCADE,
  brief_source text NOT NULL,
  brief_html text,
  pass_percent integer NOT NULL DEFAULT 70 CONSTRAINT ck_homework_settings_pass_percent CHECK (pass_percent BETWEEN 1 AND 100)
);

CREATE TABLE rubric_criteria (
  id uuid CONSTRAINT pk_rubric_criteria PRIMARY KEY,
  lesson_id uuid NOT NULL CONSTRAINT fk_rubric_criteria_lesson REFERENCES lessons (id) ON DELETE CASCADE,
  description text NOT NULL,
  max_points integer NOT NULL CONSTRAINT ck_rubric_criteria_max_points CHECK (max_points BETWEEN 1 AND 100),
  position integer NOT NULL,
  CONSTRAINT uq_rubric_criteria_lesson_position UNIQUE (lesson_id, position) DEFERRABLE INITIALLY IMMEDIATE
);

-- Thứ tự hiển thị file kèm đề homework; quyền xem file đi qua lesson_media như mọi media của học liệu.
CREATE TABLE homework_attachments (
  lesson_id uuid NOT NULL CONSTRAINT fk_homework_attachments_lesson REFERENCES lessons (id) ON DELETE CASCADE,
  media_id uuid NOT NULL CONSTRAINT fk_homework_attachments_media REFERENCES media_files (id) ON DELETE RESTRICT,
  position integer NOT NULL,
  CONSTRAINT pk_homework_attachments PRIMARY KEY (lesson_id, media_id),
  CONSTRAINT uq_homework_attachments_lesson_position UNIQUE (lesson_id, position)
);
