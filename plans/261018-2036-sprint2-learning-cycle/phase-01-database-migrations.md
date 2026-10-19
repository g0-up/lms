---
phase: 1
title: "Migrations và ràng buộc cho Sprint 2"
status: completed
priority: P0
effort: "2.5d"
dependencies: []
---

# Phase 01: Migrations và ràng buộc

## Mục tiêu

Tạo toàn bộ bảng của Phụ lục A trong năm migration expand-only (0010–0014) để mọi phase sau chỉ viết code Go và web, không phải xếp hàng số migration. Dữ liệu MVP được chuyển sang mô hình bài học theo giả định OQ-15 mà không đổi % tiến độ của học viên nào.

## Context

- Spec: Phụ lục A, §5, FR-78, FR-62, FR-71, FR-75, OQ-15.
- `docs/database.md` (D1, hợp đồng SQL có điều kiện, cách thêm migration), `apps/api/migrations/0004_stages_versions_lessons.up.sql`, `0003_media_files.up.sql`, `0008_email_outbox_…up.sql`.
- [scout-api](reports/scout-api.md) mục 1, 11.

## Thiết kế DDL

Quy ước giữ nguyên MVP: tên ràng buộc `pk_/uq_/ck_/fk_/ix_`, enum là `text` kèm CHECK, thứ tự dùng UNIQUE `DEFERRABLE INITIALLY IMMEDIATE`, không trigger hay function do người dùng định nghĩa. `gen_random_uuid()` (built-in từ PG13) chỉ dùng cho backfill trong 0010.

### 0010_stage_units

```sql
CREATE TABLE stage_units (
  id uuid CONSTRAINT pk_stage_units PRIMARY KEY,
  stage_version_id uuid NOT NULL CONSTRAINT fk_stage_units_stage_version REFERENCES stage_versions (id) ON DELETE CASCADE,
  unit_key text NOT NULL CONSTRAINT ck_stage_units_key CHECK (unit_key ~ '^[a-z0-9][a-z0-9-]{1,39}$'),
  title text NOT NULL CONSTRAINT ck_stage_units_title CHECK (char_length(btrim(title)) BETWEEN 1 AND 120),
  position integer NOT NULL CONSTRAINT ck_stage_units_position CHECK (position >= 1),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT uq_stage_units_version_key UNIQUE (stage_version_id, unit_key),
  CONSTRAINT uq_stage_units_version_position UNIQUE (stage_version_id, position) DEFERRABLE INITIALLY IMMEDIATE,
  CONSTRAINT uq_stage_units_id_version UNIQUE (id, stage_version_id)  -- đích FK ghép
);
ALTER TABLE lessons ADD COLUMN unit_id uuid;
-- OQ-15: mỗi phiên bản có sẵn một bài học mặc định chứa mọi học liệu, giữ position cũ.
INSERT INTO stage_units (id, stage_version_id, unit_key, title, position)
  SELECT gen_random_uuid(), sv.id, 'noi-dung-chang', 'Nội dung chặng', 1 FROM stage_versions sv;
UPDATE lessons l SET unit_id = u.id FROM stage_units u WHERE u.stage_version_id = l.stage_version_id;
ALTER TABLE lessons ALTER COLUMN unit_id SET NOT NULL,
  ADD CONSTRAINT fk_lessons_unit FOREIGN KEY (unit_id, stage_version_id)
    REFERENCES stage_units (id, stage_version_id) ON DELETE CASCADE;
CREATE INDEX ix_lessons_unit_id ON lessons (unit_id);
-- Tên bài học duy nhất không phân biệt hoa thường trong phiên bản.
CREATE UNIQUE INDEX uq_stage_units_version_title ON stage_units (stage_version_id, lower(btrim(title)));
```

`lessons.position` vẫn là thứ tự phẳng duy nhất trong phiên bản. Phiên bản giữ bất biến "position tăng theo (unit.position, thứ tự trong unit)", nên thứ tự trong mỗi bài học tự duy nhất (spec: "Thứ tự học liệu duy nhất trong mỗi bài học"). Down: drop FK, cột, bảng.

### 0011_quiz_homework_content

- `lessons`: thay `ck_lessons_type` thành `IN ('video','markdown','quiz','homework')`. Thay `ck_lessons_type_content`: giữ hai nhánh cũ, thêm `(type IN ('quiz','homework') AND video_media_id IS NULL AND markdown_source IS NULL)`.
- `media_files`: thay `ck_media_files_kind` thành `IN ('video','image','document')`.
- `quiz_settings(lesson_id PK FK→lessons CASCADE, mode, max_attempts, time_limit_seconds, shuffle bool NOT NULL DEFAULT true, pass_percent)`. Ràng buộc:
  - `ck_quiz_settings_mode IN ('practice','assessment')`.
  - `ck_quiz_settings_rules`: practice thì ba cột giới hạn đều NULL; assessment thì `max_attempts` 1–10, `time_limit_seconds` 60–10800, `pass_percent` 1–100.
  - "Chế độ hiển thị đáp án" của Phụ lục A suy ra từ `mode` (OQ-4), không lưu cột riêng.
