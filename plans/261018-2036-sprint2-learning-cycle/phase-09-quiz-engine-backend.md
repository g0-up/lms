---
phase: 9
title: "Quiz engine: backend"
status: completed
priority: P0
effort: "3d"
dependencies: [4]
---

# Phase 09: Quiz engine (backend)

## Mục tiêu

Học viên bắt đầu lượt, lưu từng câu, nộp và xem kết quả của cả hai loại quiz. Server ghi giờ bắt đầu và giờ hết hạn, chấm toàn bộ ở server và tự nộp lượt quá giờ. Đáp án đúng không rời server trước khi nộp; với quiz bài tập thì không bao giờ rời server về phía học viên. Hoàn thành ghi vào `lesson_progress` theo luật của FR-70.

## Context

- Spec: FR-70 (bảng cấu hình), FR-71, FR-77 (phần server: thứ tự câu, tiếp tục lượt), OQ-3, OQ-4.
- Prototype: `startAttempt` 508-520, `saveAnswer` 522-526, `scoreAttempt` 528-532, `submitAttempt` 534-545, `quizView` 1605-1669 ([scout-prototype](reports/scout-prototype.md) §3).
- Bảng `quiz_attempts`, `quiz_attempt_answers`, `quiz_attempt_grants` của [phase 01](phase-01-database-migrations.md); `learning.CompletionWriter` của [phase 06](phase-06-homework-backend.md).

## Thiết kế

### Package `internal/features/quizzes`

Gồm `entity.go`, `scoring.go`, `service.go`, `repository.go`, `repository_pg.go`, `dto.go`, `handler.go` và test. Phụ thuộc qua interface hẹp, giống `homework`: `classes.MembershipReader`, `ClassReader`, `QuizReader` (đọc `QuizContent` đã phát hành của học liệu trong course version của lớp), `learning.CompletionWriter`, `clock.Clock`, `ids`, và `Shuffler` (bọc `math/rand/v2`; test dùng bản cố định).

### Domain

- `Attempt{ID, MemberID, LessonID, No, QuestionOrder []uuid.UUID, OptionOrder map[uuid.UUID][]uuid.UUID, StartedAt, DeadlineAt *time.Time, SubmittedAt *time.Time, AutoSubmitted, Score, MaxScore *int, Passed *bool, ReviewStatus *ReviewStatus}`.
- Hằng `SubmitGrace = 30 * time.Second`. `Attempt.PastGrace(now)` đúng khi có `DeadlineAt` và `now > DeadlineAt + SubmitGrace`.
- `Score(quiz, answers) (score, max int, perQuestion []QuestionResult)` là hàm thuần trong `scoring.go`. Câu được điểm khi tập phương án chọn bằng đúng tập đáp án đúng, không có điểm một phần. Câu bỏ trống được 0 điểm.
- `Passed = score*100 >= passPercent*max`, chỉ áp dụng cho assessment; practice để `nil`.
- `Limit = maxAttempts + sum(quiz_attempt_grants.extra_attempts)` cho assessment, `nil` (không giới hạn) cho practice. Bảng grants có từ phase 01, nên phase này tính luôn; API cấp lượt nằm ở phase 15.

### API học viên (`requireStudent`)

- `GET /me/classes/:id/lessons/:lid/attempts` (màn bắt đầu):
  - Trả `{mode, questionCount, maxScore, passPercent?, timeLimitMinutes?, limit?, used, granted, openAttemptId?, best{attemptId, percent, passed}?, attempts[{id, no, submittedAt, autoSubmitted, score, maxScore, percent, passed?, reviewStatus?}], canStart, blockedReason?}`.
  - `blockedReason` dùng copy prototype: "Lớp chưa bắt đầu", "Lớp đã kết thúc", "Bạn đã dùng hết N lượt".
  - Đã đạt không chặn làm thêm lượt, giống prototype.
