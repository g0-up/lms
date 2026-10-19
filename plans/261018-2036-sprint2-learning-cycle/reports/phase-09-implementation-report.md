# Báo cáo thực hiện: quiz engine (backend)

- Phase: `phase-09-quiz-engine-backend.md` (FR-71)
- Ngày: 2026-10-19
- Trạng thái: xong, có vài điểm cần team-lead quyết (cuối báo cáo). Suite đầy đủ và lint xanh. Không commit.

## Kết quả

Package mới `internal/features/quizzes` cho học viên làm cả hai loại quiz:

- `GET`/`POST /api/v1/me/classes/{id}/lessons/{lid}/attempts`: màn bắt đầu; bắt đầu lượt (201) hoặc trả lượt đang mở với `resumed: true` (200).
- `GET /api/v1/me/attempts/{aid}`: lượt mở hoặc kết quả.
- `PUT /api/v1/me/attempts/{aid}/answers/{qid}` `{optionIds}`: lưu từng câu, mảng rỗng là bỏ chọn.
- `POST /api/v1/me/attempts/{aid}/submit`.

Server ghi giờ bắt đầu và giờ hết hạn, chấm toàn bộ ở server bằng hàm thuần `Score`, và tự nộp lượt quá `deadline + 30s` theo hai đường: đọc lười trong transaction riêng, và `CloseExpired` trong `Cleanups` của worker (mỗi giờ, tối đa 100 lượt, mỗi lượt một transaction). Nộp tay, tự nộp khi đọc và worker dùng chung `autoOrSubmit`. Hàm này trả `Outcome` chứ không trả lỗi, nên lần tự nộp vẫn commit khi `POST …/submit` trả 409.

Đáp án không rời server trước khi nộp. Quiz đánh giá không bao giờ trả đúng/sai về phía học viên. Có ba DTO riêng: `OpenAttemptDTO`, `PracticeResultDTO` và `AssessmentResultDTO`. Integration test duyệt JSON thô theo allow-list khóa, kèm denylist dự phòng. Trang học liệu quiz của `learning` chỉ trả cấu hình, số câu và tổng điểm; số câu và tổng điểm đếm trong SQL.

Quy tắc theo các quyết định đã chốt:

- Quiz đánh giá mặc định 2 lượt cộng lượt cấp thêm (`quiz_attempt_grants`), và lượt tốt nhất được tính.
- Hoàn thành ghi trong cùng transaction nộp: luyện tập ở lần nộp đầu, đánh giá ở lượt đạt đầu tiên. Mốc này không bao giờ bị xóa.
- Lượt đánh giá nộp xong có `review_status = 'pending'`, và lượt `pending` cũ của cùng thành viên và học liệu chuyển sang `superseded`.
- Mọi đường ghi khóa `class_members` trước rồi mới khóa `quiz_attempts`. `uq_quiz_attempts_one_open` map sang 409 `CONFLICT`.
- Lớp khác, học liệu không phải quiz, hay lượt của người khác đều trả 404.
- `grading` không import `quizzes`; đã kiểm bằng `go list -deps ./internal/features/grading`, kết quả rỗng.

## File đã sửa

Trong lane của phase:

- `internal/features/quizzes/` (mới), tổng cộng 3733 dòng gồm test và golden:
  - `entity.go`, `scoring.go`, `service.go`, `repository.go`, `repository_pg.go`, `dto.go`, `handler.go`;
  - unit test `entity_test.go` và `scoring_test.go` (10 test), integration test `service_integration_test.go` (15 test);
  - golden `testdata/attempt-open.json`, `attempt-result-practice.json`, `attempt-result-assessment.json`, `attempts-start.json`.
- `internal/features/learning/`:
  - `repository.go`, `repository_pg.go` (`QuestionCount`, `QuizMaxScore` của `LessonRow`), `service.go` (`contentOf` của quiz), `dto.go`;
  - `service_integration_test.go` (JSON thô của trang quiz không chứa `isCorrect`, `correct`, `options`, `questions` hay nội dung câu);
  - golden mới `testdata/lesson-quiz-attempt.json`.
- `internal/app/deps.go` (`NewQuizzesService`, `newQuizzes`), `router.go` (biến `requireStudent` dùng chung cho `learning` và `quizzes`), `router_test.go`.
- `cmd/lms/worker.go`: thêm `quizzes` vào `Cleanups`.
- `internal/seed/data.go`, `seed.go`, `seed_test.go`, `seed_integration_test.go`: 33 lượt quiz theo `seed.js` (qa1–qa32 cộng lượt luyện tập Go đang làm dở của Bích), giờ tương đối với `now`. `checkData` kiểm lượt seed theo luật của service, và integration test của seed kiểm điểm, đúng/sai, hạn, trạng thái xác nhận và mốc hoàn thành.
- `docs/architecture.md`: mục mới "Quiz: làm bài, chấm, tự nộp", worker tự nộp lượt quiz, ghi chú golden của grading.
- File phase: tích ba tiêu chí hoàn thành, `status: completed`.

Ngoài lane. Các file test này hỏng vì dữ liệu seed mới; tôi chỉ sửa test và golden, không sửa code:

- `internal/features/reports/repository_pg_test.go` và golden `class-report.json`, `dashboard.json`, `member-report.json`. An có thêm 5 học liệu bắt buộc hoàn thành nhờ lượt quiz: `RequiredDone` 11 → 16, `Percent` 44 → 64; số học viên dưới mức 6 → 5. Tôi đã so diff golden trước/sau (dựng lại bản trước bằng seed không có lượt quiz): chỉ đổi tiến độ, phần trăm và `avgPercent`.
- `internal/features/grading/service_integration_test.go` và golden `queue-teacher.json`, `queue-admin.json`, `queue-recent.json`:
  - `seedQuiz` của test chuyển từ `ds-test`/`ds-quiz` sang `web-test`/`web-practice`, vì seed giờ có lượt của An ở hai học liệu cũ (trùng `uq_quiz_attempts_member_lesson_no`).
  - Danh sách và số đếm mong đợi cập nhật theo 6 lượt chờ xác nhận của seed. Hà db-test lượt 2 chờ khoảng 84 giờ nên `overdueCount` tăng, và mục kế tiếp sau bài của Bích là lượt quiz của Hà.
  - Test còn chặt hơn trước: kiểm thêm lượt quiz `superseded` của seed không vào hàng chờ, lượt quiz của Phong rời hàng chờ khi Phong rời lớp, và `overdue=1&kind=quiz` trả đúng lượt của Hà (trước đây là rỗng).
  - Diff golden chỉ gồm các mục quiz mới của seed và tên học liệu `HTML CSS JS`.
- `internal/features/quizzes/scoring.go` có `//nolint:gosec` cho `rand.Shuffle`, theo quy ước `//nolint:gosec // lý do` sẵn có của repo. Thứ tự xáo được gửi cho học viên cùng lượt, không phải bí mật.

## Kiểm chứng

```bash
cd apps/api && set -a && . ./.env && set +a
make test                          # mọi package ok (-race, -tags integration)
make lint GOLANGCI_LINT="go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0"   # 0 issues
go test -race -count=3 -p 1 -tags integration ./internal/features/quizzes/... ./internal/features/grading/... ./internal/seed/...   # ok, không flaky
go list -deps ./internal/features/grading | grep quizzes   # rỗng
gofmt -l ./cmd ./internal          # rỗng
```

Các test đồng thời là tất định. Test giữ khóa `class_members` trong một transaction riêng, chờ đến khi đủ số request đứng chờ khóa (đọc `pg_stat_activity` với `wait_event_type = 'Lock'`), rồi mới thả khóa. Các case gồm: bắt đầu song song tạo đúng một lượt, nộp tay đua với `CloseExpired` chỉ chấm một lần, và lưu câu đua với nộp không đổi được câu của lượt đã chấm.

## Khác với plan

- DTO có thêm trường phân biệt `status` (`open`/`submitted`) và `mode`, để web chọn kiểu response mà không phải đoán.
- Kết quả luyện tập liệt kê câu theo thứ tự soạn, không theo thứ tự xáo của lượt. Lượt mở vẫn theo thứ tự đã chốt.
- Copy từ chối: thành viên đã rời lớp nhận 404 `NOT_FOUND` "Bạn không còn là thành viên đang học của lớp." (dùng lại `learning.ErrNotFound`). Lớp đã kết thúc nhận 409 "Lớp đã kết thúc, chỉ xem được học liệu.". Lớp nháp nhận 409 "Lớp chưa bắt đầu.". `blockedReason` của màn bắt đầu dùng nguyên copy prototype.
- Lượt mở của thành viên đã rời hay của lớp không còn `active` giữ nguyên trạng thái mở: không tự nộp, không vào hàng chờ. `CloseExpired` lọc các lượt này ngay trong SQL, nên chúng không chiếm chỗ trong lô 100.
- `app.NewQuizzesService` được export để `cmd/lms/worker.go` dùng chung cách nối phụ thuộc với API.
- `learning.LessonRow` thêm hai trường `QuestionCount` và `QuizMaxScore`.
- Test đồng thời dùng `sync.WaitGroup.Go` cộng chờ khóa tất định, thay cho `errgroup`. `golang.org/x/sync` hiện chỉ là phụ thuộc gián tiếp.
- Không thêm audit action nào cho bắt đầu, lưu, nộp hay tự nộp lượt. Team-lead đã xác nhận: FR-92 (spec-lms-v2.md dòng 366) không liệt kê các sự kiện này.
- Seed chỉ đặt `review_status` cho 10 lượt đã xác nhận; nhận xét và ghi chú (`quiz_attempt_reviews`) là việc của phase 10. Vì `grading.Recent` join bắt buộc với `quiz_attempt_reviews`, các lượt đã xác nhận của seed chưa hiện ở "gần đây" cho tới khi phase 10 seed nhận xét.
- Lượt đang làm dở của Bích có `first_opened_at = started_at`, khớp với hành vi của app (bắt đầu lượt thì ghi lần mở). `seed.js` lệch 0,04 ngày.

## Việc tiếp theo

- Phase 10 có thể dùng `quizzes` (khóa thành viên rồi khóa lượt, `review_status`) và phải seed `quiz_attempt_reviews` cho các lượt đã xác nhận.
- Phase 11 (web) dùng các golden trong `features/quizzes/testdata/` làm fixture.
- Phase 15 (cấp lượt): `Limit` đã cộng `quiz_attempt_grants`, nên chỉ còn thiếu API ghi.

## Câu hỏi chưa giải quyết

- Plan index (`plan.md`) vẫn ghi phase 09 là `pending`. Các phase trước cũng không tự cập nhật bảng này, nên tôi để team-lead cập nhật.