- `questions(id, lesson_id FK CASCADE, question_key, type, body_source, body_html NULL, explanation_source DEFAULT '', explanation_html NULL, points, position)`. Ràng buộc:
  - `ck_questions_type IN ('single','multi')`, `ck_questions_key` (regex như lesson_key), `ck_questions_points` 1–100.
  - `uq_questions_lesson_key`, và `uq_questions_lesson_position` DEFERRABLE.
- `question_options(id, question_id FK CASCADE, body, is_correct, position)`. Ràng buộc `ck_question_options_body` (1–500 ký tự) và `uq_question_options_question_position` DEFERRABLE.
- `homework_settings(lesson_id PK FK CASCADE, brief_source NOT NULL, brief_html NULL, pass_percent DEFAULT 70)` với `ck_homework_settings_pass_percent` 1–100.
- `rubric_criteria(id, lesson_id FK CASCADE, description, max_points, position)`. Ràng buộc `ck_rubric_criteria_max_points` 1–100 và `uq_rubric_criteria_lesson_position` DEFERRABLE.
- `homework_attachments(lesson_id FK CASCADE, media_id FK→media_files RESTRICT, position)`. Ràng buộc `pk_homework_attachments(lesson_id, media_id)` và `uq_homework_attachments_lesson_position`. Thứ tự hiển thị file kèm nằm ở đây; quyền xem đi qua `lesson_media` (phase 04 thêm attachment vào tập media của học liệu).

### 0012_homework_submissions

- `class_lesson_deadlines(class_id FK→classes RESTRICT, lesson_id FK→lessons RESTRICT, due_at, set_by FK→users, updated_at)` với `pk_class_lesson_deadlines(class_id, lesson_id)`.
- `submissions(id, class_member_id FK→class_members RESTRICT, lesson_id FK→lessons RESTRICT, attempt_no, repo_url NULL, commit_ref NULL, submitted_at, is_late, status)`. Ràng buộc:
  - `ck_submissions_status IN ('pending','passed','rework','superseded')`, `ck_submissions_attempt_no >= 1`.
  - `ck_submissions_commit_needs_repo (commit_ref IS NULL OR repo_url IS NOT NULL)`, `ck_submissions_repo_url (repo_url ~ '^https://')`.
  - `uq_submissions_member_lesson_no (class_member_id, lesson_id, attempt_no)`.
  - Partial unique `uq_submissions_one_pending ON (class_member_id, lesson_id) WHERE status = 'pending'`, và `ix_submissions_pending ON (submitted_at) WHERE status = 'pending'`.
- `submission_files(submission_id FK RESTRICT, media_id FK RESTRICT, position)` với `pk_submission_files(submission_id, media_id)`, `uq_submission_files_media (media_id)` (một file chỉ thuộc một bản nộp) và `uq_submission_files_position`.
- `submission_grades(id, submission_id FK RESTRICT, graded_by FK→users, total_points, max_points, result, comment, graded_at)`. Ràng buộc `ck_submission_grades_result IN ('passed','rework')`, `ck_submission_grades_comment` (khác rỗng) và `ck_submission_grades_points (0 <= total_points AND total_points <= max_points)`; chỉ mục `ix_submission_grades_submission (submission_id, graded_at DESC)`. Append-only: mỗi lần chấm hoặc sửa điểm là một dòng, dòng mới nhất là hiện hành, các dòng trước là lịch sử FR-63.
- `grade_criterion_scores(grade_id FK CASCADE, criterion_id FK→rubric_criteria RESTRICT, points, comment DEFAULT '')` với `pk_grade_criterion_scores(grade_id, criterion_id)` và `ck_grade_criterion_scores_points >= 0`.

### 0013_quiz_attempts

