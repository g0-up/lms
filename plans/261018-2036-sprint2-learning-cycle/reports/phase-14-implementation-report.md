# Phase 14: vận hành chấm (thời hạn, nhắc, tồn đọng), báo cáo triển khai

- Ngày: 2026-10-19
- Trạng thái: hoàn thành.
  - `make -C apps/api test` và `make -C apps/api lint` xanh.
  - Web `typecheck`, `lint`, `lint:design` và `test` xanh.
  - Chưa chạy Playwright theo yêu cầu.
- Không commit, không thêm migration (`scheduled_job_runs` đã có từ trước).

## Kết quả

- **Email tổng hợp chấm trễ (FR-66):**
  - `lms worker` chạy `grading.Jobs` mỗi giờ. Từ 08:00 theo `APP_TZ`, mỗi giảng viên không bị khóa có mục quá 72 giờ trong hàng chờ các lớp mình phụ trách nhận một email `grading_digest` mỗi ngày địa phương.
  - Lớp đã kết thúc vẫn được tính. Link trong email: `/teach/grading?overdue=1`.
- **Nhắc hạn nộp (FR-68):**
  - Học viên nhận email `deadline_reminder` khi thỏa: hạn nộp nằm trong `(now, now+24h]`, lớp và thành viên `active`, tài khoản không bị khóa, chưa có bản nộp.
  - Đổi hạn nộp thì học viên được nhắc lại. Homework không có hạn thì không nhắc. Link mở trang homework của học viên.
- **Khử trùng lặp:**
  - Mỗi email chạy trong một transaction: `INSERT … ON CONFLICT DO NOTHING` vào `scheduled_job_runs`, rồi enqueue nếu chèn được dòng.
  - Khóa: `grading_digest:<teacherId>:<yyyy-mm-dd địa phương>` và `deadline_reminder:<memberId>:<lessonId>:<dueAt RFC3339 UTC>`.
  - Lỗi enqueue thì rollback khóa của email đó và ghi log, không chặn email khác; lần chạy sau sẽ thử lại.
- **Dashboard Admin (FR-65, FR-67):**
  - `GET /dashboard` thêm `gradingBacklog[{teacherId, teacherName, pending, overdue, avgHours|null}]` (theo tên) và `overdueCount`.
  - Hai trường này đọc qua `reports.BacklogReader`, do `grading.Service.Backlog` hiện thực, cùng truy vấn hàng chờ và cùng snapshot với phần còn lại của dashboard.
- **Web:**
  - Thẻ "Tồn đọng chấm bài" với các cột Giảng viên, Chờ chấm/xác nhận, Quá 3 ngày (badge danger khi > 0) và TG chấm TB ("n giờ", "x,y ngày", "—"), cùng link "Hàng chờ chấm".
  - Alert "N bài chờ chấm hoặc xác nhận quá 3 ngày." với nút "Xem bài quá hạn chấm" tới `/admin/grading?overdue=1`; chỉ hiện khi `overdueCount > 0`.

## File

### API `apps/api/`

- `internal/features/grading/`:
  - Mới: `jobs.go`, `backlog.go`, `jobs_test.go`, `backlog_test.go`, `jobs_integration_test.go`, `backlog_integration_test.go`.
  - Sửa:
    - `repository.go` và `repository_pg.go`: thêm `Teachers`, `AvgHandledHours`, `ReminderCandidates` và `ClaimRun`.
    - `service.go`: hàng chờ dùng `stamp`, dùng chung với backlog.
    - `service_integration_test.go`: thêm helper `newITAt`.
- `internal/features/mailer/`:
  - `entity.go`: thêm hai template.
  - `templates.go`: thêm hai tiêu đề.
  - `templates_test.go`.
  - `templates/grading_digest.{html,txt}.tmpl` và `templates/deadline_reminder.{html,txt}.tmpl`.
- `cmd/lms/worker.go`: đăng ký `grading.Jobs` vào `Cleanups`, truyền `Location` và `PublicBaseURL`.
- `internal/app/deps.go` và `router.go`: truyền `gradingSvc` làm `BacklogReader`. Sửa bằng Edit, mỗi file một dòng.
- `internal/features/reports/`:
  - `service.go`: thêm `BacklogReader` và `Deps.Backlog`; `Dashboard` gọi backlog trong transaction.
  - `dto.go`: thêm `GradingBacklogDTO`.
  - `model.go`: thêm 2 field vào `Dashboard`. Đây là **file ngoài danh sách**, team-lead đã cho phép; không đụng `actionLabels`, `activityPayload` hay `summarize`.
  - `service_test.go`, `repository_pg_test.go`, `testdata/dashboard.json`.
- `docs/architecture.md`: mục "Việc hằng ngày: email chấm trễ và nhắc hạn nộp" (khóa khử trùng lặp, `APP_TZ`, mỗi email một transaction, không ghi audit). Đã sửa thêm đoạn worker trong mục outbox và bullet Dashboard.

