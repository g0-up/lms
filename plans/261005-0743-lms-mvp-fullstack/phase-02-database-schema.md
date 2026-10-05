---
phase: 02
title: "Phase 2: Database schema và migration"
status: pending
priority: P1
effort: "2 ngày"
dependencies: [1]
---

# Phase 2: Database schema và migration

## Goal

Toàn bộ schema Postgres của MVP nằm trong 9 migration SQL nhúng vào binary (`lms migrate up|down|version`), đúng thứ tự và cột ở [plan.md §6](./plan.md). Tầng database **chỉ dùng ràng buộc khai báo** (PK, UNIQUE, CHECK, FK, partial unique index, DEFERRABLE); **không có `CREATE FUNCTION`, `CREATE TRIGGER`, `CREATE TYPE`** (quyết định người dùng, Session 2 D1). Bất biến "bản đã phát hành không thể sửa", "chỉ ghi bảng con khi header còn `draft`" và "không phát hành khi còn bài markdown chưa render" được bảo vệ ở **tầng ứng dụng** theo hợp đồng ba lớp (domain, khóa hàng `FOR UPDATE`, SQL có điều kiện) định nghĩa trong phase này và hiện thực ở Phase 03 (`db.ExecAffectOne`), 05, 06. <!-- Updated: Session 2 - D1 no DB functions/triggers --> Phase này là chủ sở hữu duy nhất của hợp đồng bảng `email_outbox` và của sổ tên constraint `platform/db/pgerr/constraints.go` mà mọi phase sau tham chiếu bằng hằng Go. <!-- Red Team: RT-02/RT-11 - outbox contract + constraint registry --> Lệnh `lms seed` nạp dữ liệu mẫu tương đương `prototype/seed.js` cho dev/e2e. Có bộ integration test SQL chứng minh từng ràng buộc khai báo và chứng minh schema không chứa function/trigger nào.

## Context & Requirements

- Spec: phiên bản chặng/khóa học (US-CM-02..05), lớp tham chiếu một `course_version` cố định, tiến độ `lesson_progress` theo `class_member` (US-ST-03), audit log, outbox email, session server-side, khóa đăng nhập (spec §8).
- Quyết định có sẵn: ID uuid v7 sinh từ app (`platform/ids`, Phase 03) nên cột PK `uuid PRIMARY KEY` không `DEFAULT`; mốc thời gian `timestamptz`; enum là `text` + `CHECK` (thêm giá trị chỉ cần `ALTER TABLE ... DROP/ADD CONSTRAINT`, không cần `ALTER TYPE` và tránh vấn đề trong transaction của go-migrate; báo cáo Go §3). `ON DELETE RESTRICT` mặc định; chỉ `lessons → stage_versions` và `lesson_media → lessons` CASCADE (xóa bản nháp chặng xóa luôn bài học nháp và liên kết media của nó). <!-- Red Team: RT-15 - lesson_media --> Vị trí sắp xếp UNIQUE `DEFERRABLE INITIALLY IMMEDIATE` để reorder trong một transaction bằng `SET CONSTRAINTS ... DEFERRED`.
- **Quyết định D1 (Session 2): không function, không trigger ở tầng DB.** Hàm built-in của Postgres (`now()`, `lower()`, `btrim()`, `pg_advisory_xact_lock()`) gọi trong câu SQL của ứng dụng vẫn dùng được; cấm là định nghĩa đối tượng mới bằng `CREATE FUNCTION`/`CREATE PROCEDURE`/`CREATE TRIGGER`/`CREATE TYPE`/`CREATE EXTENSION`. Lý do: toàn bộ logic nghiệp vụ nằm ở Go để test unit, debug và review tại một nơi; schema chỉ mô tả dữ liệu. Hậu quả chấp nhận: SQL thủ công qua `psql` có thể sửa bản `published`; quy tắc vận hành (chỉ sửa dữ liệu qua ứng dụng hoặc migration, backup trước) ghi ở `docs/database.md` và `docs/runbook.md`. <!-- Updated: Session 2 - D1 -->
- Email: lưu cả `email` (hiển thị) và `email_normalized` (lowercase + trim, UNIQUE). Chuẩn hóa làm ở app (Phase 03 `domain.Email`), DB chỉ `CHECK (email_normalized = lower(btrim(email_normalized)))`.
- Dữ liệu seed lấy nguyên văn từ `prototype/seed.js` (tên, email, mã, tiêu đề, markdown, mốc ngày tương đối). Mật khẩu từ env `SEED_PASSWORD`, không hardcode, không in ra log.
- Migration chạy trong transaction mặc định của go-migrate (mỗi file một tx); không dùng `CREATE INDEX CONCURRENTLY`.
- **Bất biến phiên bản ở tầng ứng dụng** (hợp đồng Phase 03/05/06 phải tuân theo, chi tiết ở mục "Bất biến phiên bản ở tầng ứng dụng" bên dưới): hàng con (`lessons`, `course_version_stages`, `lesson_media`) chỉ được ghi khi header cha còn `draft`; chuyển trạng thái chỉ chạm header. Phát hành chặng = render `markdown_html` cho mọi bài markdown (cha còn draft) → `UPDATE status` header. Mọi thao tác ghi lên một phiên bản phải mở `Transact`, `SELECT ... FOR UPDATE` header trước, rồi mọi câu UPDATE/DELETE/INSERT đều mang điều kiện trạng thái (`WHERE status = 'draft'`, `INSERT ... SELECT ... WHERE status = 'draft'`) và kiểm `RowsAffected = 1` qua `db.ExecAffectOne`. Không có `FOR UPDATE` thì hai transaction song song (thêm bài học và phát hành) có thể cùng commit. <!-- Red Team: RT-01 - bất biến thứ tự ghi --> <!-- Updated: Session 2 - D1 app-layer guard -->
- Enum trạng thái chuẩn: `classes.status` `draft|active|ended`; `class_members.status` `active|dropped|completed` (MVP không có endpoint ghi `completed`); `email_outbox.template` `invite|added|resend|password_reset`. <!-- Red Team: RT-03 - enum chuẩn --> <!-- Updated: Validation Session 1 - V3 completed -->
- `email_outbox` không lưu bí mật trong `payload`; mật khẩu tạm hoặc token đặt lại nằm ở `secret_enc bytea` mã hóa AES-256-GCM bằng `OUTBOX_SECRET_KEY` (gói `platform/secretbox`, Phase 03); worker giải mã lúc gửi rồi `SET secret_enc = NULL` khi `sent` hoặc `failed` cuối. <!-- Updated: Validation Session 1 - V1 secret_enc -->

## Architecture / Design

### Quy ước chung

- Tên file `NNNN_<slug>.up.sql` / `.down.sql`; `down` xóa đúng những gì `up` tạo theo thứ tự ngược.
- Tên constraint rõ ràng vì app map lỗi theo `ConstraintName` (Phase 03 `pgerr`): `uq_<table>_<cols>`, `fk_<table>_<col>`, `ck_<table>_<rule>`, `pk_<table>`. Mọi tên `uq_*`, `ck_*`, `fk_*` được khai báo thành hằng Go trong `apps/api/internal/platform/db/pgerr/constraints.go` (ví dụ `const UqUsersEmailNormalized = "uq_users_email_normalized"`, `UqClassesCode`, `UqStageVersionsOneDraft`, `UqLessonsVersionPosition`, `UqCvsVersionPosition`, `UqClassMembersClassUser`, `CkClassesStatus`, `FkCvsStageVersion`...); feature phase chỉ dùng hằng, không viết chuỗi. Integration test `constraints_test.go` so tập hằng với `pg_constraint` + index unique trong `pg_class` theo cả hai chiều. <!-- Red Team: RT-11 - sổ tên constraint --> <!-- Updated: Session 2 - D1 không còn tên constraint do trigger raise -->
- `created_at timestamptz NOT NULL DEFAULT now()`, `updated_at timestamptz NOT NULL DEFAULT now()` trên bảng có sửa đổi; app set `updated_at` khi UPDATE (không trigger).
- Migration `0000_init` của Phase 01 giữ lại (no-op) để embed không rỗng.
- Không `CREATE EXTENSION`: uuid v7 và token ngẫu nhiên sinh ở Go (`platform/ids`, `crypto/rand`), nên không cần `pgcrypto`/`uuid-ossp`. <!-- Updated: Session 2 - D1 -->

