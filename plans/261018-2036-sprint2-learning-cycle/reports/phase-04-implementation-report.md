# Báo cáo thực hiện: soạn quiz và homework (backend)

- Phase: `phase-04-quiz-homework-authoring-backend.md` (FR-60, FR-70, FR-74, FR-90)
- Ngày: 2026-10-18
- Trạng thái: xong, có vài điểm cần quyết (cuối báo cáo)

## Kết quả

Backend soạn được quiz (practice/assessment) và homework trong aggregate `StageVersion`, lưu ở sáu bảng con của migration 0011. Có route admin cho câu hỏi và tiêu chí, publish render và chặn theo prototype, clone sao chép sâu, media kind `document` có URL tải luôn kèm `attachment`, và seed nội dung theo prototype. Trang học của học viên chỉ thấy loại học liệu và cấu hình, không bao giờ thấy câu hỏi hay đáp án.

## File đã sửa

Trong danh sách của phase:

- `internal/features/stages/`:
  - thêm `quiz_content.go` (426 dòng), `homework_content.go` (218 dòng) và `repository_pg_content.go` (461 dòng). File repository thứ ba tách khỏi `repository_pg.go` vì phần load/diff/insert bảng con đủ lớn để thành một ranh giới riêng;
  - sửa `entity.go`, `entity_test.go`, `service.go`, `service_test.go`, `repository_pg.go`, `repository_pg_test.go`, `dto.go`, `handler.go`;
  - golden mới `testdata/lesson_quiz.json`, `testdata/lesson_homework.json`. `markdown.go` không cần sửa: phần render câu hỏi và đề bài nằm ở `service.go` và dùng lại renderer có sẵn.
- `internal/features/media/`: `entity.go` (`KindDocument`), `service.go` (`DocumentContentType`, giới hạn dung lượng, ticket `headers`), `dto.go`, `service_test.go`, `repository_pg_test.go`, `testdata/upload_ticket.json`.
- `internal/platform/storage/`: `storage.go` (`GetOptions`), `minio.go`, `mem.go`, `minio_test.go`.
- `internal/platform/config/`: `config.go`, `config_test.go` (`MAX_DOCUMENT_BYTES`, mặc định 20 MiB).
- `internal/features/learning/`: `repository_pg.go` (`LessonRow` đọc loại mới), `service.go` (`contentOf` trả loại mới mà không có đáp án).
- `internal/seed/data.go`, `internal/seed/seed.go`.
- `docs/architecture.md`: bảng env, mục stages (quiz/homework) và mục media (`document`).
- `infra/.env.example`: thêm `MAX_DOCUMENT_BYTES` (phase cho phép "nếu có").

Ngoài danh sách. Đều cần cho build hoặc test xanh, không đổi hành vi ngoài phạm vi phase:

- `internal/app/deps.go`: truyền `MaxDocumentBytes` vào media service.
- `internal/features/learning/repository.go`: chỉ sửa comment.
- `internal/features/learning/service_integration_test.go`: thêm `TestQuizAndHomeworkPagesHideAnswers`, assert trên JSON thô của trang học.
- `internal/seed/markdown.go`: render đề bài và câu hỏi lúc seed.
- `internal/seed/seed_test.go`, `seed_integration_test.go`:
  - cập nhật số đếm của dataset;
  - thêm test dữ liệu quiz/homework xấu và kiểm tra nội dung đã seed sẵn sàng publish;
  - thêm helper `openEmpty` dọn DB sau test (lý do ở mục sự cố).
- `internal/seed/upload_sample.go`, `upload_sample_test.go`: `--upload-sample` tải thêm một zip mẫu tất định cho file kèm `ds-hw`; nếu không, URL tải của file đó trỏ vào object không tồn tại.
- `internal/features/reports/repository_pg_test.go` và `testdata/{class-report,dashboard,member-report}.json`:
  - số liệu dẫn xuất từ dataset đổi vì có thêm 12 học liệu quiz/homework bắt buộc;
  - đã tính tay rồi cập nhật, ví dụ An 10/25 = 40%, dashboard basic01 41 → 23 (160/7);
  - golden đã regenerate và đọc diff.
  - `member-report.json` đã có thay đổi chưa commit từ phase trước.
- `infra/docker-compose.prod.yml`: thêm `MAX_DOCUMENT_BYTES`. `docker compose config` hợp lệ.

## Bất biến và test chứng minh

