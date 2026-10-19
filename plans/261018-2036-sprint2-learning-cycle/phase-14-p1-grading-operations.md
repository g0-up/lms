---
phase: 14
title: "P1 vận hành chấm: thời hạn, nhắc, tồn đọng"
status: completed
priority: P1
effort: "2d"
dependencies: [10, 13]
---

# Phase 14: P1 vận hành chấm (thời hạn, nhắc, tồn đọng)

## Mục tiêu

- FR-65: bài nộp và lượt quiz bài tập chờ quá 3 ngày được đánh dấu nổi bật. Hàng chờ đã có phần này từ phase 07 và 08; phase này thêm test hợp đồng và cảnh báo trên dashboard.
- FR-66: mỗi ngày gửi một email tổng hợp cho giảng viên có mục quá hạn. Không có mục quá hạn thì không gửi.
- FR-67: Admin thấy tồn đọng theo từng giảng viên: số mục chờ, số mục quá hạn, thời gian xử lý trung bình.
- FR-68: gửi email nhắc học viên chưa nộp, 24 giờ trước hạn. Homework không có hạn thì không nhắc.

## Context

- Spec FR-65 đến FR-68 và OQ-5 ([plan.md](plan.md) §3: 72 giờ tính từ `submitted_at`; nhắc giảng viên phụ trách lớp).
- Prototype ([scout-prototype](reports/scout-prototype.md) §8):
  - `teacherBacklog` 648-658: thời gian trung bình tính từ lúc nộp tới lần chấm đầu hoặc lần xác nhận đầu, trên toàn bộ mục đã xử lý.
  - `runDailyJobs` 661-681.
  - `backlogCard` 993-999.
  - Alert "N bài chờ chấm hoặc xác nhận quá 3 ngày." dẫn tới `/admin/grading?overdue=1`.
- Code: `mailer/worker.go` (`Cleanups` chạy mỗi giờ, `lastCleanup` giữ trong bộ nhớ), `cmd/lms/worker.go`, `reports/service.go` `Dashboard`, `grading.GradingSLA` (phase 07), bảng `scheduled_job_runs` (phase 01).

## Thiết kế

### Việc hằng ngày (package `grading`, file `jobs.go`)

`grading.Jobs` hiện thực `mailer.Cleanup` và được đăng ký vào `Cleanups` trong `cmd/lms/worker.go`. Nó chạy mỗi giờ; bảng `scheduled_job_runs` bảo đảm mỗi khóa chỉ được enqueue một lần, kể cả khi worker khởi động lại hoặc chạy nhiều replica. Outbox giao ít nhất một lần, nên khi SMTP lỗi giữa chừng có thể hiếm khi trùng email.

- **Tổng hợp chấm trễ (FR-66):**
  - Chỉ chạy khi giờ địa phương `Asia/Ho_Chi_Minh` ≥ 08:00. Ngày tính theo giờ địa phương.
  - Với mỗi giảng viên không bị khóa có ít nhất một mục quá hạn (dùng cùng truy vấn hàng chờ của phase 07, lọc theo lớp giảng viên phụ trách, `overdue=1`), trong một transaction:
    1. `INSERT INTO scheduled_job_runs (run_key, ran_at) VALUES ('grading_digest:<teacherId>:<yyyy-mm-dd>', now) ON CONFLICT DO NOTHING`.
    2. Nếu chèn được một dòng thì enqueue email `grading_digest`. Nếu không thì bỏ qua, vì hôm nay đã gửi.
  - Tiêu đề lấy từ prototype: "{n} bài chờ chấm hoặc xác nhận quá 3 ngày: {Tên} (quiz|bài tập), …". Nội dung liệt kê từng mục (học viên, lớp, học liệu, đã chờ bao lâu) và link ở khóa payload `QueueURL` = `PublicBaseURL + "/teach/grading?overdue=1"` ([plan.md](plan.md) §4 mục 5).
  - Lớp `ended` vẫn tính, vì hàng chờ phase 07 vẫn hiện bài của lớp đã kết thúc.