### 0001_schema_conventions

Một câu `COMMENT ON SCHEMA public IS 'LMS: enum = text + CHECK; id uuid v7 sinh ở ứng dụng; không CREATE TYPE/FUNCTION/TRIGGER/EXTENSION; bất biến phiên bản ở tầng ứng dụng (docs/database.md)';` để quy ước hiện ngay trong `\dn+`. Không `CREATE TYPE`, không `CREATE EXTENSION`. Down: `COMMENT ON SCHEMA public IS 'standard public schema';`. <!-- Updated: Session 2 - D1 thay 0001_extensions_and_enums -->

### 0002_users_sessions

```sql
CREATE TABLE users (
  id uuid PRIMARY KEY,
  email text NOT NULL,
  email_normalized text NOT NULL,
  full_name text NOT NULL CHECK (length(btrim(full_name)) BETWEEN 1 AND 120),
  role text NOT NULL CONSTRAINT ck_users_role CHECK (role IN ('admin','teacher','student')),
  status text NOT NULL CONSTRAINT ck_users_status CHECK (status IN ('invited','active','disabled')),
  password_hash text NOT NULL,
  must_change_password boolean NOT NULL DEFAULT false,
  temp_password_expires_at timestamptz,
  last_login_at timestamptz,
  last_active_at timestamptz,
  disabled_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_users_email_normalized UNIQUE (email_normalized),
  CONSTRAINT ck_users_email_normalized CHECK (email_normalized = lower(btrim(email_normalized))),
  CONSTRAINT ck_users_disabled_at CHECK ((status = 'disabled') = (disabled_at IS NOT NULL))
);
CREATE INDEX ix_users_role_status ON users (role, status);

CREATE TABLE sessions (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL CONSTRAINT fk_sessions_user REFERENCES users(id) ON DELETE RESTRICT,
  token_hash bytea NOT NULL CONSTRAINT uq_sessions_token_hash UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  last_seen_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz,
  ip inet,
  user_agent text
);
CREATE INDEX ix_sessions_user_id ON sessions (user_id);
CREATE INDEX ix_sessions_expires_at ON sessions (expires_at);
```

`token_hash` là SHA-256 (32 byte) của token opaque; `bytea` nhỏ hơn hex text và không cần so sánh collation.

### 0003_media_files

```sql
CREATE TABLE media_files (
  id uuid PRIMARY KEY,
  kind text NOT NULL CONSTRAINT ck_media_files_kind CHECK (kind IN ('video','image')),
  storage_key text NOT NULL CONSTRAINT uq_media_files_storage_key UNIQUE,
  original_name text NOT NULL,
  content_type text NOT NULL,
  size_bytes bigint NOT NULL CHECK (size_bytes > 0),
  status text NOT NULL CONSTRAINT ck_media_files_status CHECK (status IN ('pending','ready')),
  uploaded_by uuid NOT NULL CONSTRAINT fk_media_files_uploaded_by REFERENCES users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  ready_at timestamptz
);
CREATE INDEX ix_media_files_status_created ON media_files (status, created_at);
```

Thời lượng video không nằm ở `media_files` (không có ffprobe trong MVP) mà ở `lessons.duration_seconds` do người soạn nhập (0004). <!-- Red Team: RT-07 - duration_seconds chuyển sang lessons -->

### 0004_stages_versions_lessons

```sql
CREATE TABLE stages (
  id uuid PRIMARY KEY,
  code text NOT NULL CONSTRAINT uq_stages_code UNIQUE,
  name text NOT NULL,
  created_by uuid NOT NULL CONSTRAINT fk_stages_created_by REFERENCES users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT ck_stages_code CHECK (code ~ '^[A-Z0-9][A-Z0-9_-]{1,19}$')
);

CREATE TABLE stage_versions (
  id uuid PRIMARY KEY,
  stage_id uuid NOT NULL CONSTRAINT fk_stage_versions_stage REFERENCES stages(id) ON DELETE RESTRICT,
  version_no integer NOT NULL CHECK (version_no >= 1),
  status text NOT NULL CONSTRAINT ck_stage_versions_status CHECK (status IN ('draft','published','archived')),
  title text NOT NULL,
  description text NOT NULL DEFAULT '',
  cloned_from_id uuid CONSTRAINT fk_stage_versions_cloned_from REFERENCES stage_versions(id) ON DELETE RESTRICT,
  published_at timestamptz,
  archived_at timestamptz,
  created_by uuid NOT NULL CONSTRAINT fk_stage_versions_created_by REFERENCES users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_stage_versions_stage_version_no UNIQUE (stage_id, version_no),
  CONSTRAINT uq_stage_versions_id_stage UNIQUE (id, stage_id),   -- đích cho FK ghép của course_version_stages (0005), thay trigger đồng bộ stage_id
  CONSTRAINT ck_stage_versions_published_at CHECK ((status = 'draft') = (published_at IS NULL)),
  CONSTRAINT ck_stage_versions_archived_at CHECK ((status = 'archived') = (archived_at IS NOT NULL))
);
CREATE UNIQUE INDEX uq_stage_versions_one_draft ON stage_versions (stage_id) WHERE status = 'draft';
CREATE INDEX ix_stage_versions_stage_status ON stage_versions (stage_id, status);

CREATE TABLE lessons (
  id uuid PRIMARY KEY,
  stage_version_id uuid NOT NULL CONSTRAINT fk_lessons_stage_version REFERENCES stage_versions(id) ON DELETE CASCADE,
  lesson_key text NOT NULL,
  position integer NOT NULL CHECK (position >= 1),
  title text NOT NULL,
  type text NOT NULL CONSTRAINT ck_lessons_type CHECK (type IN ('video','markdown')),
  required boolean NOT NULL DEFAULT true,
  markdown_source text,
  markdown_html text,
  video_media_id uuid CONSTRAINT fk_lessons_video_media REFERENCES media_files(id) ON DELETE RESTRICT,
  duration_seconds integer CONSTRAINT ck_lessons_duration CHECK (duration_seconds IS NULL OR duration_seconds >= 0),  -- trường "Thời lượng" của form, hiển thị "Video · 18:24"
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

CREATE TABLE lesson_media (
  lesson_id uuid NOT NULL CONSTRAINT fk_lesson_media_lesson REFERENCES lessons(id) ON DELETE CASCADE,
  media_id uuid NOT NULL CONSTRAINT fk_lesson_media_media REFERENCES media_files(id) ON DELETE RESTRICT,
  CONSTRAINT pk_lesson_media PRIMARY KEY (lesson_id, media_id)
);
CREATE INDEX ix_lesson_media_media_id ON lesson_media (media_id);
```