- `quiz_attempts(id, class_member_id FK RESTRICT, lesson_id FK RESTRICT, attempt_no, question_order uuid[] NOT NULL, option_order jsonb NOT NULL, started_at, deadline_at NULL, submitted_at NULL, auto_submitted bool DEFAULT false, score NULL, max_score, passed NULL, review_status NULL)`. Ràng buộc:
  - `uq_quiz_attempts_member_lesson_no`.
  - `ck_quiz_attempts_review_status IN ('pending','confirmed','superseded')`.
  - `ck_quiz_attempts_scored ((submitted_at IS NULL) = (score IS NULL))`.
  - `ck_quiz_attempts_review_after_submit (review_status IS NULL OR submitted_at IS NOT NULL)`.
  - Partial unique `uq_quiz_attempts_one_open ON (class_member_id, lesson_id) WHERE submitted_at IS NULL` và `uq_quiz_attempts_one_pending ON (class_member_id, lesson_id) WHERE review_status = 'pending'`.
  - `ix_quiz_attempts_open_deadline ON (deadline_at) WHERE submitted_at IS NULL` cho worker tự nộp, `ix_quiz_attempts_pending ON (submitted_at) WHERE review_status = 'pending'` cho hàng chờ.
- `quiz_attempt_answers(attempt_id FK CASCADE, question_id FK RESTRICT, selected_option_ids uuid[] NOT NULL, is_correct NULL, points_awarded NULL, saved_at)` với `pk_quiz_attempt_answers(attempt_id, question_id)`.
- `quiz_attempt_reviews(id, attempt_id FK RESTRICT, reviewer_id FK→users, comment, created_at)` với `ck_quiz_attempt_reviews_comment` (khác rỗng) và `ix_quiz_attempt_reviews_attempt (attempt_id, created_at DESC)`.
- `quiz_review_notes(review_id FK CASCADE, question_id FK RESTRICT, note)` với `uq_quiz_review_notes_review_question` (theo spec) và `ck_quiz_review_notes_note`.
- `quiz_attempt_grants(id, class_member_id FK RESTRICT, lesson_id FK RESTRICT, extra_attempts, reason, granted_by, granted_at)` với `ck_quiz_attempt_grants_extra` 1–3, `ck_quiz_attempt_grants_reason` (khác rỗng) và `ix_quiz_attempt_grants_member_lesson`.

### 0014_stage_reviews_jobs_emails

- `stage_reviews(id, class_member_id FK RESTRICT, stage_version_id FK RESTRICT, body, author_id FK→users, revision int NOT NULL DEFAULT 1, created_at, updated_at)` với `uq_stage_reviews_member_version` và `ck_stage_reviews_body` (khác rỗng). `revision` cho nhãn "đã sửa n lần"; nội dung cũ nằm trong `audit_logs.before`.
- `scheduled_job_runs(run_key text CONSTRAINT pk_scheduled_job_runs PRIMARY KEY, ran_at timestamptz NOT NULL)`. Khóa dạng `grading_digest:<teacherId>:<yyyy-mm-dd>` và `deadline_reminder:<memberId>:<lessonId>:<dueAt>`.
- `email_outbox`: thay `ck_email_outbox_template` thêm `homework_graded`, `quiz_reviewed`, `grading_digest`, `deadline_reminder`.

## Files

- Tạo: `apps/api/migrations/0010_stage_units.{up,down}.sql`, `0011_quiz_homework_content.{up,down}.sql`, `0012_homework_submissions.{up,down}.sql`, `0013_quiz_attempts.{up,down}.sql`, `0014_stage_reviews_jobs_emails.{up,down}.sql`.
- Sửa:
  - `apps/api/migrations/migrations_test.go`: `schemaVersion = 14`, test backfill, test cascade.
  - `apps/api/internal/platform/db/pgerr/constraints.go`, `map.go`, `map_test.go`.
  - `apps/api/internal/platform/testdb/fixtures/stage_db_published.sql` và `stage_db_draft.sql`: thêm `stage_units` và `unit_id`.
  - `apps/api/internal/features/stages/`: `service.go`, `repository_pg.go` và test (bản vá tối thiểu ở trên).
  - `docs/database.md`.

## Các bước

1. Sao lưu DB dev hoặc staging (`pg_dump -Fc`) trước khi chạy `migrate up` trên môi trường có dữ liệu thật; ghi lệnh vào `docs/database.md`.
2. Viết 0010 rồi chạy `make migrate-up` trên DB seed MVP. Kiểm mọi phiên bản có đúng một bài học "Nội dung chặng" và không còn lesson nào có `unit_id` NULL.
3. Viết 0011–0014 theo thiết kế trên. Mỗi file `.down.sql` đảo ngược đúng thứ tự (drop bảng con trước, khôi phục CHECK cũ). Down của 0011 thất bại nếu đã có học liệu quiz hoặc homework; ghi rõ điều này trong comment đầu file.
4. Thêm hằng cho mọi tên ràng buộc mới vào `constraints.go`. Trong `map.go`, thêm câu riêng cho:
   - `uq_stage_units_version_title` → `CONFLICT` "Tên bài học đã có trong phiên bản này."
   - `uq_stage_units_version_key` và `uq_questions_lesson_key` → `CONFLICT` "Mã đã tồn tại."
   - `uq_quiz_attempts_one_open` → `CONFLICT` "Bạn đang có một lượt làm bài chưa nộp."
   - `uq_submissions_one_pending`, `uq_quiz_attempts_one_pending` → `CONFLICT` "Xung đột dữ liệu, vui lòng thử lại".
   - `uq_submission_files_media` → `CONFLICT` "File đã dùng cho một lần nộp khác."
