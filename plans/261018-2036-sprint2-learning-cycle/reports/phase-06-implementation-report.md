# Báo cáo thực hiện: homework (backend)

- Phase: `phase-06-homework-backend.md` (FR-61, FR-62, FR-63, FR-92 cho ba hành động homework)
- Ngày: 2026-10-19
- Trạng thái: xong, còn một mối lo ngoài lane (e2e web cũ, cuối báo cáo)

## Kết quả

Giảng viên phụ trách hoặc Admin đặt và xóa hạn nộp theo lớp. Học viên đang học lớp `active` tải file qua `/me/uploads` rồi nộp repo và/hoặc file; nộp lại thì bản `pending` cũ thành `superseded`. Giảng viên chấm bản mới nhất theo tiêu chí, sửa điểm được và lịch sử giữ mọi lần chấm. Cùng transaction với lần chấm: đổi `submissions.status`, ghi hoặc xóa `completed_at`, ghi audit và đưa email `homework_graded` vào outbox. Seed có hạn nộp, bản nộp và điểm theo prototype, nên % hoàn thành trong báo cáo lớp đã tính cả homework.

## File đã sửa

Trong danh sách của phase:

- `internal/features/homework/` (mới): `entity.go`, `repository.go`, `repository_pg.go`, `service.go`, `dto.go`, `handler.go`, `entity_test.go`, `service_integration_test.go` (17 test), golden `testdata/{my-submissions,submission-detail,class-homework}.json`.
- `internal/features/mailer/`: `entity.go` (`TemplateHomeworkGraded`), `templates.go` (tiêu đề "Bài đã được chấm: …" và "Điểm đã được cập nhật: …"), `templates_test.go`, `templates/homework_graded.{html,txt}.tmpl` (mới).
- `internal/features/media/`: `handler.go` (`RegisterStudent`), `service.go` (quyền học viên, chỉ `document`, giới hạn 10 file chưa gắn), `repository.go`, `repository_pg.go` (`CanUserAccess` thêm người tải lên và giảng viên lớp có bản nộp chứa file; đếm file chưa gắn), `dto.go`, `entity.go`, `service_test.go`, `repository_pg_test.go`, `testdata/upload_ticket.json`.
- `internal/features/classes/`: `repository.go`, `repository_pg.go`, `service.go` (đổi course version thì xóa hạn nộp trong cùng transaction), `service_integration_test.go`.
- `internal/features/learning/`: `repository.go`, `repository_pg.go` (`CompletionWriter`: `MarkCompleted`, `ClearCompleted`), `service.go` (`SetCompletion` từ chối quiz/homework bằng 422; trang học homework trả đề bài, tiêu chí, ngưỡng, file kèm), `entity.go`, `dto.go`, `handler.go`, test và golden.
- Audit, đủ bốn chỗ: `internal/platform/audit/actions.go`, `actions_test.go`, `audit.go` (`knownActions`); `internal/features/reports/model.go` (nhãn "Đặt hạn nộp", "Chấm bài", "Sửa điểm", khóa `activityPayload`, nhánh `summarize`) và `model_test.go`.
- `internal/app/router.go`, `deps.go`, `router_test.go`: `newHomework` với `PublicBaseURL`, route staff/student, `/me/uploads` dùng `requireStudent`.
- `internal/seed/data.go`, `seed.go`, `seed_test.go`, `seed_integration_test.go`, `upload_sample.go`, `upload_sample_test.go`.
- `docs/architecture.md`: mục media `/me/uploads`, `--upload-sample`, và mục mới "Homework: hạn nộp, nộp bài, chấm".

Ngoài danh sách, sửa tối thiểu để suite xanh:

- `internal/features/reports/repository_pg_test.go` và golden `testdata/{class-report,member-report,dashboard}.json`. Seed homework làm số liệu dẫn xuất đổi: An 11/25 = 44%, Cường 14/25 = 56%, % theo chặng basic01 {50,67,60,40,0}, `ds-hw` của An thành `completed`, hoạt động gần đây có "Đặt hạn nộp". Assert người thực hiện hoạt động mới nhất trên dashboard giờ chấp nhận một trong ba giảng viên, vì audit seed giờ có nhiều mốc gần nhau. Golden đã regenerate bằng `-update` và đọc diff. Diff git của các golden này gồm cả thay đổi chưa commit từ phase 04 (mẫu số 14 → 25).