`lesson_media` liệt kê mọi media một bài học dùng (`video_media_id` và các `<img src="/api/v1/media/<id>/content">` trong HTML đã render); Phase 05 ghi bảng này khi lưu bài học, Phase 05 `CanUserAccess` join `lesson_media` thay vì quét chuỗi HTML. <!-- Red Team: RT-15 - lesson_media --> <!-- Red Team: RT-07 - lessons.duration_seconds -->

`lesson_key` ổn định qua các phiên bản (clone giữ key) để báo cáo so sánh tiến độ giữa bản; `cloned_from_id` lưu nguồn clone (US-CM-03).

`uq_stage_versions_id_stage` thừa về mặt logic (vì `id` đã là PK) nhưng Postgres yêu cầu một UNIQUE đúng cột để FK ghép ở 0005 trỏ vào; đây là cách khai báo thuần túy để bảo đảm `course_version_stages.stage_id` luôn khớp `stage_versions.stage_id` mà không cần trigger. `ck_stage_versions_archived_at` khóa cặp `status`/`archived_at` ở mức hàng (trước đây do trigger kiểm). <!-- Updated: Session 2 - D1 -->

### 0005_courses_versions

```sql
CREATE TABLE courses (
  id uuid PRIMARY KEY,
  code text NOT NULL CONSTRAINT uq_courses_code UNIQUE,
  name text NOT NULL,
  created_by uuid NOT NULL CONSTRAINT fk_courses_created_by REFERENCES users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT ck_courses_code CHECK (code ~ '^[A-Z0-9][A-Z0-9_-]{1,19}$')
);

CREATE TABLE course_versions (
  id uuid PRIMARY KEY,
  course_id uuid NOT NULL CONSTRAINT fk_course_versions_course REFERENCES courses(id) ON DELETE RESTRICT,
  version_no integer NOT NULL CHECK (version_no >= 1),
  status text NOT NULL CONSTRAINT ck_course_versions_status CHECK (status IN ('draft','published','archived')),
  title text NOT NULL,
  description text NOT NULL DEFAULT '',
  cloned_from_id uuid CONSTRAINT fk_course_versions_cloned_from REFERENCES course_versions(id) ON DELETE RESTRICT,
  published_at timestamptz,
  archived_at timestamptz,
  created_by uuid NOT NULL CONSTRAINT fk_course_versions_created_by REFERENCES users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_course_versions_course_version_no UNIQUE (course_id, version_no),
  CONSTRAINT ck_course_versions_published_at CHECK ((status = 'draft') = (published_at IS NULL)),
  CONSTRAINT ck_course_versions_archived_at CHECK ((status = 'archived') = (archived_at IS NOT NULL))
);
CREATE UNIQUE INDEX uq_course_versions_one_draft ON course_versions (course_id) WHERE status = 'draft';

CREATE TABLE course_version_stages (
  course_version_id uuid NOT NULL CONSTRAINT fk_cvs_course_version REFERENCES course_versions(id) ON DELETE RESTRICT,
  stage_version_id uuid NOT NULL,
  stage_id uuid NOT NULL CONSTRAINT fk_cvs_stage REFERENCES stages(id) ON DELETE RESTRICT,
  position integer NOT NULL CHECK (position >= 1),
  CONSTRAINT pk_course_version_stages PRIMARY KEY (course_version_id, stage_version_id),
  CONSTRAINT fk_cvs_stage_version FOREIGN KEY (stage_version_id, stage_id)
    REFERENCES stage_versions (id, stage_id) ON DELETE RESTRICT,   -- FK ghép: stage_id phải khớp stage_versions.stage_id, không cần trigger
  CONSTRAINT uq_cvs_version_stage UNIQUE (course_version_id, stage_id),
  CONSTRAINT uq_cvs_version_position UNIQUE (course_version_id, position) DEFERRABLE INITIALLY IMMEDIATE
);
CREATE INDEX ix_cvs_stage_version_id ON course_version_stages (stage_version_id);
```

`stage_id` denormalize để UNIQUE "một chặng chỉ xuất hiện một lần trong khóa" khả thi. Ứng dụng (Phase 06 `CourseVersionRepo.Create/SaveDraft`) điền `stage_id` từ `StageVersionRef.StageID` đã nạp qua `StageVersionReader.Refs`; nếu điền sai, FK ghép `fk_cvs_stage_version` báo `23503`. Xóa một `stage_versions` đang được khóa học tham chiếu cũng báo `23503` với cùng tên constraint (Phase 05 map thành `ErrInUse`). Không có `stage_id` tự điền: cột `NOT NULL`, app bắt buộc gửi. <!-- Updated: Session 2 - D1 FK ghép thay trigger cvs_sync_stage_id -->

### 0006_classes_members_invitations

```sql
CREATE TABLE classes (
  id uuid PRIMARY KEY,
  code text NOT NULL CONSTRAINT uq_classes_code UNIQUE,
  name text NOT NULL,
  course_version_id uuid NOT NULL CONSTRAINT fk_classes_course_version REFERENCES course_versions(id) ON DELETE RESTRICT,
  teacher_id uuid NOT NULL CONSTRAINT fk_classes_teacher REFERENCES users(id) ON DELETE RESTRICT,
  status text NOT NULL CONSTRAINT ck_classes_status CHECK (status IN ('draft','active','ended')),  -- Red Team: RT-03 - closed → ended
  start_date date NOT NULL,
  end_date date NOT NULL,
  created_by uuid NOT NULL CONSTRAINT fk_classes_created_by REFERENCES users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT ck_classes_code CHECK (code ~ '^[a-z0-9][a-z0-9-]{1,29}$'),
  CONSTRAINT ck_classes_dates CHECK (end_date > start_date)
);
CREATE INDEX ix_classes_teacher_status ON classes (teacher_id, status);
CREATE INDEX ix_classes_course_version_id ON classes (course_version_id);

CREATE TABLE class_members (
  id uuid PRIMARY KEY,
  class_id uuid NOT NULL CONSTRAINT fk_class_members_class REFERENCES classes(id) ON DELETE RESTRICT,
  user_id uuid NOT NULL CONSTRAINT fk_class_members_user REFERENCES users(id) ON DELETE RESTRICT,
  status text NOT NULL CONSTRAINT ck_class_members_status CHECK (status IN ('active','dropped','completed')),  -- Updated: Validation Session 1 - V3 thêm completed
  joined_at timestamptz NOT NULL DEFAULT now(),
  dropped_at timestamptz,
  CONSTRAINT uq_class_members_class_user UNIQUE (class_id, user_id),
  CONSTRAINT ck_class_members_dropped CHECK ((status = 'dropped') = (dropped_at IS NOT NULL))
);
CREATE INDEX ix_class_members_class_status ON class_members (class_id, status);
CREATE INDEX ix_class_members_user_id ON class_members (user_id);

CREATE TABLE invitations (
  id uuid PRIMARY KEY,
  class_id uuid NOT NULL CONSTRAINT fk_invitations_class REFERENCES classes(id) ON DELETE RESTRICT,
  user_id uuid NOT NULL CONSTRAINT fk_invitations_user REFERENCES users(id) ON DELETE RESTRICT,
  kind text NOT NULL CONSTRAINT ck_invitations_kind CHECK (kind IN ('invite','added','resend')),
  email_outbox_id uuid,                      -- FK thêm ở 0008 vì email_outbox tạo sau
  invited_by uuid NOT NULL CONSTRAINT fk_invitations_invited_by REFERENCES users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_invitations_class_user_created ON invitations (class_id, user_id, created_at DESC);
```

Chuyển trạng thái lớp chỉ `draft→active`, `active→ended` (audit `class.activated`, `class.ended`). `completed` của `class_members` có trong CHECK để không cần migration sau nhưng MVP không có endpoint ghi giá trị này. <!-- Red Team: RT-03 - enum lớp --> <!-- Updated: Validation Session 1 - V3 -->

