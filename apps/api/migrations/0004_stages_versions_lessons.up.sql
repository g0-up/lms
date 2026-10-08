-- Chặng, phiên bản chặng, học liệu và bảng nối học liệu - media.
CREATE TABLE stages (
  id uuid CONSTRAINT pk_stages PRIMARY KEY,
  code text NOT NULL CONSTRAINT uq_stages_code UNIQUE,
  name text NOT NULL,
  created_by uuid NOT NULL CONSTRAINT fk_stages_created_by REFERENCES users (id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT ck_stages_code CHECK (code ~ '^[A-Z0-9][A-Z0-9_-]{1,19}$')
);

CREATE TABLE stage_versions (
  id uuid CONSTRAINT pk_stage_versions PRIMARY KEY,
  stage_id uuid NOT NULL CONSTRAINT fk_stage_versions_stage REFERENCES stages (id) ON DELETE RESTRICT,
  version_no integer NOT NULL CONSTRAINT ck_stage_versions_version_no CHECK (version_no >= 1),
  status text NOT NULL CONSTRAINT ck_stage_versions_status CHECK (status IN ('draft', 'published', 'archived')),
  title text NOT NULL,
  description text NOT NULL DEFAULT '',
  cloned_from_id uuid CONSTRAINT fk_stage_versions_cloned_from REFERENCES stage_versions (id) ON DELETE RESTRICT,
  published_at timestamptz,
  archived_at timestamptz,
  created_by uuid NOT NULL CONSTRAINT fk_stage_versions_created_by REFERENCES users (id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_stage_versions_stage_version_no UNIQUE (stage_id, version_no),
  -- Thừa về logic (id đã là PK) nhưng là đích bắt buộc cho FK ghép fk_cvs_stage_version ở 0005.
  CONSTRAINT uq_stage_versions_id_stage UNIQUE (id, stage_id),
  CONSTRAINT ck_stage_versions_published_at CHECK ((status = 'draft') = (published_at IS NULL)),
  CONSTRAINT ck_stage_versions_archived_at CHECK ((status = 'archived') = (archived_at IS NOT NULL))
);
CREATE UNIQUE INDEX uq_stage_versions_one_draft ON stage_versions (stage_id) WHERE status = 'draft';
CREATE INDEX ix_stage_versions_stage_status ON stage_versions (stage_id, status);

-- Xóa bản nháp chặng xóa luôn học liệu của nó (CASCADE); ứng dụng chỉ xóa khi header còn draft.
CREATE TABLE lessons (
  id uuid CONSTRAINT pk_lessons PRIMARY KEY,
  stage_version_id uuid NOT NULL CONSTRAINT fk_lessons_stage_version REFERENCES stage_versions (id) ON DELETE CASCADE,
  lesson_key text NOT NULL,
  position integer NOT NULL CONSTRAINT ck_lessons_position CHECK (position >= 1),
  title text NOT NULL,
  type text NOT NULL CONSTRAINT ck_lessons_type CHECK (type IN ('video', 'markdown')),
  required boolean NOT NULL DEFAULT true,
  markdown_source text,
  markdown_html text,
  video_media_id uuid CONSTRAINT fk_lessons_video_media REFERENCES media_files (id) ON DELETE RESTRICT,
  -- Trường "Thời lượng" của form soạn, hiển thị "Video · 18:24".
  duration_seconds integer CONSTRAINT ck_lessons_duration CHECK (duration_seconds IS NULL OR duration_seconds >= 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_lessons_version_key UNIQUE (stage_version_id, lesson_key),
  CONSTRAINT uq_lessons_version_position UNIQUE (stage_version_id, position) DEFERRABLE INITIALLY IMMEDIATE,
  CONSTRAINT ck_lessons_key CHECK (lesson_key ~ '^[a-z0-9][a-z0-9-]{1,39}$'),
  CONSTRAINT ck_lessons_type_content CHECK (
    (type = 'video' AND video_media_id IS NOT NULL AND markdown_source IS NULL)
    OR (type = 'markdown' AND markdown_source IS NOT NULL AND video_media_id IS NULL)
  )
);
CREATE INDEX ix_lessons_video_media_id ON lessons (video_media_id);

-- Mọi media một học liệu dùng (video và ảnh trong HTML đã render); nguồn kiểm quyền truy cập media.
CREATE TABLE lesson_media (
  lesson_id uuid NOT NULL CONSTRAINT fk_lesson_media_lesson REFERENCES lessons (id) ON DELETE CASCADE,
  media_id uuid NOT NULL CONSTRAINT fk_lesson_media_media REFERENCES media_files (id) ON DELETE RESTRICT,
  CONSTRAINT pk_lesson_media PRIMARY KEY (lesson_id, media_id)
);
CREATE INDEX ix_lesson_media_media_id ON lesson_media (media_id);
