# Báo cáo thực hiện: xác nhận và nhận xét quiz (backend)

- Phase: `phase-10-quiz-review-backend.md` (FR-75, FR-76)
- Ngày: 2026-10-19
- Trạng thái: xong. Suite đầy đủ (kèm integration) và lint xanh. Có vài điểm lệch plan cần team-lead xem (cuối báo cáo). Không commit.

## Kết quả

`ReviewService` mới trong `internal/features/quizzes` cho staff (admin và giảng viên) có ba route:

- `GET /api/v1/quiz-attempts/{aid}`: trang xem lượt đã nộp. Trả câu theo thứ tự học viên thấy, phương án kèm đúng/đã chọn và trạng thái từng câu (`correct`, `wrong`, `blank`), thời gian làm, các lần nhận xét (mới nhất trước, kèm ghi chú từng câu) và lịch sử lượt của học viên với quiz đó (`isLatest`).
- `POST /api/v1/quiz-attempts/{aid}/reviews` `{comment, notes:[{questionId, note}]}`: trả 201 `{review, reviewStatus, updated, nextQueueItem}`.
- `GET /api/v1/classes/{id}/quizzes`: tab Quiz của lớp. Trả `{class, items}`; mỗi quiz có thống kê (số thành viên, đã làm, đạt, điểm trung bình, chờ xác nhận) và trạng thái từng thành viên đang học.

Quy tắc:

- Admin thấy mọi lớp. Giảng viên chỉ thấy lớp mình phụ trách; lớp được xác định từ lượt, không tin vào client. Lớp khác trả 403, học viên 403, chưa đăng nhập 401. Lượt đang làm, không tồn tại hay id sai dạng trả 404 "Không tìm thấy lượt làm bài đã nộp.".
- Nhận xét chung bắt buộc, tối đa 5000 ký tự tính theo rune. Ghi chú từng câu tối đa 1000 ký tự, phải thuộc lượt và không trùng câu. Ghi chú rỗng bị bỏ. Lỗi trả 422 `VALIDATION_FAILED`; JSON hỏng trả 400.
- Lượt `superseded` trả 409 `INVALID_TRANSITION` "Học viên đã nộp lượt mới hơn. Hãy xác nhận lượt mới nhất.".
- Lưu nhận xét chỉ thêm, không sửa: mỗi lần lưu chèn một dòng `quiz_attempt_reviews` kèm `quiz_review_notes`. Lần đầu đổi `review_status` thành `confirmed`: lượt đánh giá từ `pending`, lượt luyện tập từ NULL. Không đổi `score`, `passed`, `submitted_at` hay `completed_at`.
- Mọi bước chạy trong một transaction, khóa `class_members` rồi đến lượt (cùng thứ tự với nộp bài):
  - Audit `quiz_attempt.confirmed` (đánh giá) hoặc `quiz_attempt.commented` (luyện tập). Các lần sau ghi `quiz_attempt.review_updated` kèm `before`. Mọi audit có `request_id`.
  - Email `quiz_reviewed` gửi học viên. Tiêu đề "[GoUp LMS] Giảng viên đã xác nhận kết quả: …" hoặc "… đã nhận xét: …"; khi sửa thì thêm "Nhận xét đã được cập nhật". Link nằm ở `ResultURL`. Payload không có khóa Link, Token hay TempPassword, cũng không có nội dung câu, phương án, giải thích hay ghi chú.
- `nextQueueItem` chỉ có ở lần xác nhận đầu của lượt đánh giá, lấy qua `grading.NextFinder`. Luyện tập không bao giờ vào hàng chờ.
- `grading` vẫn không import `homework`, `quizzes` hay `reports`; đã kiểm bằng `go list -deps`.
- Seed: lượt đã xác nhận có nhận xét thật, kèm ghi chú từng câu, cùng audit tương ứng. Seed không tạo email outbox.

## File đã sửa

Mới:

- `apps/api/internal/features/quizzes/review.go`: hàm thuần.
  - `reviewContent`, `nextReviewStatus`, `reviewAction`, `staffCanAccessClass`.
  - `questionStatus`, `minutesTaken`, `memberQuizStatus`, `avgPercent`, `shortTitle`.
  - Các lỗi kèm copy.
- `apps/api/internal/features/quizzes/review_service.go`: `ReviewService` gồm `AttemptReview`, `Review` và `ClassQuizzes`.
- `apps/api/internal/features/quizzes/review_test.go`: unit test cho các hàm thuần.
- `apps/api/internal/features/quizzes/review_integration_test.go`: các test lifecycle, practice, access, validation và race (chi tiết ở phần Kiểm chứng).
- `apps/api/internal/features/quizzes/seed_integration_test.go` và `export_integration_test.go`: test `class-quizzes` trên dữ liệu `lms seed`.
- `apps/api/internal/features/quizzes/testdata/`: `review-assessment.json`, `review-practice.json`, `class-quizzes.json`, `attempt-result-assessment-reviewed.json`.
- `apps/api/internal/features/mailer/templates/quiz_reviewed.{txt,html}.tmpl`.

Sửa:

- `quizzes/{entity,dto,handler,repository,repository_pg}.go`: DTO staff, handler, ghi review và note, các truy vấn đọc lượt, lịch sử và thành viên.
- `app/deps.go`, `app/router.go`, `app/router_test.go`: nối `ReviewService` với `grading.NextFinder` và mailer, rồi đăng ký route staff.
- `platform/audit/actions.go`, `actions_test.go`, `audit.go`: ba action quiz.
- `features/reports/model.go`, `model_test.go`: nhãn hoạt động "Xác nhận quiz", "Nhận xét quiz", "Sửa nhận xét quiz".
- `features/mailer/entity.go`, `templates.go`, `templates_test.go`: template `quiz_reviewed` và `TestRenderQuizReviewed`.
- `seed/data.go`, `seed.go`, `seed_test.go`, `seed_integration_test.go`: review, note và audit của seed. Whitelist action trong test bất biến audit có thêm hai action quiz.
- Ngoài danh sách File của phase, sửa vì seed đổi:
  - `grading/service_integration_test.go` (`TestRecentGradedAndConfirmed`) và `grading/testdata/queue-recent.json`.
  - `reports/testdata/dashboard.json`: diff chỉ thêm hai mục hoạt động của seed.
- `docs/architecture.md`, mục Quiz: route staff, quyền, giới hạn, audit, email; ghi chú golden dựng trên seed.
- Phase file: `status: completed` và tick ba tiêu chí.

## Kiểm chứng

```
cd apps/api
make test                       # TEST_DATABASE_URL trong .env, -tags integration: mọi package ok
go test -tags integration -count=2 ./internal/features/quizzes/   # ổn định, ok
make lint GOLANGCI_LINT="go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0"   # 0 issues
gofmt -l ./internal ./cmd       # rỗng
go list -deps ./internal/features/grading | grep -E "features/(homework|quizzes|reports)"   # không có dòng nào
```

`make lint` với `golangci-lint` cài sẵn trong `~/go/bin` lỗi vì binary build bằng go1.26, thấp hơn go1.27 mà module nhắm. Đây là lỗi môi trường, không phải lỗi code; chạy bằng v2.14.0 như phase 09 thì sạch.

Test chính:

- `TestAssessmentReviewLifecycle`:
  - Lượt bị thay thế trả 409.
  - Xác nhận: `nextQueueItem` là lượt kế tiếp trong hàng chờ, lượt rời hàng chờ, điểm và các mốc giờ giữ nguyên.
  - Audit `confirmed` có `before` NULL và `request_id`. Tiêu đề email khớp chính xác; kiểm `ResultURL`; email không lộ nội dung quiz.
  - Sửa nhận xét: `updated: true`, không có `next`, có hai dòng review, audit `review_updated` có `before`.
  - Golden cho trang staff và kết quả của học viên (allow-list khóa, không lộ đáp án, có ghi chú).