- `POST /me/classes/:id/lessons/:lid/attempts` (bắt đầu hoặc tiếp tục). Trong một transaction:
  1. Kiểm thành viên và lớp `active` như phase 06.
  2. Khóa `class_members` `FOR UPDATE`, tuần tự hóa với lần bắt đầu song song.
  3. Nếu có lượt mở còn hạn thì trả lượt đó với `resumed: true` (200). Lượt mở đã quá ân hạn đã được đóng ở transaction trước (mục Tự nộp).
  4. Kiểm lượt còn lại. Hết lượt thì trả `INVALID_TRANSITION` "Bạn đã dùng hết N lượt làm bài.".
  5. Tạo lượt với `attempt_no = max+1` và `deadline_at = started_at + timeLimit` (assessment). Khi `shuffle` bật thì xáo `question_order` và từng `option_order`; khi tắt thì giữ thứ tự soạn.
  6. `uq_quiz_attempts_one_open` là lớp bảo vệ cuối ở database; `pgerr` map sang `CONFLICT` "Bạn đang có một lượt làm bài chưa nộp.".
  7. Ghi `lesson_progress.first_opened_at` nếu chưa có, như khi mở học liệu.
- `GET /me/attempts/:aid` (lượt của chính học viên):
  - Lượt mở: `{id, no, limit, deadlineAt, serverNow, savedAt, questions[{id, type, bodyHtml, points, options[{id, body}], selectedOptionIds}]}` theo `question_order` và `option_order`. **Không có** `isCorrect` hay `explanationHtml`. Lượt mở đã quá ân hạn được tự nộp ngay khi đọc rồi trả kết quả.
  - Kết quả practice: điểm, %, từng câu với `options[{id, body, isCorrect, selected}]`, `correct`, `pointsAwarded`, `explanationHtml`, và nhận xét cùng ghi chú từng câu của giảng viên (phase 10 điền).
  - Kết quả assessment: điểm, %, `passed`, `passPercent`, `autoSubmitted`, `reviewStatus`, `best{percent}`, `attemptsLeft`, và nhận xét cùng ghi chú từng câu kèm `bodyHtml` của câu. **Không có** phương án, đáp án hay đúng/sai từng câu (OQ-4).
  - Ba loại response dùng ba kiểu DTO riêng: `OpenAttemptDTO`, `PracticeResultDTO`, `AssessmentResultDTO`. `AssessmentResultDTO` không nhúng kiểu nào của staff hay practice, kể cả cho `review.notes`; mỗi ghi chú chỉ có `{questionId, position, bodyHtml, note}`.
- `PUT /me/attempts/:aid/answers/:qid {optionIds}`. Trong một transaction:
  1. Đọc `MemberContext` và yêu cầu `CanRecord` của `learning` (thành viên `active`, lớp `active`). Không đạt thì trả đúng copy từ chối của FR-71 như màn bắt đầu.
  2. Khóa `class_members`, rồi khóa lượt `FOR UPDATE`. Kiểm lại trong transaction: lượt của học viên, `submitted_at IS NULL`, chưa quá ân hạn. Ngược lại trả `INVALID_TRANSITION` "Lượt làm bài này đã được nộp." hoặc "Đã hết giờ làm bài; câu trả lời không được lưu.".
  3. Câu phải thuộc lượt. Mọi phương án phải thuộc câu. Câu `single` nhận tối đa 1 phương án. Mảng rỗng nghĩa là bỏ chọn.
  4. Upsert `quiz_attempt_answers`, rồi trả `{savedAt}`. Khóa lượt bảo đảm câu trả lời không bị ghi sau khi lượt đã được chấm.
- `POST /me/attempts/:aid/submit`:
  1. Đọc `MemberContext` và yêu cầu `CanRecord` như trên.
  2. Gọi `autoOrSubmit` (mục dưới) trong một transaction. Hàm trả một giá trị `Outcome{Submitted | AlreadySubmitted | Expired}` chứ không trả lỗi, để transaction commit được lần tự nộp.
  3. Sau khi commit: `AlreadySubmitted` trả `INVALID_TRANSITION` "Lượt làm bài này đã được nộp."; `Expired` trả `INVALID_TRANSITION` "Đã hết giờ làm bài; hệ thống đã tự nộp với các câu đã lưu." (hành vi "từ chối" của FR-71, nhưng các câu đã lưu vẫn được chấm); `Submitted` trả kết quả như `GET /me/attempts/:aid`.