### 0007_lesson_progress

```sql
CREATE TABLE lesson_progress (
  class_member_id uuid NOT NULL CONSTRAINT fk_lesson_progress_member REFERENCES class_members(id) ON DELETE RESTRICT,
  lesson_id uuid NOT NULL CONSTRAINT fk_lesson_progress_lesson REFERENCES lessons(id) ON DELETE RESTRICT,
  first_opened_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT pk_lesson_progress PRIMARY KEY (class_member_id, lesson_id),
  CONSTRAINT ck_lesson_progress_order CHECK (completed_at IS NULL OR completed_at >= first_opened_at)
);
CREATE INDEX ix_lesson_progress_lesson_completed ON lesson_progress (lesson_id) WHERE completed_at IS NOT NULL;
```

Báo cáo lớp (US-TE-02) join `class_members(class_id)` → `lesson_progress(class_member_id)` (PK đã phủ) → `lessons`.

### 0008_email_outbox_password_resets_login_attempts

```sql
CREATE TABLE email_outbox (
  id uuid PRIMARY KEY,
  to_email text NOT NULL,
  template text NOT NULL CONSTRAINT ck_email_outbox_template CHECK (template IN ('invite','added','resend','password_reset')),  -- Red Team: RT-02 - bỏ account_disabled, tên khớp invitations.kind
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,            -- KHÔNG chứa bí mật
  secret_enc bytea,                                      -- Updated: Validation Session 1 - V1: AES-256-GCM(mật khẩu tạm | token đặt lại), NULL sau khi sent/failed
  status text NOT NULL DEFAULT 'queued' CONSTRAINT ck_email_outbox_status CHECK (status IN ('queued','sending','sent','failed')),
  attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  run_at timestamptz NOT NULL DEFAULT now(),
  locked_until timestamptz,
  last_error text,
  sent_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_email_outbox_claim ON email_outbox (run_at) WHERE status IN ('queued','sending');  -- Red Team: RT-02 - claim theo run_at, gồm hàng sending bị kẹt

ALTER TABLE invitations
  ADD CONSTRAINT fk_invitations_email_outbox FOREIGN KEY (email_outbox_id) REFERENCES email_outbox(id) ON DELETE RESTRICT;
CREATE INDEX ix_invitations_email_outbox_id ON invitations (email_outbox_id);

CREATE TABLE password_reset_tokens (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL CONSTRAINT fk_prt_user REFERENCES users(id) ON DELETE RESTRICT,
  token_hash bytea NOT NULL CONSTRAINT uq_prt_token_hash UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_prt_user_id ON password_reset_tokens (user_id);

CREATE TABLE login_attempts (
  id bigserial PRIMARY KEY,
  email_normalized text NOT NULL,
  ip inet,
  succeeded boolean NOT NULL,
  attempted_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_login_attempts_email_time ON login_attempts (email_normalized, attempted_at DESC);
```

Hợp đồng `email_outbox` (Phase 04 cài worker, Phase 07 chỉ đọc qua `mailer.OutboxStatusReader`): claim bằng `UPDATE email_outbox SET status='sending', locked_until=now()+interval '2 minutes' WHERE id IN (SELECT id FROM email_outbox WHERE (status='queued' AND run_at<=now()) OR (status='sending' AND locked_until<now()) ORDER BY run_at LIMIT $1 FOR UPDATE SKIP LOCKED) RETURNING *`; `attempts` chỉ tăng trong `MarkFailedAttempt` (≥3 → `failed`, `secret_enc=NULL`; ngược lại `queued` với backoff 1m/5m/15m); `MarkSent` đặt `sent`, `sent_at`, `secret_enc=NULL`. Không có `updated_at` vì mọi chuyển trạng thái đều ghi cột thời điểm riêng. <!-- Red Team: RT-02 - hợp đồng outbox duy nhất -->

`login_attempts` dùng `bigserial` vì là log append-only khối lượng lớn, không cần uuid. Chỉ cần index theo `email_normalized` vì khóa đăng nhập tính theo email (không khóa theo IP); worker (Phase 04) xóa hàng cũ hơn 30 ngày mỗi giờ. <!-- Updated: Validation Session 1 - V2 không khóa theo IP --> Down của 0008 phải `ALTER TABLE invitations DROP CONSTRAINT fk_invitations_email_outbox` trước khi `DROP TABLE email_outbox`.

### 0009_audit_logs

```sql
CREATE TABLE audit_logs (
  id uuid PRIMARY KEY,
  actor_id uuid CONSTRAINT fk_audit_logs_actor REFERENCES users(id) ON DELETE RESTRICT,  -- NULL cho hành động hệ thống (worker)
  action text NOT NULL,
  target_type text NOT NULL,
  target_id uuid,
  before jsonb,
  after jsonb,
  request_id text,
  at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ix_audit_logs_at ON audit_logs (at DESC);
CREATE INDEX ix_audit_logs_target ON audit_logs (target_type, target_id, at DESC);
CREATE INDEX ix_audit_logs_actor_at ON audit_logs (actor_id, at DESC);
```

`action` không CHECK để thêm hành động mới không cần migration; danh sách chuẩn giữ ở `platform/audit` (Phase 03) theo plan.md §7.

### Bất biến phiên bản ở tầng ứng dụng (hợp đồng cho Phase 03/05/06)

<!-- Updated: Session 2 - D1 - thay toàn bộ 0010_immutability_triggers -->

Không còn migration `0010`. Mọi bất biến liên hàng mà bản plan trước giao cho trigger nay là hợp đồng mã Go; Phase 02 định nghĩa hợp đồng (và ghi vào `docs/database.md`), Phase 03 cung cấp helper, Phase 05/06 hiện thực và test trên repository thật.