| Bất biến | Test |
|---|---|
| DTO học viên không có đáp án (assert JSON thô) | `learning/service_integration_test.go` `TestQuizAndHomeworkPagesHideAnswers` (DB thật, đáp án seed riêng không xuất hiện trong JSON) |
| Ghi câu hỏi/phương án/tiêu chí/cấu hình/file kèm vào bản published → `VERSION_IMMUTABLE`, không đổi dòng nào | Repository ghi thẳng: `TestImmutability_QuizHomeworkContent` (15 ca, so snapshot trước và trong transaction). Service: `TestImmutability_QuizHomeworkServiceMutations`; unit trong `entity_test.go` |
| Validate câu hỏi → `VALIDATION_FAILED` | `entity_test.go`, `service_test.go` (2–6 phương án, 1–500 ký tự, ít nhất một đúng, `single` đúng một, key) |
| Clone sao chép sâu với id mới | `TestCloneDeepCopiesQuizAndHomeworkOnRealSchema` và unit trong `entity_test.go` |
| Giảng viên nhận 403 ở `POST /questions`, `PATCH /lessons/:lid`, `PUT /questions/order` | `TestQuizRoutesRequireAdmin` |
| Luật type/đuôi của upload document | `media/service_test.go`: `.sql` gửi `application/octet-stream` được lưu `text/plain; charset=utf-8`; `.html`/`.svg` bị 422; `headers` của ticket khớp type chuẩn; vượt dung lượng bị từ chối |
| URL tải document luôn có `attachment` | `minio_test.go` (query `response-content-disposition`), `media/service_test.go` |
| Publish render và chặn | `TestPublishQuizHomeworkOnRealSchema`, `TestPublishBlocksIncompleteQuizOnRealSchema`; guard `TransitionStatus` từ chối câu hỏi hoặc đề bài chưa render |

## Lệnh đã chạy

| Lệnh | Kết quả |
|---|---|
| `make -C apps/api test` (race, tag integration, `-p 1`) | 23 package `ok`, không có FAIL |
| `make -C apps/api lint GOLANGCI_LINT="go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0"` | `go vet` sạch, `0 issues.` |
| `gofmt -l apps/api` | rỗng |
| `grep -rn '"uq_\|"ck_\|"fk_' internal --include='*.go' \| grep -v pgerr/constraints.go` | rỗng |
| `make seed-reset` | 28 học liệu, 10 `quiz_settings`, 46 câu hỏi, 184 phương án, 2 homework, 6 tiêu chí, 1 file kèm, 10 `media_files`, 10 `lesson_media` |

`make lint` với binary mặc định (`~/go/bin/golangci-lint`) không chạy được vì nó build bằng go1.26, còn module nhắm go1.27. Đây là lỗi môi trường, không phải lỗi code; chạy bản v2.14.0 qua `go run` thì sạch.

## Lệch so với plan (theo code)

1. Lưu câu hỏi bắt buộc có ít nhất một đáp án đúng, nên plan cho lưu câu chưa có đáp án nhưng code thì không. Lý do chặn publish "còn câu chưa có đáp án đúng" vẫn được giữ cho dữ liệu cũ hoặc seed.
2. `questionKey` là tùy chọn. Bỏ trống thì sinh từ `cau-hoi` qua `uniqueKey`; tạo rồi thì không đổi được (khóa ổn định cho attempt sau này).
3. `PATCH …/questions/{qid}` dùng ngữ nghĩa thay toàn bộ: client gửi lại đủ câu, phương án có `id` thì giữ id.
4. Thời gian làm bài là `time.Duration` trong domain, giây trong DB (`time_limit_seconds`) và phút trong API (`timeLimitMinutes`).
5. File kèm là struct `Attachment` (media id, tên, dung lượng hiển thị) chứ không phải mảng uuid trần.
6. Thêm export `media.DocumentContentType`, để seed và stages kiểm đuôi file theo cùng một luật.
7. `go-hw` không có file kèm: file của prototype là `.csv`, không nằm trong danh sách đuôi cho phép. `ds-hw` có một zip.
8. Hai học liệu `go-hello-read` và `go-hello-practice` của `prototype/seed.js` không được seed, vì plan chỉ liệt kê quiz/homework. Unit "Kiểm tra cuối chặng" của Go gồm `go-hw` và `go-quiz`.
9. DTO học viên ở `learning` chỉ trả loại học liệu và cấu hình quiz/homework. Nội dung làm bài thuộc các phase 06 và 09.

## Sự cố trong lúc làm

- Test seed để lại học liệu quiz/homework và media `document` trong `lms_test`. Package `migrations` chạy ngay sau đó, và `TestMigrationRoundTrip` down 0011 vỡ ở `ck_media_files_kind`, để DB ở trạng thái dirty version 10.
  - Cách sửa: helper `openEmpty` reset DB cả trước và sau mỗi test seed. File test của migrations không bị sửa.
  - Tôi đã kiểm tra bảng của 0011 có mặt và 0012–0014 vắng mặt, rồi phục hồi thủ công bằng `UPDATE schema_migrations SET version = 11, dirty = false` trên `lms_test` (container `lms-postgres-1`; không chạm container khác và không dừng container nào).
  - Sau đó toàn bộ test xanh.

## Câu hỏi mở

1. Có cho đuôi `.csv` (`text/csv`) làm file kèm homework không? Nếu có thì `go-hw` sẽ có file kèm như prototype.
2. Có seed `go-hello-read` và `go-hello-practice` cho khớp đủ prototype không?
3. Phần trăm tiến độ trong báo cáo giảm, vì có thêm học liệu quiz/homework bắt buộc mà chưa có attempt hay bài nộp nào. Công thức SQL được giữ nguyên theo plan. Số sẽ hồi lại khi phase 06/09 seed attempt và submission; cần xác nhận đây là hành vi mong muốn trong khoảng giữa.
4. Đến phase 06, học viên vẫn tự đánh dấu hoàn thành được học liệu quiz/homework. Phase 06 sẽ cho `SetCompletion` từ chối.
