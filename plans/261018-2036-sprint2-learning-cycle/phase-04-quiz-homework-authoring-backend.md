---
phase: 4
title: "Soạn quiz và homework: backend"
status: completed
priority: P0
effort: "3d"
dependencies: [2]
---

# Phase 04: Soạn quiz và homework (backend)

## Mục tiêu

Trong bản nháp, Admin tạo học liệu loại quiz học liệu, quiz bài tập hoặc homework; soạn câu hỏi, phương án, tiêu chí chấm và file kèm; rồi phát hành khi đủ điều kiện. Sau phát hành mọi nội dung đó bất biến. Nhân bản sao chép sâu. Giảng viên không gọi được API soạn.

## Context

- Spec: FR-60, FR-70, FR-74, FR-90; mục 4 "Versioning"; bảng cấu hình FR-70.
- Prototype: `LESSON_KINDS` 1727-1729, `saveLesson` 196-231, `saveQuestion` 704-719, `saveCriterion` 722-729, `lessonBlockers` 684-695 ([scout-prototype](reports/scout-prototype.md) §2).
- Code: `stages/entity.go` (`LessonContent`, `contentFactory`, `copyContent`, `Publish`), `stages/repository_pg.go` (`valuesOf`, `toLesson`, `replaceLessonMedia`), `media/entity.go` (`allowedTypes`, `Limits`), `platform/storage/storage.go:19` (`PresignGet`).

## Thiết kế domain

- `LessonQuiz = "quiz"`, `LessonHomework = "homework"`; `ParseLessonType` nhận cả hai.
- `QuizContent{Mode, MaxAttempts *int, TimeLimit *time.Duration, Shuffle bool, PassPercent *int, Questions []*Question}`.
  - `Validate()` áp quy tắc theo chế độ. Practice: ba trường giới hạn nil. Assessment: lượt 1–10, thời gian 1–180 phút, ngưỡng 1–100.
  - Mặc định khi tạo assessment: 2 lượt, 20 phút, 70%. `Shuffle` mặc định bật.
- `Question{id, key, type single|multi, bodySource, bodyHTML, explanationSource, explanationHTML, points 1–100, options []Option}` và `Option{id, body, correct}`.
  - Validate khi lưu: nội dung khác rỗng, 2–6 phương án, nội dung phương án 1–500 ký tự, ít nhất 1 đáp án đúng, câu `single` có đúng 1 đáp án đúng.
  - Một câu chưa có đáp án đúng vẫn lưu được ở nháp (prototype hiện badge "Chưa có đáp án đúng"), nhưng chặn phát hành.
- `HomeworkContent{BriefSource, BriefHTML, PassPercent (mặc định 70), Criteria []*Criterion, Attachments []uuid.UUID}` và `Criterion{id, description, maxPoints 1–100}`. `MaxPoints()` là tổng điểm tối đa.
- Đổi loại khi sửa học liệu (FR-70): đổi giữa practice và assessment giữ `Questions`, chỉ đổi cấu hình. Đổi sang loại khác bỏ nội dung cũ; repository xóa dòng con khi diff.
- `Publish` thêm các lý do chặn, dùng nguyên văn của prototype, mọi lý do đều là `domain.ErrInvalid`:
  - `"X" chưa có câu hỏi.`
  - `"X" có N câu chưa đánh dấu đáp án đúng.`
  - `"X" chưa có đề bài.`
  - `"X" chưa có tiêu chí chấm.`
- `RenderMarkdown` render thêm `bodyHTML`, `explanationHTML` của câu hỏi và `BriefHTML` (cùng renderer và cùng kiểm ảnh như bài đọc).
- `CloneAsDraft`:
  - Câu hỏi, phương án và tiêu chí nhận id mới. `question_key` giữ nguyên.
  - Cấu hình quiz và homework được chép.
  - File kèm chỉ chép tham chiếu `media_id`.

