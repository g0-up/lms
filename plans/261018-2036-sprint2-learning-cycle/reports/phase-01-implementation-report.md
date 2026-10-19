# Báo cáo triển khai: Migrations và ràng buộc Sprint 2

- Phase: [phase-01-database-migrations.md](../phase-01-database-migrations.md)
- Trạng thái: hoàn thành, có lưu ý (sửa thêm file ngoài danh sách của phase để suite MVP xanh)
- Ngày: 2026-10-18. Chưa commit.

## Kết quả

Năm migration `0010`–`0014` tạo đủ bảng của Phụ lục A. Migrate up, down rồi up lại đều sạch trên `lms_test`, và `schemaVersion = 14`. Backfill đưa mọi học liệu đã có vào bài học `noi-dung-chang` ("Nội dung chặng"), giữ nguyên `position`. Test chứng minh % tiến độ của học viên trước và sau migrate bằng nhau. Mọi ràng buộc mới đều có hằng trong `pgerr`, đã khớp hai chiều với `pg_constraint` nhờ `constraints_test.go`. Toàn bộ suite API xanh.

## File đã sửa

Migration (mới):

- `apps/api/migrations/0010_stage_units.{up,down}.sql`: tạo `stage_units`, backfill, `lessons.unit_id` NOT NULL, `fk_lessons_unit` ghép theo `(unit_id, stage_version_id)` có CASCADE, và `ix_lessons_unit_id`.
- `apps/api/migrations/0011_quiz_homework_content.{up,down}.sql`: thêm `lessons.type` quiz/homework và `media_files.kind` document, cùng sáu bảng nội dung.
- `apps/api/migrations/0012_homework_submissions.{up,down}.sql`: hạn nộp, bài nộp, file nộp, kết quả chấm append-only và điểm tiêu chí.
- `apps/api/migrations/0013_quiz_attempts.{up,down}.sql`: lượt quiz, câu trả lời, xác nhận append-only, nhận xét từng câu và lượt cấp thêm.
- `apps/api/migrations/0014_stage_reviews_jobs_emails.{up,down}.sql`: `stage_reviews`, `scheduled_job_runs` và bốn mẫu email mới.

pgerr:

- `apps/api/internal/platform/db/pgerr/constraints.go`: hằng cho mọi ràng buộc mới.
- `apps/api/internal/platform/db/pgerr/map.go`: thông điệp `CONFLICT` cho bảy unique theo phase.
- `apps/api/internal/platform/db/pgerr/map_test.go`: 7 case mới.
- `apps/api/internal/platform/db/pgerr/map_integration_test.go`: 1 case mới, tên bài học chỉ khác hoa thường và khoảng trắng.

Bản vá `stages` để giữ MVP xanh:

- `apps/api/internal/features/stages/repository_pg.go`:
  - `createUnits` tạo bài học mặc định cho chặng mới. Khi nhân bản, nó chép bài học của phiên bản nguồn và ánh xạ `unit_id` theo `lesson_key`.
  - `insertUnit` ghi có điều kiện bản nháp.
  - `insertLesson` nhận `unitID`. Khi giá trị là nil (đường `SaveDraft`), học liệu rơi về bài học `position = 1` của phiên bản.
- `apps/api/internal/features/stages/repository_pg_test.go`: thêm `TestCreateStageAddsDefaultUnitForNewLessons` và `TestCloneCopiesUnitsAndKeepsLessonsInTheirUnit`.

Test và fixture:

- `apps/api/migrations/migrations_test.go`:
  - `schemaVersion = 14`.
  - Allow-list CASCADE có 13 FK, kèm kiểm tra cột của FK ghép.
  - `TestStageUnitsBackfill`.
  - Bổ sung cascade xóa bản nháp cho bài học và nội dung quiz/homework.
  - Năm test ràng buộc: `TestStageUnitConstraints`, `TestQuizHomeworkContentConstraints`, `TestSubmissionConstraints`, `TestQuizAttemptConstraints`, `TestStageReviewsJobsAndEmails`.