- **Hàm dùng chung `autoOrSubmit(tx, attemptID, now)`**, dùng cho nộp tay, tự nộp khi đọc lười và `CloseExpired`:
  1. Đọc `class_member_id` của lượt không khóa, khóa `class_members` `FOR UPDATE`, rồi khóa lượt `FOR UPDATE`.
  2. `submitted_at` khác NULL thì trả `AlreadySubmitted` và không ghi gì.
  3. Quá ân hạn thì `submitted_at = deadline_at`, `auto_submitted = true`, kết quả `Expired`; ngược lại `submitted_at = now`, kết quả `Submitted`.
  4. Chấm bằng `Score`. Ghi `is_correct` và `points_awarded` cho mọi câu của lượt (chèn dòng rỗng cho câu bỏ trống, để thống kê FR-73 có mẫu số), rồi ghi `score`, `max_score`, `passed` bằng `UPDATE … WHERE id = $1 AND submitted_at IS NULL` với `ExecAffectOne`.
  5. Assessment: chuyển lượt `pending` cũ của cùng (thành viên, học liệu) sang `superseded`, rồi đặt lượt này `review_status = 'pending'`. Practice: `review_status` để NULL.
  6. Hoàn thành: practice gọi `MarkCompleted` ngay lần nộp đầu; assessment gọi khi `passed`. Không bao giờ xóa mốc hoàn thành, vì lượt tốt nhất được tính.
- **Thứ tự khóa** là `class_members` rồi `quiz_attempts`, ở mọi đường ghi: bắt đầu, lưu câu, nộp, tự nộp, xác nhận (phase 10) và cấp lượt (phase 15). Không đường nào khóa lượt trước thành viên.

### Tự nộp lượt quá giờ

- `quizzes.Service.CloseExpired(ctx, now)` chọn id của tối đa 100 lượt mở có `deadline_at < now - interval '30 seconds'` (không khóa; dùng `ix_quiz_attempts_open_deadline`). Mỗi id chạy `autoOrSubmit` trong transaction riêng. Lỗi của một lượt chỉ ghi log và không chặn các lượt khác; lượt đã được nộp bởi đường khác trả `AlreadySubmitted` và bị bỏ qua.
- Worker: `quizzes` hiện thực `mailer.Cleanup` và được thêm vào `Cleanups` trong `cmd/lms/worker.go`, chạy mỗi giờ như các cleanup hiện có. Không thêm cơ chế lịch mới.
- Đọc lười: `GET /me/attempts/:aid`, `GET …/attempts` và `POST …/attempts` tự nộp lượt quá hạn của chính học viên bằng một transaction `autoOrSubmit` riêng, commit xong mới đọc dữ liệu trả về. `POST …/attempts` đóng lượt hết giờ trong transaction riêng trước khi mở transaction bắt đầu lượt mới.
- Hệ quả: lượt bỏ dở không ai mở lại vào hàng chờ chậm tối đa một giờ. Chấp nhận được, vì hàng chờ tính hạn 72 giờ.

### Thay đổi ở `learning`

- `contentOf` cho quiz chỉ trả `{mode, questionCount, maxScore, passPercent?, timeLimitMinutes?, maxAttempts?}`, không có câu hỏi. Trang học liệu gọi API attempts để lấy phần còn lại.
- Test khẳng định JSON thô của `GET /me/classes/:id/lessons/:lid` (quiz) không chứa chuỗi `isCorrect` hay `correct`.

## Files

- Tạo `internal/features/quizzes/*`, cùng `testdata/attempt-open.json`, `attempt-result-practice.json`, `attempt-result-assessment.json`, `attempts-start.json`.
- Sửa `cmd/lms/worker.go`: thêm `quizzes` vào `Cleanups`.
- Sửa `internal/app/router.go`: tạo biến `requireStudent := identity.RequireRole(domain.RoleStudent)` (hiện `learning` gọi inline ở `router.go:72`; đổi chỗ đó dùng biến), rồi `newQuizzes(d).Register(authed, requireStudent)`.
- Sửa `internal/features/learning/service.go` (`contentOf`) và golden `stages/testdata/lesson_quiz.json`; golden học viên `learning/testdata/lesson-quiz-attempt.json`.
- Sửa `internal/seed/data.go`: lượt quiz theo `seed.js` (đã nộp, đang chờ xác nhận, đã thay thế), dùng giờ tương đối với `now` như seed hiện có.
- Sửa `docs/architecture.md`: mục quizzes, quy tắc lộ đáp án, tự nộp lượt quá hạn trong `Cleanups`.