- **Nhắc hạn nộp (FR-68):**
  - Chọn các cặp (thành viên, homework) thỏa: hạn nộp trong `(now, now + 24h]`, lớp `active`, thành viên `active`, tài khoản học viên không bị khóa, và chưa có bản nộp nào.
  - Với mỗi cặp, trong một transaction: chèn khóa `deadline_reminder:<memberId>:<lessonId>:<dueAt RFC3339 UTC>` với `ON CONFLICT DO NOTHING`, rồi enqueue `deadline_reminder` nếu chèn được.
  - `dueAt` nằm trong khóa, nên khi giảng viên đổi hạn nộp, học viên được nhắc lại theo hạn mới. Prototype cũng làm vậy.
  - Tiêu đề: "Nhắc hạn nộp: {học liệu} · hạn {dd/mm/yyyy hh:mm}". Giờ hiển thị theo `Asia/Ho_Chi_Minh`. Nội dung có link tới trang homework ở khóa payload `LessonURL`.
- Mỗi job dùng truy vấn tập hợp, không N+1, và một transaction cho mỗi email. Lỗi của một email chỉ được ghi log và không chặn các email khác.
- Không ghi audit cho email tự động. Prototype có ghi dòng "Chạy tác vụ email hằng ngày", nhưng nút đó là điều khiển giả lập và đã ở ngoài phạm vi ([plan.md](plan.md) §2). Nhật ký outbox đủ để tra cứu.

### Mail

- `mailer/entity.go`: thêm `TemplateGradingDigest` và `TemplateDeadlineReminder`. CHECK đã được sửa ở phase 01.
- Thêm `templates/grading_digest.{html,txt}.tmpl` và `templates/deadline_reminder.{html,txt}.tmpl`, cùng chuỗi tiêu đề. Cả hai template đều không chứa dữ liệu bí mật.

### Dashboard Admin (FR-65, FR-67)

- `GET /dashboard` thêm hai trường:
  - `gradingBacklog[{teacherId, teacherName, pending, overdue, avgHours}]`: mọi giảng viên không bị khóa, sắp theo tên.
    - `pending` và `overdue` tính trên hàng chờ của các lớp giảng viên phụ trách.
    - `avgHours` là trung bình (lần chấm đầu − `submitted_at`) của bài nộp **do giảng viên đó chấm lần đầu**, gộp với (lần xác nhận đầu − `submitted_at`) của lượt quiz bài tập **do giảng viên đó xác nhận lần đầu**, trên toàn bộ dữ liệu như prototype. Không có mục nào thì trả null.
  - `overdueCount`: tổng số mục quá hạn của toàn hệ thống.
- `grading` thêm `Backlog(ctx, now) ([]TeacherBacklog, overdueTotal, error)`, dùng lại đúng truy vấn hàng chờ của phase 07, để hai nơi không thể lệch số. `reports.Service.Dashboard` gọi nó qua interface `BacklogReader` khai báo trong `reports`. `grading` không import `reports` nên hướng import `reports → grading` không tạo vòng.

### Web

- `src/features/dashboard/`:
  - `components/backlog-card.tsx` "Tồn đọng chấm bài": cột "Giảng viên", "Chờ chấm, xác nhận", "Quá 3 ngày" (badge danger khi > 0), "TG chấm TB" ("n giờ" khi < 24, ngược lại "x,y ngày", "—" khi null). Link "Hàng chờ chấm".
  - Alert ở đầu dashboard khi `overdueCount > 0`: "N bài chờ chấm hoặc xác nhận quá 3 ngày." kèm link `/admin/grading?overdue=1`.
  - Schema và golden được cập nhật; `format-hours.ts` cùng test.

## Files

