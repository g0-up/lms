-- Khóa học, phiên bản khóa học và danh sách phiên bản chặng có thứ tự của mỗi phiên bản.
CREATE TABLE courses (
  id uuid CONSTRAINT pk_courses PRIMARY KEY,
  code text NOT NULL CONSTRAINT uq_courses_code UNIQUE,
  name text NOT NULL,
  created_by uuid NOT NULL CONSTRAINT fk_courses_created_by REFERENCES users (id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT ck_courses_code CHECK (code ~ '^[A-Z0-9][A-Z0-9_-]{1,19}$')
);

CREATE TABLE course_versions (
  id uuid CONSTRAINT pk_course_versions PRIMARY KEY,
  course_id uuid NOT NULL CONSTRAINT fk_course_versions_course REFERENCES courses (id) ON DELETE RESTRICT,
  version_no integer NOT NULL CONSTRAINT ck_course_versions_version_no CHECK (version_no >= 1),
  status text NOT NULL CONSTRAINT ck_course_versions_status CHECK (status IN ('draft', 'published', 'archived')),
  title text NOT NULL,
  description text NOT NULL DEFAULT '',
  cloned_from_id uuid CONSTRAINT fk_course_versions_cloned_from REFERENCES course_versions (id) ON DELETE RESTRICT,
  published_at timestamptz,
  archived_at timestamptz,
  created_by uuid NOT NULL CONSTRAINT fk_course_versions_created_by REFERENCES users (id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_course_versions_course_version_no UNIQUE (course_id, version_no),
  CONSTRAINT ck_course_versions_published_at CHECK ((status = 'draft') = (published_at IS NULL)),
  CONSTRAINT ck_course_versions_archived_at CHECK ((status = 'archived') = (archived_at IS NOT NULL))
);
CREATE UNIQUE INDEX uq_course_versions_one_draft ON course_versions (course_id) WHERE status = 'draft';

-- stage_id denormalize để UNIQUE "một chặng một lần trong khóa"; FK ghép bảo đảm khớp stage_versions.stage_id.
CREATE TABLE course_version_stages (
  course_version_id uuid NOT NULL CONSTRAINT fk_cvs_course_version REFERENCES course_versions (id) ON DELETE RESTRICT,
  stage_version_id uuid NOT NULL,
  stage_id uuid NOT NULL CONSTRAINT fk_cvs_stage REFERENCES stages (id) ON DELETE RESTRICT,
  position integer NOT NULL CONSTRAINT ck_cvs_position CHECK (position >= 1),
  CONSTRAINT pk_course_version_stages PRIMARY KEY (course_version_id, stage_version_id),
  CONSTRAINT fk_cvs_stage_version FOREIGN KEY (stage_version_id, stage_id)
    REFERENCES stage_versions (id, stage_id) ON DELETE RESTRICT,
  CONSTRAINT uq_cvs_version_stage UNIQUE (course_version_id, stage_id),
  CONSTRAINT uq_cvs_version_position UNIQUE (course_version_id, position) DEFERRABLE INITIALLY IMMEDIATE
);
CREATE INDEX ix_cvs_stage_version_id ON course_version_stages (stage_version_id);