## API (đều qua `requireAdmin`)

- `POST /stage-versions/:vid/lessons` và `PATCH …/:lid` nhận thêm:
  - `type` thuộc `quiz | homework`.
  - `quiz: {mode, maxAttempts, timeLimitMinutes, shuffle, passPercent}`.
  - `homework: {briefSource, passPercent, attachmentMediaIds}`.
- Câu hỏi:
  - `POST /stage-versions/:vid/lessons/:lid/questions` `{questionKey?, type, bodySource, explanationSource, points, options:[{id?, body, correct}]}`.
  - `PATCH …/questions/:qid`, `DELETE …/questions/:qid`, `PUT …/questions/order {questionIds}`.
- Tiêu chí:
  - `POST …/criteria {description, maxPoints}`.
  - `PATCH …/criteria/:cid`, `DELETE …/criteria/:cid`, `PUT …/criteria/order {criterionIds}`.
- Mọi thao tác đi qua `lockDraft` và `mutateVersion`, rồi `SaveDraft`. Không có bảng hay route riêng nào bỏ qua aggregate.
- `LessonDTO` thêm:
  - `quiz{mode, maxAttempts, timeLimitMinutes, shuffle, passPercent, questions[{id, questionKey, type, bodySource, bodyHtml, explanationSource, explanationHtml, points, options[{id, body, isCorrect}]}]}`. DTO này chỉ dùng cho route admin.
  - `homework{briefSource, briefHtml, passPercent, maxPoints, criteria[], attachments[{mediaId, fileName, sizeBytes}]}`.

## Media `document`

- `media/entity.go`:
  - Thêm `KindDocument`.
  - Kind `document` dùng bảng đuôi → content type chuẩn: `.zip` → `application/zip`, `.pdf` → `application/pdf`, `.md` và `.txt` và `.sql` → `text/plain; charset=utf-8`, `.png` → `image/png`, `.jpg`/`.jpeg` → `image/jpeg`. Server quyết định theo đuôi và bỏ qua MIME client gửi (trình duyệt gửi `.sql` là rỗng, `application/sql` hoặc `application/octet-stream`). Đuôi ngoài bảng trả `VALIDATION`. Type chuẩn được ký vào URL PUT và lưu ở `media.content_type`.
  - Upload ticket vẫn trả `headers` từ `PresignPut` (`storage.go:16-17`). Phía web phải gửi đúng các header đó (xem phase 05), không dùng `file.type`.
  - `Limits.MaxDocumentBytes`, lấy từ `MAX_DOCUMENT_BYTES` (mặc định 20 MB) trong `config.go`. Thêm vào `testCfg` ở `media/service_test.go:30`.
- `storage.Storage.PresignGet` đổi tham số cuối thành `GetOptions{ContentType, AttachmentName string}`. MinIO đặt `response-content-disposition=attachment; filename*=UTF-8''…` khi `AttachmentName` khác rỗng. Sửa `mem.go` và các test.
- `media.Service.SignedURL` luôn dùng `attachment` và `response-content-type` bằng type chuẩn đã lưu cho kind `document`; không bao giờ phản chiếu type client gửi. Route `/media/:id/content` thêm `X-Content-Type-Options: nosniff`, header này đã có trong SecurityHeaders; kiểm lại.
- `replaceLessonMedia` ghi cả `homework_attachments.media_id` vào `lesson_media`, nên `CanUserAccess` hiện có cấp quyền tải cho học viên và giảng viên của lớp mà không cần sửa SQL.

## Files

- Sửa `internal/features/stages/`:
  - `entity.go`. Tách `quiz_content.go` và `homework_content.go` vì cùng package và ranh giới thật.
  - `entity_test.go`, `service.go` (`contentFactory`, các lệnh câu hỏi và tiêu chí), `service_test.go`, `repository_pg.go` (load, diff và insert các bảng con trong `SaveDraft` và `Create`), `repository_pg_test.go`, `dto.go`, `handler.go`, `markdown.go`, `testdata/*.json`.
