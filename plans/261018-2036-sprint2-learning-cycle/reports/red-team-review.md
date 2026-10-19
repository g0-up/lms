# Red team review: plan Sprint 2 vòng học khép kín

- Ngày: 2026-10-18
- Plan: [plan.md](../plan.md)
- Reviewer: bốn persona (bảo mật, lỗi vận hành, giả định, phạm vi và hợp đồng).
- Kết quả: gộp thành 17 phát hiện. Nhận 15 (một mục nhận một phần), bỏ 2.
- Người dùng chọn "Áp dụng hết".
- Mỗi phát hiện được nhận đều đã được kiểm bằng chứng `file:line` trong code trước khi sửa plan.

## Phát hiện được nhận

| # | Mức | Phát hiện | Bằng chứng | Phase đã sửa |
|---|---|---|---|---|
| 1 | High | Action audit mới thiếu trong `knownActions`, nên `PG.Record` từ chối và giao dịch rollback. Nhật ký hoạt động thiếu nhãn và tóm tắt. | `platform/audit/audit.go:32`, `reports/model.go:417` | plan §4 mục 7; 06, 10, 12, 15 |
| 2 | High | Bản vá tạm ở phase 01 làm hỏng `CreateStage` (v1 rỗng) và `Clone`. Unique tên bài học phân biệt hoa thường. Bước sửa `truncateAll`/`testdb.Reset` là thừa. | `seed/seed.go:171`, `testdb/testdb.go:98-118` | 01, 02 |
| 3 | High | Chấm bài và nộp lại chạy đua, nên bài đã Đạt có thể bị đổi thành "bị thay thế". | phase 06 bản trước | plan §4 mục 9; 06 |
| 4 | High | Quiz khóa lượt trước thành viên, trái quy tắc chung. "Tự nộp, commit rồi trả lỗi" chạy trong `Transact`, mà `Transact` rollback khi trả lỗi. Lưu câu trả lời không khóa lượt. | `platform/db` `Transact` | plan §4 mục 9; 09, 15 |
| 5 | High | Lưu và nộp quiz không kiểm thành viên và lớp còn `active`. Hàng chờ hiện học viên đã rời lớp. | phase 07, 09 bản trước | 07, 09 |
| 6 | High | Truy cập chéo lớp: `:mid` hoặc `:svid` của lớp khác đọc được qua drawer, review và cấp lượt. | `reports.MemberReport` là mẫu đúng | plan §4 mục 8; 12, 15 |
| 7 | High | Test chống lộ đáp án chỉ cấm hai khóa. | phase 09, 10 bản trước | 09, 10 |
| 8 | High | Upload của học viên: nhóm `/media` chỉ Admin; `InitUpload` chặn người không phải Admin; chưa có `requireStudent`. | `router.go:59,65,72`, `media/service.go:74,98` | 06, 09 |
| 9 | Medium | Content type tài liệu: client phải gửi đúng header đã ký; allow-list theo đuôi file; tải xuống luôn là `attachment`. | `storage/storage.go:16-17`, `stages/api/upload-media.ts:35` | 04, 05 |
| 10 | High | Mailer cấm khóa `Link`. Việc quét chạy trong `Cleanups` mỗi giờ, không thêm `Sweeps` mỗi lần poll. "Đúng một lần" sửa thành "enqueue một lần, giao ít nhất một lần". | `mailer/service.go:16`, `mailer/worker.go:25,65-88`, `classes/service.go:636` | plan §4 mục 5; 06, 09, 10, 14, 16 |
| 11 | High | Tab mặc định của giảng viên phải giữ `report`. Thanh tab nhận danh sách tab qua props. | `prototype/app.js:1394`, `class-tabs.test.ts:5` | 08, 11, 13 |
| 12 | High | Ẩn cả `complete-bar` làm mất `LessonNav`; `TickCheckbox` trên `lesson-row` bị bỏ sót. | `complete-bar.tsx:33`, `lesson-row.tsx:41` | 08, 13 |
| 13 | High | Enum `lessonTypeSchema` và `lessonSub` chưa có quiz và homework. Câu ghi chú tự xác nhận cần đổi. | `learning/model/schemas.ts:10`, `reports/model/schemas.ts:111`, `report-panel.tsx:90` | 08 |
| 14 | Medium | Gold plating và trích sai: badge nav, zod `.strict()`, tên golden, `ErrOrderMismatch`, `day.ts`. `ChangeCourseVersion` để lại hạn nộp mồ côi. Sửa `src/app`, `src/shared` chưa ghi là việc nền. | `stages/entity.go:25`, `stages/testdata/`, `learning/testdata/` | plan §4 mục 10; 01–16 |
| 15 | Medium | Thiếu test: FR-61/62, FR-71 tiếp tục lượt, FR-77 đếm câu trống, chặng không có mục bắt buộc. `page.clock` không chứng minh được server tự nộp. | spec FR-61, 62, 71, 77 | 06, 09, 11, 12, 16 |

### Điều chỉnh khi áp dụng

- Mục 13, phần `selfReported`: giữ `z.literal(true)`. Server vẫn trả hằng `true` (`reports/service.go:101,129`), nên chỉ đổi câu ghi chú theo prototype (`app.js:1368`) và sửa test `report-panel.test.tsx:49,233`.
- Mục 11: thanh tab chuyển thành `src/shared/ui/tab-bar.tsx` để `classes` và `reports` cùng dùng mà không có import vòng.
- Mục 2: phase 01 tạo bài học mặc định trong `CreateStage`. Phase 02 gỡ bản vá này để chặng mới có v1 rỗng như prototype (`app.js:165`).
- Mục 15, chặng không có học liệu bắt buộc: phase 12 trả `Complete=false`, khớp `domain.Percent` trả 0. Cần người dùng xác nhận khi validate.

## Phát hiện bị bỏ

| Phát hiện | Lý do |
|---|---|
| `homework_attachments` trùng `lesson_media` | Không trùng. `lesson_media` không có cột thứ tự và còn chứa ảnh nhúng trong markdown (`0004_…up.sql:61-66`). `homework_attachments` giữ danh sách file đính kèm có thứ tự. |
| Migration 0010 phải chia expand rồi contract | Prod chỉ có một API instance với `MIGRATE_ON_START=true`. Phase 01 ghi chú: sao lưu DB, rồi deploy image mới; không chạy migration trước bằng image cũ. |
| Job dọn file mồ côi (một phần của mục 8) | Giới hạn 10 file chưa gắn cho mỗi học viên đã chặn lạm dụng. Việc dọn file vẫn là nợ kỹ thuật, đã ghi ở phase 06. |

## Câu hỏi còn mở

- Chặng không có học liệu bắt buộc: `Complete=false` (đề xuất) hay coi là hoàn thành.
- Bài của học viên đã rời lớp: ẩn khỏi hàng chờ (đề xuất) hay vẫn hiện.
