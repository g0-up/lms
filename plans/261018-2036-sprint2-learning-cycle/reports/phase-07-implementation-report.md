# Báo cáo thực hiện: hàng chờ chấm (backend)

- Phase: `phase-07-grading-queue-backend.md` (FR-64)
- Ngày: 2026-10-19
- Trạng thái: xong. Suite đầy đủ và lint xanh. Không commit.

## Kết quả

Package mới `internal/features/grading` cung cấp một read model hợp nhất cho các mục chờ xử lý:

- `GET /api/v1/grading/queue?kind=homework|quiz&classId=&lessonId=&overdue=1`
- `GET /api/v1/grading/recent?limit=`

Chỉ Admin và giảng viên gọi được. Giảng viên chỉ thấy lớp có `classes.teacher_id` là mình, Admin thấy mọi lớp, học viên nhận 403. Response có `Cache-Control: private, no-store`.

Hàng chờ là một truy vấn `UNION ALL`:

- bản nộp `submissions.status='pending'`;
- lượt quiz `quiz_attempts.review_status='pending'`.

Cả hai nhánh chỉ lấy thành viên `active` và gồm cả lớp đã kết thúc, sắp `submitted_at, item_id`. Lượt quiz chỉ có `scorePercent`, không có đáp án hay điểm từng câu.

`POST /submissions/{sid}/grades` giờ điền `nextQueueItem`: mục cũ nhất còn lại trong hàng chờ của người chấm, bỏ qua bài vừa chấm. Giá trị là null khi sửa điểm hoặc khi hàng chờ đã hết.

`grading` không import `homework`, `quizzes` hay `reports`, đã kiểm bằng `go list`. Hướng phụ thuộc là `homework → grading` qua interface `grading.NextFinder`.

## File đã sửa

Trong danh sách của phase:

- `internal/features/grading/` (mới):
  - `queue.go`: kiểu miền, `GradingSLA`, filter, `buildQueue`, `next`.
  - `repository.go`, `repository_pg.go`, `service.go`, `dto.go`, `handler.go`.
  - `queue_test.go`: 8 unit test.
  - `service_integration_test.go`: 7 integration test.
  - Golden `testdata/queue-teacher.json`, `queue-admin.json`, `queue-recent.json`.
- `internal/features/homework/service.go`:
  - field `next grading.NextFinder` (qua `Deps.Next`);
  - `GradeOutcome.Next`;
  - gọi `Next` trong transaction chấm khi là lần chấm đầu.
- `internal/features/homework/service_integration_test.go`: wire `grading.NewService` và thêm `TestGradeReturnsNextQueueItem`.
- `internal/app/router.go`: `newGrading(gradingSvc).Register(authed, requireStaff)`; truyền `gradingSvc` vào `newHomework`.
- `internal/app/router_test.go`: `TestGradingRoutesRequireSession`.
- `docs/architecture.md`: mục mới "Hàng chờ chấm", sau mục Homework.

Ngoài danh sách, sửa tối thiểu để wiring đúng:

- `internal/app/deps.go`: `newGradingService`, `newGrading`, và `newHomework(d, enq, next)`. Router gọi các constructor ở đây, nên phải sửa file này.
- `internal/features/homework/dto.go`: `GradeOutcomeDTO.NextQueueItem` đổi từ luôn null sang `*grading.QueueItemDTO`, map bằng `grading.ToQueueItemDTO`.

## Bất biến và test chứng minh

| Yêu cầu | Test |
|---|---|
| Thứ tự hợp nhất hai loại, cũ nhất trước | `TestQueueScopeUnifiedOrderAndCounts`: Hương thấy theo thứ tự Bích hw (108h, quá hạn), An quiz (70%), Phong hw, Cường hw. Golden `queue-teacher.json` |
| Giảng viên A không thấy lớp của B; Admin thấy hết | Cùng test trên: Bảo chỉ thấy basic02; Admin thấy 6 mục, quá hạn 2. Golden `queue-admin.json`. Unit `TestActorScope` |
| Lọc `kind`, `classId`, `lessonId`, `overdue`; tham số sai → 422 | `TestQueueFilters` (classId của lớp giảng viên khác → rỗng; option học liệu của Admin không trùng), unit `TestBuildQueueFiltersItemsButCountsWholeQueue`, `TestBuildQueueLessonOptions`, `TestParseFilter` |
| `overdue` ở biên 72 giờ | `TestQueueOverdueBoundary` (`clock.Fake`: đúng 72h → false, thêm 1µs → true), unit `TestWithClockOverdueBoundary` |
| Bản `superseded` không có; thành viên `dropped` bị ẩn khỏi hàng chờ, `total` và `overdueCount` | `TestQueueHidesDroppedMembersAndSuperseded` |
| Nhánh quiz chạy, chỉ có % tổng | Lượt quiz `pending` chèn bằng SQL fixture trong `TestQueueScopeUnifiedOrderAndCounts`; assert response không chứa `answers`, `points`, `"score"` |
| Danh sách gần đây | `TestRecentGradedAndConfirmed`: lần chấm hiện hành, nhận xét quiz hiện hành, bỏ quiz luyện tập, phạm vi giảng viên, `limit`, 422. Golden `queue-recent.json` |
| `Next` bỏ qua mục vừa xử lý, trong phạm vi actor | `TestNextSkipsHandledItemWithinScope`, unit `TestNextSkipsHandledItem` |
| `nextQueueItem` sau khi chấm | `homework` `TestGradeReturnsNextQueueItem`: Hương chấm An → Khang; sửa điểm → null; Admin chấm Khang → Linh (lớp khác); Bảo chấm Linh → null |
| Route cần phiên, cần staff | `app` `TestGradingRoutesRequireSession` (401), `TestGradingRoutesRequireStaff` (học viên 403) |