- Sửa `internal/features/media/`: `entity.go`, `service.go`, `service_test.go`, `repository_pg_test.go`.
- Sửa `internal/platform/storage/`: `storage.go`, `minio.go`, `mem.go`, `minio_test.go`.
- Sửa `internal/platform/config/config.go` và `config_test.go`; thêm biến vào `.env.example` nếu có.
- Sửa `internal/features/learning/`: `repository_pg.go` (`LessonRow` đọc được loại mới; nội dung chi tiết ở phase 06 và 09) và `service.go` (`contentOf` trả loại mới mà không lộ đáp án; xem phase 09).
- Sửa `internal/seed/data.go`: helper `quiz(...)`, `homework(...)`. Seed theo prototype:
  - `db-test` (assessment, 2 lượt, 10 phút, 70%), `go-quiz`, `re-test`, `web-test`.
  - Mỗi chặng một quiz luyện tập.
  - `ds-hw` (40/30/30) và `go-hw` (40/40/20).
- Sửa `docs/architecture.md`: mục media (kind `document`, attachment) và mục stages.

## Các bước

1. Test domain trước:
   - Validate cấu hình theo chế độ.
   - Validate câu hỏi (single hay multi, số phương án, đáp án đúng).
   - Đổi practice sang assessment giữ câu hỏi; đổi quiz sang video xóa câu hỏi.
   - Publish đủ bốn lý do chặn.
   - Clone giữ `question_key` và cho id mới.
   - Mọi lệnh trên bản published trả `ErrVersionImmutable`.
2. Repository:
   - `loadQuiz` và `loadHomework` theo lô (một truy vấn mỗi bảng cho cả phiên bản, không N+1).
   - Diff `questions`, `question_options`, `rubric_criteria`, `homework_attachments` với điều kiện `status='draft'` qua `EXISTS`. Defer các UNIQUE position.
   - Test integration: ghi thẳng repository vào bản published không đổi dòng nào (khớp tiêu chí "service và database" của FR-90).
3. Service, handler và DTO, kèm golden `stages/testdata/lesson_quiz.json`, `lesson_homework.json` (cùng kiểu đặt tên gạch dưới với `lesson_markdown.json`).
4. Media `document` và `PresignGet` với attachment. Test `minio_test` khẳng định query `response-content-disposition` có mặt. Test service: file `.sql` gửi kèm `application/octet-stream` được nhận với type chuẩn `text/plain; charset=utf-8`; file `.html` hoặc `.svg` bị từ chối; header trong ticket khớp type chuẩn.
5. Seed nội dung quiz và homework, rồi `make seed-reset` để kiểm tra.
6. Test FR-74: dùng session teacher gọi `POST /questions`, `PATCH /lessons/:lid`, `PUT /questions/order`; mọi lệnh phải trả 403.

## Kiểm chứng

```bash
cd apps/api && make lint && TEST_DATABASE_URL=... make test
grep -rn '"uq_\|"ck_\|"fk_' internal --include='*.go' | grep -v pgerr/constraints.go   # rỗng
```

## Tiêu chí hoàn thành

- [x] FR-60, FR-70 (phần soạn), FR-74 (API) và FR-90 (quiz, homework) có test.
- [x] Bản published bất biến cho câu hỏi, phương án, tiêu chí và cấu hình, ở cả tầng service lẫn tầng repository.
- [x] Tải file kind `document` luôn nhận URL có `attachment`.

## Rủi ro

- Đổi chữ ký `PresignGet` lan tới mọi chỗ gọi. Hiện chỉ có `media/service.go:144` và test; grep lại khi làm.
- `SaveDraft` thêm 4 bảng con làm hàm dài hơn. Tách `saveQuiz` và `saveHomework` trong cùng file `repository_pg.go`, hoặc file `repository_pg_content.go`.