## Test

- Unit `scoring_test.go`: single đúng hoặc sai; multi đúng đủ, thiếu một hoặc thừa một; bỏ trống; ngưỡng biên (70% với max 10 và 3 câu lẻ điểm).
- Integration:
  - Hai request bắt đầu song song (`errgroup`) tạo đúng một lượt mở. Request thứ hai nhận lượt đó với `resumed: true`.
  - Hết lượt thì bị chặn; thêm một dòng grant thì bắt đầu được.
  - Lưu câu sau `deadline + 30s` bị từ chối. Nộp sau ân hạn trả lỗi, và lượt **vẫn được tự nộp sau khi commit** với `submitted_at = deadline_at` (đọc lại từ DB để chứng minh không bị rollback).
  - `CloseExpired` với `clock.Fake` tự nộp, chấm các câu đã lưu và đặt `auto_submitted`. Một lượt lỗi không chặn các lượt còn lại.
  - Đua: lưu câu và nộp chạy song song (`errgroup`, `-race`): câu trả lời lưu sau không đổi được `selected_option_ids` của lượt đã chấm; nộp tay và `CloseExpired` song song chỉ chấm một lần; không deadlock.
  - Tiếp tục lượt (FR-71): bắt đầu, lưu hai câu, gọi lại `POST …/attempts` thì nhận cùng lượt với `resumed: true`, `question_order` và `option_order` giữ nguyên, hai câu đã lưu có trong `selectedOptionIds`.
  - Thành viên bị loại khỏi lớp, hoặc lớp chuyển `ended`, khi đang có lượt mở: lưu câu và nộp đều bị từ chối với copy FR-71; lượt không vào hàng chờ.
  - Lượt assessment thứ hai làm lượt `pending` thứ nhất thành `superseded`.
  - Practice nộp lần đầu thì `completed_at` có; assessment dưới ngưỡng thì không có, lượt sau đạt thì có, lượt sau nữa trượt thì vẫn giữ.
  - Học viên khác hoặc thành viên `dropped` gọi lượt của người khác nhận 404 hoặc 403. Lớp `ended` không bắt đầu được.
- JSON thô (FR-71, OQ-4): helper test `assertKeysAllowed(t, body, allow)` duyệt đệ quy JSON và lỗi khi gặp khóa ngoài allow-list. Áp dụng cho lượt mở và kết quả assessment, có cả ghi chú của giảng viên. Thêm denylist dự phòng trên body thô: `isCorrect`, `correct`, `explanation`, `pointsAwarded`, `selected`, `options` (assessment).

## Kiểm chứng

```bash
cd apps/api && make lint && TEST_DATABASE_URL=... make test
go test -race ./internal/features/quizzes/...
```

## Tiêu chí hoàn thành

- [x] FR-71 đủ: ghi giờ, không lộ đáp án, tự lưu, tiếp tục lượt, tự nộp, ân hạn 30 giây, giới hạn lượt, một lượt mở ở database, chỉ lớp `active`.
- [x] Bảng FR-70 "Tính vào hoàn thành", "Sau khi nộp học viên thấy" và "Điểm ghi nhận" có test.
- [x] Không có lộ trình nào gửi `isCorrect` cho học viên trước khi nộp, hoặc với quiz bài tập.

## Rủi ro

- Lệch giờ giữa máy học viên và server: trả `serverNow` để client tính đồng hồ theo độ lệch, và server luôn là bên quyết định.
- Worker dừng lâu: đọc lười vẫn đóng lượt của học viên khi họ quay lại. Lượt treo không ai mở chỉ ảnh hưởng hàng chờ, và sẽ được đóng khi worker chạy lại.