### Web `apps/web/src/features/dashboard/`

- Mới:
  - `components/backlog-card.tsx`.
  - `model/format-hours.ts` cùng `format-hours.test.ts`.
- Sửa:
  - `model/schemas.ts`: thêm `teacherBacklogSchema`, `gradingBacklog`, `overdueCount`.
  - `components/outdated-alerts.tsx`: alert chấm trễ.
  - `pages/dashboard-page.tsx`.
  - `pages/dashboard-page.test.tsx`.
- Không cần sửa `api/msw-handlers.ts`, `app-shell.test.tsx` hay `session-isolation.test.tsx`: chúng đọc thẳng golden `dashboard.json` của API, nên đã có hai trường mới.

## Test

- **Unit grading** (`clock.Fake`):
  - 07:59 không gửi, 08:00 gửi, chạy lại không gửi thêm.
  - Mục quá hạn sau 08:00 vẫn được gửi trong cùng ngày; ngày địa phương tính theo `APP_TZ`.
  - Cửa sổ nhắc lấy `now+24h` và bỏ `now`.
  - Payload và khóa của email nhắc.
  - Lỗi một email không chặn email khác và được thử lại lần sau; lỗi đọc của digest không chặn reminder.
  - Nhãn "x,y ngày".
  - `buildBacklog` và `overdueByTeacher`.
- **Integration grading:**
  - Digest một lần mỗi ngày và lại có vào ngày sau.
  - Giảng viên không có mục quá hạn, hoặc bị khóa, thì không nhận.
  - Không nhắc khi: học viên đã nộp, học viên bị khóa, lớp `ended`, homework không có hạn. Đổi hạn thì được nhắc lại.
  - Biên `now`, `now+24h` và `now+24h+1s`.
  - Hai goroutine chạy cùng lúc: mỗi khóa chỉ có một dòng outbox.
  - Payload qua jsonb render được `QueueURL` và `LessonURL`, không có khóa `Link`.
  - `avgHours` trên dữ liệu có thời điểm cố định (32/3, 6, null); `pending` và `overdue` khớp hàng chờ.
- **Mailer:** render hai template mới (escape HTML, link, thiếu `LessonURL` thì lỗi).
- **Reports:**
  - Unit `TestDashboard`: map backlog, `now` từ clock, lỗi backlog được trả lên.
  - `TestHTTPDashboard` khớp golden. `overdueCount` bằng 3, bằng `OverdueCount` của hàng chờ Admin và bằng tổng `overdue`.
- **Web:**
  - `format-hours`: ba dạng thời gian.
  - Dashboard: thẻ backlog render 19 giờ, 1,0 ngày và "—", badge chỉ hiện khi > 0.
  - Alert hiện khi `overdueCount = 3` với link `?overdue=1`; ẩn khi bằng 0.
  - Khối alert biến mất khi không còn việc gì.

## Lệnh đã chạy

| Lệnh | Kết quả |
|------|---------|
| `make -C apps/api test` | pass, 27 package `ok` |
| `make -C apps/api lint GOLANGCI_LINT="go run …golangci-lint@v2.14.0"` | 0 issues; `go vet` có và không có tag integration đều sạch |
| `gofmt -l internal cmd` | sạch |
| `corepack pnpm typecheck` / `lint` / `lint:design` (trong `apps/web`) | pass / pass / 0 violations |
| `corepack pnpm test` (trong `apps/web`) | 97 file, 1008 test pass |

## Sai khác và vấn đề gặp phải

1. `Backlog(ctx, ex db.Executor, now)` nhận thêm transaction (plan ghi `Backlog(ctx, now)`), để số tồn đọng cùng snapshot REPEATABLE READ với phần còn lại của dashboard.
2. `avgHours` không làm tròn ở API; web lo phần hiển thị. Golden vẫn ổn định vì clock test cắt về giây.
3. "TG chấm TB" so sánh số giờ **sau khi làm tròn** với 24, nên 23,87 giờ hiện "1,0 ngày" chứ không phải "24 giờ" như prototype.
4. Học viên `invited` (chưa đăng nhập lần đầu, không bị khóa) vẫn được nhắc hạn, đúng câu chữ "tài khoản học viên không bị khóa".
5. Thẻ backlog nằm cùng lưới hai cột với "Lớp học" và "Nhật ký thao tác". Thẻ "Email tự động" của prototype là điều khiển giả lập, ngoài phạm vi.
6. Database `lms_test` dirty ở version 10:
   - Lúc chạy `make test` đầy đủ lần đầu, mọi package integration fail; schema thật lúc đó là version 11.
   - Tôi không được quyền sửa nên đã báo team-lead, không vòng qua.
   - Sau đó DB đã về version 14 sạch và bộ test đầy đủ xanh.
   - Nguyên nhân dirty chưa rõ: trông như một lần chạy 0011.down trước đó bị gián đoạn, có thể ngoài lane này.

## Câu hỏi còn mở

- Không có.