| Bất biến | Cơ chế cũ (đã bỏ) | Cơ chế mới ở ứng dụng | Phase hiện thực |
|---|---|---|---|
| Header `published`/`archived` không sửa, không xóa; chỉ `published → archived` | `forbid_non_draft_change()` + trigger trên `stage_versions`, `course_versions` | Domain: `StageVersion`/`CourseVersion` từ chối mutation khi `status != draft` (`ErrVersionImmutable`), `Archive()` chỉ từ `published`, `CanDelete()` chỉ `draft`. Repository: `SaveDraft` UPDATE header `WHERE id=$1 AND status='draft'`; `TransitionStatus` `WHERE id=$1 AND status=$from`; `Delete` `WHERE id=$1 AND status='draft'`; mọi câu qua `db.ExecAffectOne`, 0 dòng → `ErrVersionImmutable` (SaveDraft/Delete) hoặc `ErrInvalidTransition` (transition). DB: `ck_*_published_at`, `ck_*_archived_at` khóa cặp cột ở mức hàng | 03 (helper), 05, 06 |
| Bảng con (`lessons`, `lesson_media`, `course_version_stages`) chỉ ghi khi cha `draft` | `lessons_guard()`, `course_version_stages_guard()` | Repository ghi con bằng SQL có điều kiện trên cha: INSERT `INSERT INTO lessons (...) SELECT $2, $1, ... FROM stage_versions WHERE id = $1 AND status = 'draft'`; UPDATE `UPDATE lessons l SET ... FROM stage_versions sv WHERE l.id = $1 AND sv.id = l.stage_version_id AND sv.status = 'draft'`; DELETE `DELETE FROM lessons l USING stage_versions sv WHERE l.id = $1 AND sv.id = l.stage_version_id AND sv.status = 'draft'`; `lesson_media` ghi theo `lesson_id` ngay sau khi bài học đã qua điều kiện trên trong cùng tx; 0 dòng → `ErrVersionImmutable`. Xóa bản nháp vẫn dùng `ON DELETE CASCADE` của FK | 05, 06 |
| Không phát hành chặng khi còn bài markdown `markdown_html IS NULL` | `stage_versions_publish_guard` | Domain: `v.Publish(now)` trả `ErrNotRendered` khi còn bài chưa render (409 `INVALID_TRANSITION` "Phiên bản còn học liệu chưa render."). Repository `TransitionStatus(draft→published)`: `UPDATE stage_versions SET status='published', published_at=$2, updated_at=$2 WHERE id=$1 AND status='draft' AND NOT EXISTS (SELECT 1 FROM lessons WHERE stage_version_id=$1 AND type='markdown' AND markdown_html IS NULL)`; 0 dòng → `ErrInvalidTransition` | 05 |
| `course_version_stages.stage_id` khớp `stage_versions.stage_id` | `cvs_sync_stage_id()` tự điền và kiểm | App điền `stage_id` từ `StageVersionRef.StageID`; FK ghép `fk_cvs_stage_version (stage_version_id, stage_id) → stage_versions(id, stage_id)` báo `23503` khi lệch (khai báo, không trigger) | 02 (DDL), 06 |
| Hai transaction song song (thêm bài học và phát hành) không cùng commit | (trigger cũng không bảo đảm) | Mọi use case ghi: `Transact` → `ByIDForUpdate` header (`SELECT ... FOR UPDATE`) trước mọi đọc/ghi con. Khóa hàng tuần tự hóa; bên đến sau đọc trạng thái mới và nhận `ErrVersionImmutable` từ domain hoặc 0 dòng từ SQL có điều kiện | 05, 06 |

Helper dùng chung (Phase 03, gói `platform/db`):

```go
var ErrNoRowsAffected = errors.New("db: no rows affected")
// ExecAffectOne chạy câu lệnh và trả ErrNoRowsAffected khi RowsAffected() != 1.
// Mọi UPDATE/DELETE/INSERT...SELECT có điều kiện trạng thái bắt buộc đi qua hàm này.
func ExecAffectOne(ctx context.Context, ex Executor, query string, args ...any) error
```

Quy ước review (ghi trong `docs/database.md` và `docs/architecture.md`): trong `features/stages` và `features/courses`, không có câu UPDATE/DELETE lên `stage_versions`, `course_versions`, `lessons`, `lesson_media`, `course_version_stages` mà thiếu điều kiện trạng thái; không có câu INSERT vào ba bảng con mà không phải dạng `INSERT ... SELECT ... WHERE status = 'draft'`. Integration test của Phase 05/06 gọi repository trực tiếp lên bản `published` (bỏ qua service) để chứng minh lớp SQL có điều kiện tự đứng được khi lớp domain bị bỏ qua.

Giới hạn chấp nhận: SQL thủ công qua `psql` hoặc công cụ ngoài không bị chặn. Quy tắc vận hành: không sửa dữ liệu nghiệp vụ bằng SQL tay trên production; mọi sửa chữa đi qua ứng dụng hoặc migration có review và `pg_dump -Fc` trước (xem `docs/runbook.md`, Phase 14).

Test bảo vệ quyết định D1 (trong `migrations_test.go`): `SELECT count(*) FROM pg_trigger WHERE NOT tgisinternal` = 0; `SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace WHERE n.nspname = 'public'` = 0; `SELECT count(*) FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace WHERE n.nspname='public' AND t.typtype = 'e'` = 0; `SELECT count(*) FROM pg_extension WHERE extname <> 'plpgsql'` = 0. Thêm trigger/function sau này sẽ làm test đỏ.

### Seed (`lms seed`)