- Tạo `internal/features/grading/jobs.go`, `jobs_test.go`, `jobs_integration_test.go`.
- Tạo `internal/features/mailer/templates/grading_digest.{html,txt}.tmpl`, `deadline_reminder.{html,txt}.tmpl`; sửa `mailer/entity.go`, `templates.go`, `templates_test.go`.
- Sửa `cmd/lms/worker.go`: đăng ký `grading.Jobs` vào `Cleanups`, truyền `PublicBaseURL`.
- Sửa `internal/app/deps.go` (khoảng dòng 195, nơi dựng `reports`) để truyền `grading` làm `BacklogReader`; sửa các chỗ dựng `reports.Service` trong test: `reports/service_test.go:140`, `reports/repository_pg_test.go:114`.
- Tạo `internal/features/grading/backlog.go` cùng test; sửa `grading/repository_pg.go`.
- Sửa `internal/features/reports/`: `service.go` (`Dashboard`, interface `BacklogReader`), `dto.go`, golden `dashboard.json`; sửa `internal/app/router.go` để truyền `grading` vào `reports`.
- Sửa `apps/web/src/features/dashboard/`: thêm `components/backlog-card.tsx`, `model/format-hours.ts`; sửa `pages/dashboard-page.tsx`, `model/schemas.ts`, `msw-handlers.ts`. Cập nhật fixture dashboard dùng trong `src/app/app-shell.test.tsx:6` và `session-isolation.test.tsx:6` nếu chúng mock `GET /dashboard` (việc nền, [plan.md](plan.md) §4 mục 10).
- Sửa `docs/architecture.md`: mục việc hằng ngày, khóa khử trùng lặp, múi giờ.

## Test

- Unit, dùng `clock.Fake`:
  - 07:59 giờ Việt Nam thì không gửi tổng hợp; 08:00 thì gửi.
  - Hàm chọn cặp nhắc hạn: hạn đúng `now + 24h` được nhắc; hạn bằng `now` không được nhắc.
- Integration:
  - Chạy `Jobs.Cleanup` hai lần trong cùng một ngày: mỗi giảng viên chỉ có một email tổng hợp.
  - Sang ngày hôm sau, nếu vẫn còn mục quá hạn, giảng viên nhận email mới.
  - Giảng viên không có mục quá hạn thì không nhận email.
  - Giảng viên bị khóa thì không nhận email.
  - Học viên đã nộp, lớp `ended`, và homework không có hạn thì đều không được nhắc. Đổi hạn nộp thì học viên được nhắc lại.
  - Hai goroutine chạy job cùng lúc: chỉ có một dòng outbox, nhờ khóa chính của `scheduled_job_runs`.
  - Payload hai template enqueue thành công (không có khóa `Link`) và render được `QueueURL`, `LessonURL`.
  - `GET /dashboard` khớp golden; `avgHours` đúng trên dữ liệu có thời điểm cố định. `overdueCount` bằng tổng `overdue` của hàng chờ Admin.
- Web: `backlog-card` render đúng ba dạng thời gian; alert chỉ hiện khi `overdueCount > 0`.

## Kiểm chứng

```bash
cd apps/api && make lint && TEST_DATABASE_URL=... make test
pnpm -C apps/web lint typecheck test lint:design
```

## Tiêu chí hoàn thành

- [x] FR-65 đến FR-68 có test, gồm cả các trường hợp không gửi.
- [x] Mỗi khóa email tự động chỉ được enqueue một lần, kể cả khi worker khởi động lại.

## Rủi ro

- `Cleanups` chạy mỗi giờ, nên email tổng hợp có thể đến muộn tới một giờ sau 08:00, và email nhắc có thể đến khi còn từ 23 đến 24 giờ. Spec không đòi chính xác tới phút.
- Worker dừng quá 24 giờ trước hạn nộp thì mất email nhắc. Chấp nhận được, vì chỉ là email nhắc.
