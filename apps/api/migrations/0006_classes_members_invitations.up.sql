-- Lớp học (tham chiếu một phiên bản khóa học cố định), thành viên lớp và lịch sử lời mời.
CREATE TABLE classes (
  id uuid CONSTRAINT pk_classes PRIMARY KEY,
  code text NOT NULL CONSTRAINT uq_classes_code UNIQUE,
  name text NOT NULL,
  course_version_id uuid NOT NULL CONSTRAINT fk_classes_course_version REFERENCES course_versions (id) ON DELETE RESTRICT,
  teacher_id uuid NOT NULL CONSTRAINT fk_classes_teacher REFERENCES users (id) ON DELETE RESTRICT,
  status text NOT NULL CONSTRAINT ck_classes_status CHECK (status IN ('draft', 'active', 'ended')),
  start_date date NOT NULL,
  end_date date NOT NULL,
  created_by uuid NOT NULL CONSTRAINT fk_classes_created_by REFERENCES users (id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT ck_classes_code CHECK (code ~ '^[a-z0-9][a-z0-9-]{1,29}$'),
  CONSTRAINT ck_classes_dates CHECK (end_date > start_date)
);
CREATE INDEX ix_classes_teacher_status ON classes (teacher_id, status);
CREATE INDEX ix_classes_course_version_id ON classes (course_version_id);

-- 'completed' có sẵn trong CHECK để khỏi cần migration sau; MVP chưa có luồng ghi giá trị này.
CREATE TABLE class_members (
  id uuid CONSTRAINT pk_class_members PRIMARY KEY,
  class_id uuid NOT NULL CONSTRAINT fk_class_members_class REFERENCES classes (id) ON DELETE RESTRICT,
  user_id uuid NOT NULL CONSTRAINT fk_class_members_user REFERENCES users (id) ON DELETE RESTRICT,
  status text NOT NULL CONSTRAINT ck_class_members_status CHECK (status IN ('active', 'dropped', 'completed')),
  joined_at timestamptz NOT NULL DEFAULT now(),
  dropped_at timestamptz,
  CONSTRAINT uq_class_members_class_user UNIQUE (class_id, user_id),
  CONSTRAINT ck_class_members_dropped CHECK ((status = 'dropped') = (dropped_at IS NOT NULL))
);
CREATE INDEX ix_class_members_class_status ON class_members (class_id, status);
CREATE INDEX ix_class_members_user_id ON class_members (user_id);

CREATE TABLE invitations (
  id uuid CONSTRAINT pk_invitations PRIMARY KEY,
  class_id uuid NOT NULL CONSTRAINT fk_invitations_class REFERENCES classes (id) ON DELETE RESTRICT,
  user_id uuid NOT NULL CONSTRAINT fk_invitations_user REFERENCES users (id) ON DELETE RESTRICT,
  kind text NOT NULL CONSTRAINT ck_invitations_kind CHECK (kind IN ('invite', 'added', 'resend')),
  -- FK tới email_outbox thêm ở 0008 vì bảng đó tạo sau.
  email_outbox_id uuid,
  invited_by uuid NOT NULL CONSTRAINT fk_invitations_invited_by REFERENCES users (id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_invitations_class_user_created ON invitations (class_id, user_id, created_at DESC);