## Bất biến và test chứng minh

| Yêu cầu | Test |
|---|---|
| Nộp: happy path repo + file, validate (repo host, commit, ≤5 file, file người khác, file đã gắn, không `ready`) | `TestSubmitRepoAndFiles`, `TestSubmitValidation`, `entity_test.go` `TestSubmitInputNormalize` |
| Nộp lại thay bản `pending`; đã Đạt thì chặn; chỉ chấm bản mới nhất | `TestPassedLocksResubmitAndOnlyLatestIsGraded`, `TestNextStatusOnResubmit` |
| Chấm theo rubric ở biên ngưỡng (so số nguyên, 69.99 / 70) | `TestResultOfAtThreshold`, `TestNewGrade`, `TestGradeThresholdCompletionAndHistory` |
| `completed_at` ghi khi Đạt, xóa khi Cần làm lại, cùng transaction; lỗi giữa chừng không để lại gì | `TestGradeThresholdCompletionAndHistory`, `TestGradeIsAtomic`, `learning` `TestCompletionWriter` |
| Sửa điểm giữ lịch sử, mỗi dòng có điểm từng tiêu chí | `TestGradeThresholdCompletionAndHistory`, golden `submission-detail.json` |
| Validate điểm (thiếu tiêu chí, ngoài 0..max, tiêu chí lạ, nhận xét rỗng/quá dài) → 422 kèm `details` | `TestGradeValidation` |
| Email `homework_graded` vào outbox với `ResultURL`; audit `homework.graded` / `homework.grade_updated` có `before` | `TestGradeEnqueuesEmailAndRecordsAudit`, `mailer` `templates_test.go` |
| Hạn nộp đặt/xóa, audit `class.homework_deadline_changed`, lớp `ended` → 409, `is_late` đúng tại `dueAt` là `false` | `TestDeadlineSetClearAndLateFlag` |
| Đổi course version xóa hạn nộp | `classes/service_integration_test.go` |
| Học viên không phải thành viên / `dropped` / lớp `draft` / `ended` bị chặn | `TestStudentGates`, `TestMemberGate` |
| Id ngoại lớp trên mọi route theo lớp → `NOT_FOUND` (mỗi route một ca) | `TestClassScopedRoutesRejectForeignIDs` |
| Giảng viên lớp khác → 403 | `TestOtherTeacherIsForbidden`, `TestStaffCanAccessClass` |
| Hai lần nộp đồng thời → `attempt_no` 1 và 2, đúng một `pending` | `TestConcurrentSubmitsAreSerialized` |
| Hai lần chấm đồng thời tuần tự hóa; chấm và nộp lại song song không bao giờ biến `passed` thành `superseded` | `TestConcurrentGradesAreSerialized`, `TestConcurrentGradeAndResubmit` |
| Upload học viên: chỉ `document`, 20 MB, allow-list đuôi, thành viên `active` của lớp `active`, file chưa gắn thứ 11 → 409, giới hạn tuần tự hóa | `media` `TestStudentUploadRules`, `TestStudentCompletesOnlyOwnUpload`, `TestStudentUploadAndAccess`, `TestStudentUploadRequiresActiveEnrollment`, `TestStudentUploadKindAndLimits`, `TestStudentUnattachedLimitIsSerialized`, `TestCanUserAccess` |
| URL tải file bản nộp có `attachment`; học viên khác lớp không lấy được | `TestSubmissionFilesDownloadAsAttachment` |
| `PUT completion` trên homework → 422 | `learning/service_integration_test.go` (ca quiz/homework) |
| Route homework và `/me/uploads` cần phiên | `router_test.go` `TestHomeworkRoutesRequireSession` |
| Seed tuân luật service (trạng thái, muộn, regrade, completion, file, audit) và từ chối dữ liệu xấu | `seed_integration_test.go` "homework work follows the service rules", `seed_test.go` `TestCheckDataRejectsBadHomeworkWork` |