- `TestPracticeReviewIsNeverQueued`: audit `commented`, không vào hàng chờ, có golden `review-practice.json`; lần sửa ghi `review_updated`.
- `TestReviewAccess`: giảng viên lớp khác, học viên và người chưa đăng nhập; 404 cho lượt mở, id ngẫu nhiên và id sai dạng; các đường lỗi không ghi gì. Admin xác nhận được, rồi giảng viên sửa được.
- `TestReviewValidation`: mọi copy 422, JSON hỏng trả 400; không ghi gì.
- `TestReviewRacesSubmit`: chạy cả hai thứ tự với waiter khóa FIFO, kết quả xác định:
  - Xác nhận trước: `[confirmed, pending]`.
  - Nộp trước: 409 và `[superseded, pending]`; không có review và không có mail.
- `TestClassQuizzesMatchesSeed`: golden trên seed, số liệu khớp DB, kiểm 403 giữa các lớp.
- `TestRenderQuizReviewed` và unit test trong `review_test.go`.

## Khác với plan

1. POST của giảng viên lớp khác trả 403 với copy `ErrNotOwnReview` "Bạn chỉ nhận xét bài của lớp mình phụ trách." (theo prototype). GET lượt và tab Quiz dùng `classes.ErrNotOwnClass`.
2. DTO kết quả của học viên giữ tên trường của phase 09. `position` của ghi chú là vị trí câu trong quiz, giống DTO phase 09, không phải thứ tự đã xáo.
3. Mã trạng thái thành viên trong tab Quiz do tôi đặt, theo `lessonStatus` của prototype: `todo`, `inProgress`, `done` (luyện tập), `passed`, `failed`, `outOfAttempts`.
4. `ReviewService` tách riêng khỏi `Service` của học viên để giữ ranh giới quyền staff và học viên.
5. Response `ClassQuizzes` là `{class, items}`. Response POST có thêm `reviewStatus`, `updated` và id người nhận xét.
6. Lượt luyện tập sau nhận xét đầu có `review_status = confirmed` để phân biệt "đã nhận xét"; nó không vào hàng chờ vì hàng chờ chỉ đọc `pending`.
7. Seed không tạo email `quiz_reviewed` trong outbox, để không gửi mail khi chạy seed. Cờ `confirmed` của seed được thay bằng struct `review` có nhận xét và ghi chú.
8. Có sửa file ngoài danh sách của phase: test và golden của grading, golden dashboard của reports, test seed. Lý do là seed có thêm review thật nên các golden đó đổi theo. Đã đọc diff: chỉ thêm mục mới, không đổi dữ liệu cũ.

## Việc tiếp theo

- Phase 11 (Quiz web) dùng được ba route staff và kết quả của học viên có nhận xét.
- Phase 12 và 14 dùng được ba action audit và trạng thái `confirmed`.

## Câu hỏi chưa giải quyết

1. Web nên hiển thị `position` của ghi chú theo vị trí trong quiz (như hiện tại) hay theo thứ tự học viên thấy? Đã chốt: theo thứ tự học viên thấy. E2E cho thấy giảng viên ghi chú "câu 1" trên trang staff thì học viên lại thấy "Câu 5.", nên `toAssessmentResultDTO` đổi sang đánh số theo `question_order`, có cập nhật test và golden.
2. Phase 11 có chấp nhận các mã trạng thái thành viên ở mục 3 không, hay cần theo một danh sách khác?
3. Tách copy 403 (POST dùng copy nhận xét, GET dùng copy lớp) có đúng ý không, hay nên dùng một copy?
4. Các file ngoài danh sách (grading, reports, test seed) đã sửa vì seed đổi. Team-lead xác nhận việc này không đụng phase khác đang chạy.