- Từ chối khi `cfg.IsProduction()` (exit 1, thông điệp "seed bị tắt ở production"). Yêu cầu `SEED_PASSWORD` (≥ 12 ký tự), nếu thiếu exit 1. Cờ chỉ có `--reset` và `--upload-sample` (thân lệnh `--upload-sample` do Phase 05 viết); không có env `SEED_UPLOAD_SAMPLES`. <!-- Red Team: RT-14 - cờ seed --> <!-- Updated: Validation Session 1 - V7 bỏ SEED_UPLOAD_SAMPLES -->
- Idempotent: chạy trong một transaction; nếu `users` đã có `quan.tran@goup.vn` thì in "đã seed" và thoát 0. `--reset` TRUNCATE mọi bảng theo thứ tự ngược FK rồi seed lại, chỉ khi `cfg.SeedResetAllowed()` (`APP_ENV` là `dev` hoặc `e2e`); giá trị khác → exit 1. `make seed-reset` (Phase 01) gọi `lms seed --reset`; E2E dùng target này. <!-- Red Team: RT-11/RT-14 - --reset chỉ dev|e2e -->
- Nội dung: mọi dữ liệu trong `prototype/seed.js` được chuyển sang Go struct literal trong `internal/seed/data.go` (users, stages/versions/lessons với markdown nguyên văn, course `BASIC` v1, 3 lớp `basic01`/`basic02` (`active`) và `basic03` (`draft`) đúng seed.js, enrollments, progress `done()/opened()`, invitations với email_outbox tương ứng trạng thái `sent|failed|queued` và `template` theo `invitations.kind` (`invite|added|resend`), audit entries với action theo bảng chuẩn `platform/audit` (Phase 03). Mốc thời gian tương đối tính từ `clock.Now()` giống seed.js (`−70d`, `+2d`…). Video `media_files` tạo bản ghi `status='ready'` với `storage_key='seed/<file>.mp4'`; `lessons.duration_seconds` đổi từ chuỗi `duration` (`'18:24'` → 1104) của seed.js; mỗi bài video có một hàng `lesson_media` trỏ `video_media_id`. File thật không tồn tại trong MinIO (dev chấp nhận video 404; `lms seed --upload-sample` do Phase 05 cài). Hàng outbox `sent|failed` có `secret_enc = NULL`; hàng `queued` được seed với `secret_enc = NULL` và `payload` không mật khẩu, Phase 04 (phase duy nhất ngoài Phase 02 được sửa `seed.go`, theo RT-14) bổ sung `secretbox.Seal(SEED_PASSWORD)` khi nối `platform/secretbox`. <!-- Red Team: RT-03/RT-07/RT-14/RT-15 - dữ liệu seed theo seed.js, duration_seconds, lesson_media --> <!-- Updated: Validation Session 1 - V1 secret_enc trong seed -->
- Seed ghi SQL trực tiếp (không qua repository) nên chèn header `stage_versions`/`course_versions` với trạng thái cuối (`published`, `published_at`) rồi chèn con; dữ liệu vẫn phải thỏa bất biến mà ứng dụng giả định: mọi bài markdown của bản `published` có `markdown_html` render sẵn từ `markdown.go` (cùng renderer + sanitizer của Phase 05 nếu đã có, nếu chưa thì HTML tĩnh viết tay trong `markdown.go` và Phase 05 thay bằng render thật), `course_version_stages.stage_id` điền đúng (FK ghép bắt lệch). <!-- Updated: Session 2 - D1 seed không còn publish guard -->
- Mật khẩu: user `active` nhận `argon2id(SEED_PASSWORD)`; user `invited` nhận cùng hash nhưng `must_change_password=true` và `temp_password_expires_at` theo seed.js; `disabled` giữ hash, `disabled_at = −15d`.
- Không in email/mật khẩu ra log; chỉ in số bản ghi mỗi bảng.

## Files to Create / Modify

```text
apps/api/migrations/0001_schema_conventions.{up,down}.sql          # Updated: Session 2 - D1 (thay 0001_extensions_and_enums)
apps/api/migrations/0002_users_sessions.{up,down}.sql
apps/api/migrations/0003_media_files.{up,down}.sql
apps/api/migrations/0004_stages_versions_lessons.{up,down}.sql     # + uq_stage_versions_id_stage, ck_stage_versions_archived_at
apps/api/migrations/0005_courses_versions.{up,down}.sql            # + ck_course_versions_archived_at, fk_cvs_stage_version là FK ghép
apps/api/migrations/0006_classes_members_invitations.{up,down}.sql
apps/api/migrations/0007_lesson_progress.{up,down}.sql
apps/api/migrations/0008_email_outbox_password_resets_login_attempts.{up,down}.sql
apps/api/migrations/0009_audit_logs.{up,down}.sql
apps/api/migrations/migrations_test.go          # //go:build integration: ràng buộc khai báo + test "không function/trigger"
apps/api/internal/platform/db/pgerr/constraints.go       # hằng Go cho mọi tên constraint/index  <!-- Red Team: RT-11 -->
apps/api/internal/platform/db/pgerr/constraints_test.go  # //go:build integration: so hằng với pg_constraint/pg_class  <!-- Red Team: RT-11 -->
apps/api/internal/seed/{seed.go,data.go,markdown.go,seed_test.go}
apps/api/cmd/lms/seed.go                        # sửa: gọi seed.Run(ctx, db, cfg, clock)
apps/api/internal/platform/testdb/testdb.go     # sửa nếu Phase 03 chưa có: Open(t), Reset(t) (xem Phase 03)
docs/database.md                                # mới: sơ đồ bảng, quyết định text+CHECK, quyết định D1 (không function/trigger) + bảng bất biến ở tầng ứng dụng, hợp đồng email_outbox, lesson_media, sổ tên constraint, cách thêm migration, quy trình rollback expand/contract (Phase 14 chỉ kiểm, không sửa)  <!-- Updated: Validation Session 1 - V7 docs/database.md do Phase 02 sở hữu --> <!-- Updated: Session 2 - D1 -->
```

Không có `apps/api/migrations/0010_*.sql`. `db.ExecAffectOne` và `db.ErrNoRowsAffected` thuộc Phase 03 (`platform/db`), không viết ở phase này. <!-- Updated: Session 2 - D1 -->

`argon2id` dùng cho seed: thêm `github.com/alexedwards/argon2id@v1.0.0` ở phase này (Phase 04 dùng lại qua `identity`). `pgerr/constraints.go` chỉ chứa hằng chuỗi (không import gì) để Phase 03 thêm `map.go` cùng gói mà không vòng phụ thuộc. <!-- Red Team: RT-11 --> Nếu Phase 03 chưa có `platform/testdb`, phase này tạo bản tối thiểu và Phase 03 hoàn thiện.

## Tasks & Steps

1. **Viết 9 cặp migration** theo DDL ở trên (0001 chỉ `COMMENT ON SCHEMA`; 0004 gồm `lessons.duration_seconds`, bảng `lesson_media`, `uq_stage_versions_id_stage`, `ck_stage_versions_archived_at`; 0005 FK ghép `fk_cvs_stage_version` và `ck_course_versions_archived_at`; 0006 enum `ended`/`completed`; 0008 `secret_enc`, template 4 giá trị, index claim theo `run_at`). <!-- Red Team: RT-02/RT-03/RT-07/RT-15 --> <!-- Updated: Session 2 - D1 --> Mỗi file `.up.sql` mở đầu bằng comment một dòng mô tả. `down` theo thứ tự ngược, dùng `DROP ... IF EXISTS`. Kiểm `lms migrate up` rồi `down` về 0 rồi `up` lại không lỗi (round-trip). Không có file nào chứa `CREATE FUNCTION`, `CREATE TRIGGER`, `CREATE TYPE`, `CREATE EXTENSION` (grep trong CI, xem Verification).
2. **`pgerr/constraints.go`**: sinh danh sách hằng từ chính các file `.sql` (grep `CONSTRAINT <tên>` và `CREATE UNIQUE INDEX <tên>`); `constraints_test.go` đọc `pg_constraint.conname` ∪ `pg_class.relname WHERE relkind='i' AND relname LIKE 'uq\_%'` và so hai chiều với tập hằng (không còn ngoại lệ nào vì không có tên do trigger raise). <!-- Red Team: RT-11 - sổ tên constraint --> <!-- Updated: Session 2 - D1 -->
3. **`migrations_test.go`** (`//go:build integration`): dùng `testdb.Open(t)`, fixture SQL tối thiểu (1 admin, 1 stage, 1 stage_version published, 1 lesson, 1 course_version published tham chiếu). Mỗi case là subtest, assert bằng `errors.As(err, &pgErr)` rồi so `pgErr.Code` và `pgErr.ConstraintName`. Phase này chỉ chứng minh **ràng buộc khai báo**; bất biến phiên bản (sửa bản published, ghi con của bản không nháp, publish guard, đua AddLesson vs Publish) được test ở Phase 05/06 `repository_pg_test.go` trên repository thật. <!-- Updated: Session 2 - D1 -->
   - Không function/trigger: bốn câu đếm ở mục "Bất biến phiên bản ở tầng ứng dụng" đều trả 0.
   - Insert hai `stage_versions` draft cùng `stage_id` → `23505`, `uq_stage_versions_one_draft`; tương tự `uq_course_versions_one_draft`.
   - `INSERT stage_versions` với `status='published'` nhưng `published_at IS NULL` → `23514`, `ck_stage_versions_published_at`; `status='archived'` nhưng `archived_at IS NULL` → `23514`, `ck_stage_versions_archived_at`; `status='draft'` kèm `archived_at` → `23514`. Tương tự `course_versions`.
   - `DELETE FROM stage_versions` draft được tham chiếu bởi `course_version_stages` → `23503`, `fk_cvs_stage_version`.
   - Xóa `stage_version` draft có lessons (không bị khóa học tham chiếu) → thành công, lessons bị cascade (count = 0).
   - `INSERT course_version_stages` với `stage_id` đúng → OK; `stage_id` sai (của chặng khác) → `23503`, `fk_cvs_stage_version`; `stage_id` NULL → `23502`. <!-- Updated: Session 2 - D1 FK ghép -->
   - Reorder: trong tx `SET CONSTRAINTS uq_lessons_version_position DEFERRED`, hoán vị position 1↔2 → commit OK; không DEFERRED → `23505`.
   - Insert `lessons` type video thiếu `video_media_id` → `23514`, `ck_lessons_type_content`.
   - `users` với `email_normalized` chứa chữ hoa → `23514`, `ck_users_email_normalized`; trùng email → `23505`, `uq_users_email_normalized`.
   - `classes` với `end_date <= start_date` → `23514`, `ck_classes_dates`.
   - Migration round-trip: `Migrate(down all)` rồi `Migrate(up)` → không lỗi, `Version()` = 9.
   - `classes.status='closed'` → `23514`, `ck_classes_status`; `status='ended'` → OK. `class_members.status='completed'` → OK. <!-- Red Team: RT-03 --> <!-- Updated: Validation Session 1 - V3 -->
   - `lesson_media`: xóa `lessons` draft → hàng `lesson_media` biến mất (CASCADE); `DELETE FROM media_files` đang được `lesson_media` tham chiếu → `23503`, `fk_lesson_media_media`. <!-- Red Team: RT-15 -->
   - `lessons.duration_seconds=-1` → `23514`, `ck_lessons_duration`. <!-- Red Team: RT-07 -->
   - `email_outbox.template='account_disabled'` → `23514`, `ck_email_outbox_template`; câu claim ở mục 0008 trả hàng `queued` đến hạn và hàng `sending` có `locked_until` quá khứ, bỏ qua hàng `failed`. <!-- Red Team: RT-02 -->
   - Mẫu SQL có điều kiện của hợp đồng chạy đúng trên schema thật (test thuần SQL, không cần repository): `UPDATE stage_versions SET title='x' WHERE id=$1 AND status='draft'` trên bản published → 0 dòng; `INSERT INTO lessons (...) SELECT ... FROM stage_versions WHERE id=$1 AND status='draft'` vào bản published → 0 dòng, vào bản draft → 1 dòng; `UPDATE ... SET status='published' ... WHERE ... AND NOT EXISTS (markdown chưa render)` khi còn `markdown_html IS NULL` → 0 dòng, sau khi `UPDATE lessons SET markdown_html` → 1 dòng. Chứng minh câu SQL hợp đồng là khả thi trước khi Phase 05/06 dùng. <!-- Updated: Session 2 - D1 -->
4. **`internal/seed`**: `data.go` chứa dữ liệu chuyển từ `prototype/seed.js` (đối chiếu từng dòng; markdown để trong `markdown.go` dạng raw string). `seed.go`: `Run(ctx, db *sqlx.DB, cfg config.Config, clk clock.Clock, opts Options) error` dùng `db.Transact` (Phase 03) hoặc `BeginTxx` nếu chạy trước Phase 03; thứ tự insert theo FK: users → media_files → stages → stage_versions → lessons → courses → course_versions → course_version_stages → classes → class_members → email_outbox → invitations → lesson_progress → audit_logs. Header `stage_versions`/`course_versions` chèn thẳng trạng thái cuối (`published`, `published_at` theo seed.js); bài markdown chèn kèm `markdown_html` render sẵn; `course_version_stages.stage_id` lấy từ chặng tương ứng. <!-- Updated: Session 2 - D1 --> `seed_test.go` (integration): chạy `Run` trên DB test → count users = 17, stages = 5, lessons = 16, classes = 3, class_members = 13 (An, Bích, Cường, Dũng, Hà, Khang, Thảo, Phong ×2 lớp, Linh, Minh, Nhàn, Quyên, Sơn, Tú → đếm lại từ `data.go`, test dùng số từ hằng trong package để không lệch), chạy lần hai → không đổi (idempotent); mọi bài markdown của bản published có `markdown_html IS NOT NULL`.
5. **`cmd/lms/seed.go`**: flag `--reset` (chỉ khi `cfg.SeedResetAllowed()`), khai báo flag `--upload-sample` nhưng thân lệnh do Phase 05 viết; kiểm `APP_ENV`, `SEED_PASSWORD`; gọi `seed.Run`; in bảng số bản ghi. <!-- Red Team: RT-14 -->
6. **Makefile**: `make seed` và `make seed-reset` ở gốc gọi target cùng tên trong `apps/api` với `WITH_ENV`; `make e2e` đã gọi `exec api /lms seed --reset` (Phase 01), cần `SEED_PASSWORD` trong compose profile `full` (`SEED_PASSWORD=Seed-Password-E2E-1`, chỉ e2e). <!-- Red Team: RT-14 - seed-reset -->
7. **`docs/database.md`**: sơ đồ quan hệ (Mermaid `erDiagram`, gồm `lesson_media`), bảng quyết định (text+CHECK, uuid v7 app-side, DEFERRABLE, hai CASCADE, FK ghép `stage_id`, **D1: không function/trigger/type/extension** kèm lý do và giới hạn chấp nhận), mục "Bất biến phiên bản ở tầng ứng dụng" chép bảng ở trên (cơ chế cũ → mới, câu SQL mẫu, `db.ExecAffectOne`, quy ước review), hợp đồng `email_outbox` (cột, câu claim, quy tắc `attempts`/`secret_enc`), sổ tên constraint `pgerr/constraints.go`, quy trình thêm migration (`lms migrate version`, đặt tên, viết down, expand/contract để image N-1 chạy được trên schema N, test, grep cấm `CREATE FUNCTION|TRIGGER|TYPE|EXTENSION`), xử lý `dirty` (sửa tay `schema_migrations` sau khi kiểm, không có lệnh `force`), rollback (`pg_dump -Fc` trước, `migrate down` chỉ cho migration cuối), quy tắc vận hành "không sửa dữ liệu nghiệp vụ bằng SQL tay trên production", cách chạy integration test. <!-- Red Team: RT-02/RT-04/RT-11 --> <!-- Updated: Validation Session 1 - V7 bỏ migrate force --> <!-- Updated: Session 2 - D1 -->

## Verification

```bash
cd apps/api
! grep -rniE 'CREATE (OR REPLACE )?(FUNCTION|PROCEDURE|TRIGGER|TYPE|EXTENSION)' migrations/   # D1: không function/trigger/type/extension  <!-- Updated: Session 2 - D1 -->
ls migrations/ | grep -c '0010_'                                               # 0
make migrate-up && go run ./cmd/lms migrate version                          # 9
go run ./cmd/lms migrate down && go run ./cmd/lms migrate version            # down toàn bộ về rỗng → in `no migration`, exit 0  <!-- Red Team: RT-04 -->
make migrate-up
TEST_DATABASE_URL=postgres://lms:lms@localhost:5432/lms_test?sslmode=disable go test -race -count=1 -tags integration ./migrations/... ./internal/seed/... ./internal/platform/db/pgerr/...   # gồm constraints_test  <!-- Red Team: RT-11 -->
SEED_PASSWORD='Dev-Password-123' APP_ENV=dev go run ./cmd/lms seed           # in số bản ghi, exit 0
SEED_PASSWORD='Dev-Password-123' APP_ENV=dev go run ./cmd/lms seed           # "đã seed", exit 0
APP_ENV=production go run ./cmd/lms seed; echo $?                            # 1
SEED_PASSWORD='Dev-Password-123' APP_ENV=dev go run ./cmd/lms seed --reset   # TRUNCATE rồi seed lại, exit 0  <!-- Red Team: RT-14 -->
SEED_PASSWORD='Dev-Password-123' APP_ENV=production go run ./cmd/lms seed --reset; echo $?   # 1  <!-- Red Team: RT-11 -->
psql "$DATABASE_URL" -Atc "SELECT count(*) FROM pg_trigger WHERE NOT tgisinternal"                                            # 0
psql "$DATABASE_URL" -Atc "SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public'"  # 0
psql "$DATABASE_URL" -Atc "SELECT array_length(conkey,1) FROM pg_constraint WHERE conname='fk_cvs_stage_version'"              # 2 (FK ghép)
psql "$DATABASE_URL" -c "SELECT conname FROM pg_constraint WHERE conname IN ('ck_classes_status','ck_email_outbox_template','fk_lesson_media_media','ck_lessons_duration','ck_stage_versions_archived_at','ck_course_versions_archived_at')"   # 6 dòng  <!-- Red Team: RT-02/RT-03/RT-07/RT-15 -->
psql "$DATABASE_URL" -c "SELECT count(*) FROM users"                                       # 17
golangci-lint run ./... && go vet -tags integration ./...
```

## Security notes

- Không hardcode mật khẩu/hash trong repo; `SEED_PASSWORD` bắt buộc từ env; seed từ chối ở production.
- `token_hash` (sessions, password_reset_tokens) chỉ lưu hash; token gốc không bao giờ chạm DB.
- `audit_logs.before/after` không chứa `password_hash`: app chịu trách nhiệm (Phase 03 `audit.Recorder` strip các khóa nhạy cảm); ghi chú trong `docs/database.md`.
- `login_attempts` lưu `email_normalized` và `ip`: dữ liệu cá nhân; worker (Phase 04) xóa hàng cũ hơn 30 ngày mỗi giờ, không làm ở phase này. <!-- Updated: Validation Session 1 - V2 dọn login_attempts ở worker -->
- `email_outbox.payload` không bao giờ chứa mật khẩu tạm hay token; bí mật chỉ nằm ở `secret_enc` (AES-256-GCM, khóa `OUTBOX_SECRET_KEY`), bị xóa khi `sent`/`failed`, không log, không trả qua API. <!-- Updated: Validation Session 1 - V1 -->
- Không có lớp chặn bất biến ở DB (D1). Mọi đường ghi lên phiên bản phải đi qua repository với `ByIDForUpdate` và SQL có điều kiện; tài khoản DB của ứng dụng là tài khoản duy nhất có quyền ghi trên production, không cấp quyền ghi ad-hoc cho người; mọi sửa chữa dữ liệu qua migration có review. <!-- Updated: Session 2 - D1 -->

## Risks & Rollback

| Rủi ro | Biện pháp |
|---|---|
| Một code path ghi lên bảng version bỏ quên điều kiện trạng thái → sửa được bản `published` | Hợp đồng SQL có điều kiện + `db.ExecAffectOne` là quy ước bắt buộc; integration test Phase 05/06 gọi repository trực tiếp lên bản published và test đua AddLesson vs Publish; checklist review trong `docs/database.md`; test SQL mẫu ở Task 3 chứng minh câu lệnh hợp đồng đúng trên schema thật <!-- Updated: Session 2 - D1 --> |
| SQL thủ công (psql, công cụ ngoài) sửa bản `published` vì không còn trigger | Chấp nhận theo D1; quy tắc vận hành trong runbook (không SQL tay trên prod, backup trước migration); tài khoản ghi duy nhất là của ứng dụng <!-- Updated: Session 2 - D1 --> |
| Ai đó thêm lại trigger/function ở migration sau | Test `pg_trigger`/`pg_proc`/`pg_type`/`pg_extension` = 0 trong `migrations_test.go` và grep cấm trong Verification/CI <!-- Updated: Session 2 - D1 --> |
| App điền `stage_id` sai cho `course_version_stages` | FK ghép `fk_cvs_stage_version` báo `23503`; `pgerr.Map` dịch thành 409 `IN_USE`/404 như các FK khác; Phase 06 test insert `stage_id` lệch |
| go-migrate chạy mỗi file trong một tx; lỗi giữa chừng để lại `dirty` | Không có lệnh `force`: kiểm schema bằng `psql`, sửa tay `schema_migrations` (`UPDATE schema_migrations SET dirty=false, version=<v>`), rồi `lms migrate up`; `docs/database.md` ghi quy trình <!-- Updated: Validation Session 1 - V7 bỏ migrate force --> |
| Hai transaction song song (thêm bài học và phát hành) cùng commit | Bất biến `SELECT ... FOR UPDATE` header trước mọi ghi (Phase 05/06 `ByIDForUpdate`); test đua trong `repository_pg_test.go` của Phase 05 chứng minh <!-- Red Team: RT-01 --> <!-- Updated: Session 2 - D1 test chuyển sang Phase 05 --> |
| Lệch dữ liệu seed so với prototype khiến E2E khác kỳ vọng | Test `seed_test.go` so số lượng với hằng trong `data.go`; review đối chiếu `seed.js` từng khối |
| Rollback | Dev: `docker compose down -v` xóa volume. Prod: hạ tag image với `MIGRATE_ON_START=false` (migration viết theo expand/contract nên image N-1 chạy trên schema N); `lms migrate down` chỉ sau `pg_dump -Fc` và chỉ cho migration cuối cùng <!-- Red Team: RT-04 - quy trình rollback --> |

## Success Criteria

- [ ] `lms migrate up` từ DB trống đạt version 9; `down` toàn bộ về 0 và `up` lại sạch. <!-- Updated: Session 2 - D1 -->
- [ ] Schema không có function, trigger, enum type hay extension do người dùng tạo: bốn câu đếm catalog trả 0; grep `CREATE (FUNCTION|PROCEDURE|TRIGGER|TYPE|EXTENSION)` trong `migrations/` rỗng; không tồn tại `0010_*`. <!-- Updated: Session 2 - D1 -->
- [ ] Toàn bộ subtest trong `migrations_test.go` xanh: `23505` cho draft thứ hai với đúng tên index; `23503` khi xóa `stage_version` đang được khóa học tham chiếu và khi `stage_id` lệch (`fk_cvs_stage_version`, FK ghép 2 cột); `23514` cho các CHECK gồm `ck_*_published_at`, `ck_*_archived_at`; CASCADE xóa nháp; mẫu SQL có điều kiện của hợp đồng trả 0/1 dòng đúng kỳ vọng. <!-- Updated: Session 2 - D1 -->
- [ ] Hợp đồng "Bất biến phiên bản ở tầng ứng dụng" (bảng cơ chế cũ → mới, câu SQL mẫu, `db.ExecAffectOne`, quy ước review, giới hạn chấp nhận) có trong phase này và trong `docs/database.md`; Phase 03/05/06 tham chiếu đúng mục này. <!-- Updated: Session 2 - D1 -->
- [ ] `ck_classes_status` nhận `draft|active|ended`, `ck_class_members_status` nhận `active|dropped|completed`, `ck_email_outbox_template` nhận `invite|added|resend|password_reset`; `email_outbox` có `secret_enc`, không có `updated_at`; index claim theo `(run_at) WHERE status IN ('queued','sending')`. <!-- Red Team: RT-02/RT-03 --> <!-- Updated: Validation Session 1 - V1/V3 -->
- [ ] `lesson_media` tồn tại với FK CASCADE/RESTRICT đúng; `lessons.duration_seconds` có CHECK ≥ 0. <!-- Red Team: RT-07/RT-15 -->
- [ ] `pgerr/constraints.go` có hằng cho mọi tên constraint; `constraints_test.go` xanh hai chiều, không ngoại lệ. <!-- Red Team: RT-11 --> <!-- Updated: Session 2 - D1 -->
- [ ] Reorder position trong tx với `SET CONSTRAINTS DEFERRED` thành công.
- [ ] `lms seed` idempotent, từ chối ở production, `--reset` chỉ chạy khi `APP_ENV` là `dev|e2e`, không hardcode mật khẩu, dữ liệu khớp `prototype/seed.js` (tên, email, mã, bài học kèm thời lượng, lớp, tiến độ, lời mời, audit); mọi bài markdown của bản published có `markdown_html`. <!-- Red Team: RT-11/RT-14 -->
- [ ] Mỗi FK là RESTRICT trừ `lessons.stage_version_id` và `lesson_media.lesson_id` CASCADE; mọi constraint có tên theo quy ước và có hằng trong `pgerr/constraints.go` để Phase 03 map được. <!-- Red Team: RT-11/RT-15 -->
- [ ] `docs/database.md` mô tả schema, quyết định D1 và bảng bất biến ở tầng ứng dụng, hợp đồng `email_outbox`, quy trình migration và rollback expand/contract, quy tắc vận hành không SQL tay; CI job `api` chạy integration test xanh. <!-- Updated: Validation Session 1 - V7 --> <!-- Updated: Session 2 - D1 -->