- `apps/api/internal/platform/testdb/fixtures.go`: thêm `UnitDBV1ID`, `UnitDBV2ID`.
- `apps/api/internal/platform/testdb/fixtures/stage_db_published.sql` và `stage_db_draft.sql`: thêm bài học và gán `unit_id`.

Tài liệu:

- `docs/database.md`:
  - sơ đồ quan hệ và bảng migration `0010`–`0014`;
  - danh sách CASCADE mới và lý do `position` phẳng;
  - giả định OQ-15;
  - append-only của `submission_grades` và `quiz_attempt_reviews`;
  - partial unique "một bản chờ" và các mẫu email mới;
  - khử trùng lặp của `scheduled_job_runs`;
  - down `0011`/`0014` lỗi khi đã có dữ liệu mới;
  - quy trình deploy: `pg_dump -Fc`, rồi deploy image mới với `MIGRATE_ON_START`, không migrate trước bằng image cũ;
  - seed có `stage_units`.

## Kiểm chứng

- `make -C apps/api test` (gồm integration trên `lms_test`): xanh, exit 0.
- `go test -count=3 -tags integration ./migrations/...`: xanh cả ba lần. `TestStageUnitsBackfill` chạy riêng cũng xanh.
- `golangci-lint` v2.14.0: 0 issue.
- `grep -rn '"uq_\|"ck_\|"fk_' internal --include='*.go' | grep -v pgerr/constraints.go`: rỗng.
- Grep cấm `CREATE FUNCTION/TRIGGER/TYPE/EXTENSION` trên `migrations/`: rỗng. `TestNoUserDefinedDatabaseObjects` vẫn xanh.
- DB dev `lms`:
  - Đã sao lưu trước khi migrate: `/tmp/claude-1000/-home-cesc-Documents-personal-workspace-lms/993304ac-9dde-4814-a7d6-ce55ddf59433/scratchpad/lms-dev-before-0010.dump`. Thư mục này là scratchpad của phiên, sẽ không còn lâu.
  - `migrate up` từ 9 lên 14.
  - 5/5 phiên bản có đúng một bài học mặc định, 0 học liệu có `unit_id` NULL, 16/16 học liệu thuộc bài học của chính phiên bản mình.
  - `make seed-reset` thành công (`stage_units: 5`, `lessons: 16`).

## Sai lệch so với phase

1. **Sửa file ngoài danh sách của phase.** `unit_id` NOT NULL buộc mọi đường chèn `lessons` phải đổi, nên đã sửa thêm:
   - `internal/seed/seed.go`: chèn bài học mặc định cho mỗi phiên bản và truyền `unit_id`.
   - `internal/seed/seed_integration_test.go`: thêm số đếm `stage_units`.
   - `internal/features/learning/service_integration_test.go`: dữ liệu inline chèn `lessons` phải có bài học.

   Cả ba chỉ là thay đổi tối thiểu do schema, không thêm hành vi.
2. **`stages/service.go` không đổi.** Phase ghi vị trí vá ở `service.go:111` và `:333`, nhưng tạo bài học và ánh xạ `unit_id` hoàn toàn là việc của SQL. Vì vậy bản vá nằm trọn trong `repository_pg.go` (`Create` và `SaveDraft`), và `service.go` không cần sửa.
3. **Thông điệp `CONFLICT` không có dấu chấm cuối**, để khớp các thông điệp hiện có trong `map.go`. Phase viết có dấu chấm.
4. **`CkHomeworkSettingsPassPercent` mang `//nolint:gosec`.** gosec G101 nhầm chữ "Pass" là mật khẩu; chú thích theo đúng mẫu nolint đã có trong repo.
5. **Không thêm CHECK `position` hay index nào ngoài DDL của phase.**

## Việc tiếp theo

- Phase 02 có thể thay bản vá tối thiểu trong `stages/repository_pg.go` (`createUnits`, fallback `position = 1`) bằng mô hình bài học đầy đủ.
- `plan.md` vẫn ghi phase 01 là `pending`. Bảng chỉ mục do orchestrator cập nhật.