5. Không cần sửa `truncateAll` của seed hay `testdb.Reset`: cả hai truncate động theo `pg_tables` (`seed/seed.go:171`, `testdb/testdb.go:98-118`). Chỉ chạy `make seed-reset` để xác nhận.
6. Cập nhật fixture `stage_db_published.sql` và `stage_db_draft.sql`: thêm bài học (id hằng `testdb.UnitDBV1ID`, `UnitDBV2ID`) và gán `unit_id`.
7. Thêm vào `migrations_test.go`:
   - Up/down/up sạch tới version 14.
   - Backfill: dựng DB ở version 9 có 2 phiên bản và 3 học liệu, migrate lên 10, khẳng định mỗi phiên bản một bài học, học liệu giữ position, % tiến độ của một thành viên trước và sau bằng nhau (cùng SQL `percentOfRequired`).
   - FK ghép từ chối học liệu trỏ bài học của phiên bản khác.
   - Hai partial unique chặn hai lượt quiz mở và hai lượt `pending`.
   - Xóa bản nháp phiên bản chặng cascade tới `stage_units`, `questions`, `rubric_criteria`.
8. Cập nhật `docs/database.md`: bảng migration 0010–0014, lý do `position` phẳng, giả định OQ-15, và việc append-only của `submission_grades` và `quiz_attempt_reviews`.

### Bản vá tối thiểu cho `stages` (giữ MVP xanh tới phase 02)

`unit_id` là NOT NULL ngay từ 0010, nên code `stages` hiện có phải ghi được `unit_id` trong phase này:

- `CreateStage` (`stages/service.go:111`) tạo bản nháp v1 rỗng: chèn thêm một bài học mặc định `noi-dung-chang` / "Nội dung chặng" cho phiên bản đó.
- `Clone` (`stages/service.go:333`, `repository_pg.go:311-348`) sao chép mọi `stage_units` của phiên bản nguồn sang id mới và ánh xạ `unit_id` của học liệu.
- `SaveDraft` gán học liệu mới chưa có `unit_id` vào bài học có `position = 1` của phiên bản. Mọi phiên bản luôn có bài học đó nhờ backfill và hai điểm trên.
- Test: tạo chặng mới rồi thêm học liệu; clone phiên bản có học liệu; cả hai thành công và mỗi học liệu có `unit_id` thuộc đúng phiên bản.

Phase 02 thay bản vá này bằng mô hình bài học đầy đủ.

## Kiểm chứng

```bash
cd apps/api
make migrate-up && make migrate-version          # 14
make lint
TEST_DATABASE_URL=... make test-integration       # migrations, pgerr, seed, các feature MVP vẫn xanh
grep -rn '"uq_\|"ck_\|"fk_' internal --include='*.go' | grep -v pgerr/constraints.go   # rỗng
```

## Tiêu chí hoàn thành

- [x] Năm migration up/down sạch, `schemaVersion = 14`, test MVP hiện có vẫn xanh.
- [x] Mọi tên ràng buộc có hằng `pgerr`; lệnh grep cấm chuỗi trả rỗng.
- [x] Test backfill chứng minh % tiến độ không đổi.

## Rủi ro

- Backfill trên DB lớn khóa bảng `lessons` lâu. Dữ liệu MVP nhỏ (vài chục phiên bản) nên chấp nhận được; ghi chú chạy ngoài giờ cao điểm.
- `gen_random_uuid()` sinh uuid v4 cho bài học backfill, khác v7 của ứng dụng. Không ảnh hưởng tính đúng; ghi chú trong migration.
- Down 0011 không an toàn khi đã có dữ liệu quiz hoặc homework. Rollback thật đi qua bản sao lưu ở bước 1.
- 0010 không theo kiểu expand rồi contract: code cũ không ghi `unit_id` nên sẽ lỗi khi chèn học liệu sau khi migrate. Bản prod chạy một API instance với `MIGRATE_ON_START=true` (`infra/docker-compose.prod.yml:67`), nên migration và code mới lên cùng một lần deploy. Ghi rõ trong `docs/database.md`: sao lưu, rồi deploy image mới; không chạy migrate trước với image cũ.