`submissions` bất biến trừ `status`: `grep -rn "UPDATE submissions" internal/` chỉ ra hai lệnh, đều ở `homework/repository_pg.go`: `SET status = 'superseded' WHERE id=$1 AND status='pending'` và `SET status = $2 WHERE id=$1 AND status <> 'superseded'`. Cả hai dùng `ExecAffectOne`.

## Lệnh đã chạy

| Lệnh | Kết quả |
|---|---|
| `make -C apps/api test` | tất cả package `ok` (homework 19.6s, seed 8.9s, reports 11.4s) |
| `make -C apps/api lint GOLANGCI_LINT="go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0"` | 0 issues |
| `go test -race -count=3 -p 1 -tags integration ./internal/features/homework/...` | `ok` (55.8s), không flaky |
| `grep -rn "UPDATE submissions" apps/api/internal/` | chỉ hai lệnh đổi `status` (trên) |

## Sai khác so với plan

Theo code và `plan.md` §4 khi phase file lệch:

- Khóa payload email là `ResultURL` (`PublicBaseURL + "/learn/classes/{id}/lessons/{lid}"`) theo `plan.md` §4 mục 5; phase file ghi `LessonURL`.
- Repo URL dùng allow-list host đúng (github.com, gitlab.com, bitbucket.org, cho phép `www.`, không phân biệt hoa thường) thay vì regex prefix, nên `github.com.evil.io` bị từ chối.
- Không phải thành viên hoặc `dropped` trên route học viên → 403 `FORBIDDEN`, như phase file.
- Chấm không kiểm trạng thái thành viên hay lớp: giảng viên vẫn chấm được bài đã nộp sau khi học viên `dropped` hoặc lớp `ended`, vì bài đã nộp hợp lệ. Cần xác nhận nếu sản phẩm muốn khác.
- `DELETE` hạn nộp → 204; xóa khi chưa có hạn là no-op (không audit).
- `dueAt` trong quá khứ được chấp nhận (prototype có hạn đã qua 20 ngày).
- Test đồng thời dùng `sync.WaitGroup` và barrier thay cho `errgroup`; kết quả kiểm tất định trên DB sau khi cả hai xong.
- `POST /submissions/:sid/grades` → 201, `nextQueueItem` luôn `null` (phase 07 điền).
- `/media` và `/me/uploads` dùng chung một rate limiter.
- `MarkCompleted` giữ `completed_at` đầu tiên (`COALESCE`), như phase file.
- Id dạng uuid sai cú pháp → 404 theo `httpx.UUIDParam` có sẵn.
- Sửa lỗi trùng tên cột trong truy vấn `SubmissionContext` (`s.id AS submission_id`, `s.status AS submission_status`).
- Seed thêm một lần sửa điểm cho bài nộp thứ hai của Cường (prototype không có) để có dữ liệu `homework.grade_updated`.
- Seed không đưa email `homework_graded` vào outbox; seed chỉ ghi dữ liệu nghiệp vụ và audit.
- `sampleDocument` của `--upload-sample` chỉ có mẫu zip, png và text; seed không có pdf/jpg, loại khác trả lỗi rõ ràng.

## Câu hỏi mở và mối lo

- `apps/web/e2e/reports.spec.ts` (ngoài lane, chưa sửa) đã lệch dữ liệu seed:
  - Test mong "71% hoàn thành" cho An; số mới là 44%.
  - `below=30` mong [DUNG, HA, KHANG, PHONG]; giờ Bích (28%) cũng dưới ngưỡng.
  - `below=50` giờ thêm An và Bích.
  - Comment "Hà, Phong cùng 29%" giờ là 16%.
  - Phần lớn do phase 04 đổi mẫu số (14 → 25), phần còn lại do completion homework của seed. Cần lane web cập nhật.
- Có cho chấm khi học viên đã `dropped` hoặc lớp đã `ended` không (hiện cho phép)?
- File tải lên không nộp vẫn là rác trong storage. Giới hạn 10 file chỉ chặn lạm dụng; việc dọn file mồ côi vẫn là nợ kỹ thuật, như phần rủi ro của phase.