## Lệnh và kết quả

- `gofmt -l ./internal`: sạch.
- `make -C apps/api test` (`-race -count=1 -p 1 -tags integration ./...`): tất cả package `ok`, gồm `grading` 11.6s và `homework` 21.4s.
- `make -C apps/api lint GOLANGCI_LINT="go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0"`: `go vet` thường và `-tags integration` sạch; golangci-lint `0 issues`.
- Golden đã đọc từng dòng: thứ tự, `waitedHours`, `overdue`, `scorePercent` null cho homework.
- `EXPLAIN` trên `lms_test` sau khi seed, với `SET enable_seqscan=off`: `Index Scan using ix_submissions_pending` và `Bitmap Index Scan on ix_quiz_attempts_pending`. Với cài đặt mặc định và seed nhỏ, planner có thể chọn seq scan vì bảng chỉ vài chục dòng. Điều đó là bình thường; index đúng điều kiện partial nên sẽ được dùng khi dữ liệu lớn.

## Khác với plan và quyết định

1. Chữ ký `NextFinder.Next(ctx, ex db.Executor, actor, after)` thêm `ex` so với `Service.Next(ctx, actor, after)` trong plan. `homework` gọi trong transaction chấm, vì hai lý do. Bài vừa chấm đã rời hàng chờ nên kết quả nhất quán. Lỗi đọc thì rollback cả lần chấm, thay vì báo lỗi sau khi đã commit (khi email đã vào outbox). Phase 10 (quiz) dùng cùng cách.
2. `recent` chỉ gồm nhận xét của lượt quiz bài tập đã `confirmed`, theo prototype. Lượt luyện tập không vào danh sách. Mỗi bài nộp hoặc lượt chỉ lấy lần chấm hoặc nhận xét mới nhất. Không lọc theo trạng thái thành viên: lịch sử chấm vẫn hiện với học viên đã rời lớp.
3. `total`, `quizPending`, `overdueCount` đếm trên toàn hàng chờ của actor, không theo bộ lọc, giống prototype. `filters.lessons` thu hẹp theo lớp và loại đang chọn, mỗi học liệu một lần; quiz chỉ gồm `mode='assessment'`.
4. `waitedHours` làm tròn xuống theo giờ và không âm. `overdue` là chờ `> 72h`, tức đúng 72h chưa quá hạn, khớp `submittedAt < now - 72h` trong plan.
5. `grading` dùng `reports.ReadTx` (REPEATABLE READ READ ONLY) qua wiring ở `app/deps.go`. Package `grading` chỉ thấy interface `db.Tx` nên không import `reports`.
6. Tồn đọng (`BacklogReader`) thuộc phase 14 nên chưa tạo. `Repo.Queue(ctx, ex, teacherID *uuid.UUID)` đã trả `teacher_id` từng mục và nhận nil cho phạm vi Admin, nên phase 14 dùng lại trực tiếp.
7. Quyết định đã ghi: bài của học viên đã rời lớp, hoặc của lớp đã kết thúc, vẫn chấm được qua link trực tiếp. Hàng chờ chỉ ẩn thành viên không `active`; lớp `ended` vẫn có trong hàng chờ.

## Câu hỏi mở

- Bảng Phases trong `plan.md` vẫn ghi `pending` cho mọi phase. Tôi chỉ đổi `status` của file phase 07 và để orchestrator cập nhật bảng.
- Web phase 08 cần biết các điểm sau:
  - `nextQueueItem` có cùng shape với một mục của `GET /grading/queue`;
  - response không có envelope `data`;
  - `classId` hoặc `lessonId` sai định dạng trả 422, không phải danh sách rỗng.
