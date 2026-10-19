---
phase: 15
title: "P1 quiz và khóa: cấp lượt, thống kê câu hỏi, tổng kết khóa"
status: completed
priority: P1
effort: "2d"
dependencies: [13]
---

# Phase 15: P1 quiz và khóa (cấp lượt, thống kê câu hỏi, tổng kết khóa)

## Mục tiêu

- FR-72: giảng viên phụ trách lớp, hoặc Admin, cấp thêm 1–3 lượt quiz bài tập cho một học viên. Lý do bắt buộc và được lưu vào lịch sử.
- FR-73: tab "Quiz" của lớp có tỉ lệ trả lời đúng từng câu, và đánh dấu câu có tỉ lệ đúng dưới 30%.
- FR-81: trang tổng kết của học viên có phần tổng kết toàn khóa: số chặng hoàn thành trên tổng số chặng, mục còn thiếu theo chặng, điểm quiz bài tập và homework của từng chặng. Không có điểm tổng toàn khóa.

## Context

- Spec FR-72, FR-73, FR-81, OQ-7 (đáp án sai khi lớp đang chạy chỉ xử lý bằng cấp thêm lượt).
- Prototype ([scout-prototype](reports/scout-prototype.md) §6):
  - `grantAttempts` 566-574, dialog 1833-1838, nút và lịch sử trong `renderQuizTab` 1297-1310.
  - `questionRates` 1289-1293, copy "Dưới 30%: xem lại câu hỏi".
  - `pageLearnSummary` 1567-1576.
- Code của phase 09 (`limit = maxAttempts + sum(grants)`), phase 10 (`GET /classes/:id/quizzes` có sẵn `grants: []`), phase 12 và 13 (`summary`).

## Thiết kế

### FR-72: cấp thêm lượt (package `quizzes`)

- `POST /classes/:id/members/:mid/grants {lessonId, extraAttempts, reason}` (staff phụ trách lớp). Trong một transaction:
  1. Kiểm quyền staff. Teacher không phụ trách nhận `FORBIDDEN` với copy "Bạn chỉ cấp thêm lượt cho lớp mình phụ trách.".
  2. Lớp phải `active`; nếu không, trả `INVALID_TRANSITION` "Lớp không ở trạng thái đang chạy".
  3. Thành viên phải thuộc lớp `:id` (truy vấn có `class_id = :id`; sai thì `NOT_FOUND` theo [plan.md](plan.md) §4 mục 8) và đang `active`. Học liệu phải thuộc course version của lớp, là quiz chế độ `assessment`; nếu không, trả `VALIDATION` "Chỉ cấp thêm lượt cho quiz bài tập.".
  4. `extraAttempts` là số nguyên 1–3 ("Số lượt cấp thêm từ 1 đến 3."). `reason` trim và khác rỗng ("Nhập lý do cấp thêm lượt. Lý do được lưu vào lịch sử."), tối đa 500 ký tự.
  5. Khóa `class_members` trước, theo thứ tự khóa chung ở [plan.md](plan.md) §4 mục 9 (phase 09 áp dụng cho mọi đường ghi quiz), để cấp lượt và bắt đầu hoặc nộp lượt không cắt ngang nhau.
  6. Chèn `quiz_attempt_grants`. Audit `quiz.attempts_granted` (nhãn "Cấp thêm lượt"), target là lớp, `after` có `memberId`, `studentName`, `lessonId`, `lessonTitle`, `extraAttempts`, `reason`.
- Học viên đã đạt vẫn cấp được qua API; UI ẩn nút như prototype. Spec không cấm, và giới hạn này chỉ là UX.
- `GET /classes/:id/quizzes` điền `grants[{id, memberId, studentName, extraAttempts, reason, grantedByName, grantedAt}]` (mới nhất trước) cho mỗi quiz. `members[].granted` có từ phase 10.
- Phía học viên: dòng "Số lượt" trên màn bắt đầu quiz thêm "· đã được cấp thêm" khi có grant. `GET …/attempts` của phase 09 thêm cờ `granted`.

### FR-73: thống kê câu hỏi

- `GET /classes/:id/quizzes` thêm `questionStats{attempts, rows[{questionId, position, bodyText, correctPercent}]}` cho mỗi quiz, gồm cả hai chế độ, như prototype.
  - Mẫu số: mọi lượt đã nộp (kể cả lượt bị thay thế) của thành viên `active`.
  - Một câu được tính đúng khi `pointsAwarded > 0`. Chấm theo kiểu tất cả hoặc không ở phase 09, nên điều này tương đương chọn đúng chính xác tập phương án đúng.
  - Không có lượt nào thì `correctPercent = null`.
  - Một truy vấn `GROUP BY question_id` trên `quiz_attempt_answers`. Phase 09 đã chèn dòng cho câu bỏ trống, nên mẫu số đúng.
- `bodyText` là bản văn bản thuần của câu hỏi, cắt ở 120 ký tự phía server, để không render HTML trong bảng.

### FR-81: tổng kết toàn khóa (package `summary`)

`GET /me/classes/:id/summary` của phase 12 thêm `course{stagesComplete, stagesTotal, stages[{stageVersionId, name, complete, missingCount, assessments[{lessonId, title, bestPercent?, passed}], homework[{lessonId, title, total?, max, status}]}]}`.

- Không có trường điểm tổng nào. Test hợp đồng kiểm rằng JSON không có `totalScore` hay `courseScore`.
- Dữ liệu dựng từ `StageSummary` sẵn có; không thêm truy vấn mới.

### Web

- `src/features/grading/components/quiz-tab.tsx`:
  - Nút "Cấp thêm lượt" trên dòng học viên của quiz bài tập khi học viên chưa đạt. Với lớp không `active`, nút có `aria-disabled` kèm lý do.
  - Thêm `grant-dialog.tsx`: tiêu đề "Cấp thêm lượt cho {tên}", dòng "{quiz} · đã dùng u/l lượt.", select 1–3, textarea lý do với help "Lưu vào lịch sử cấp lượt của lớp." và placeholder "vd: Mất kết nối khi đang làm bài". Thành công thì toast "Đã cấp thêm n lượt cho {tên}.".
  - Thêm `grant-history.tsx`: "Lịch sử cấp thêm lượt", mỗi dòng "+n lượt cho {tên}", lý do, người cấp, thời điểm.
  - Thêm `question-stats.tsx`: bảng câu và % đúng; câu dưới 30% có badge "Dưới 30%: xem lại câu hỏi" dạng chữ, không chỉ màu.
- `src/features/learning/components/quiz-start.tsx`: thêm "· đã được cấp thêm".
- `src/features/summary/pages/learn-summary-page.tsx`: thẻ "Toàn khóa" thêm "x/y chặng hoàn thành" và một bảng gọn: chặng, trạng thái, "còn n học liệu", điểm quiz bài tập ("Tốt nhất x% · Đạt|Chưa đạt"), homework ("a/b · Đạt|Cần làm lại|Chờ chấm|Chưa nộp").

## Files

- Sửa `internal/features/quizzes/`: thêm `grant.go`, `grant_service.go`; sửa `repository*.go`, `dto.go`, `handler.go`, golden `class-quizzes.json`.
- Sửa `internal/features/summary/`: `entity.go`, `service.go`, `dto.go`, golden `learner-summary.json`.
- Audit `quiz.attempts_granted`, đủ bốn chỗ theo [plan.md](plan.md) §4 mục 7: `internal/platform/audit/actions.go`, `actions_test.go`, `audit.go` `knownActions`, `internal/features/reports/model.go` (nhãn "Cấp thêm lượt", `activityPayload`, `summarize`) và `model_test.go`.
- Sửa `internal/app/router.go`.
- `seed.js` không có grant (`attemptGrants: []`), nên seed không thêm grant. Test tự tạo dữ liệu.
- Web: các file nêu ở mục Web, cùng `grading-api.ts`, `msw-handlers.ts`, schema và test component.

## Test

- Integration:
  - Cấp lượt cho học viên đã dùng hết lượt rồi bắt đầu lượt mới thành công; `limit` tăng đúng.
  - Từ chối khi học liệu là quiz luyện tập, `extraAttempts` = 0 hoặc 4, lý do rỗng, lớp `ended`, hoặc teacher khác lớp. Mỗi trường hợp có đúng copy.
  - Teacher dùng id lớp mình với id thành viên lớp khác nhận 404, không có grant nào được ghi.
  - Cấp lượt và bắt đầu lượt chạy song song không vượt giới hạn và không deadlock.
  - Audit có đủ trường trong `after`.
  - `questionStats` khớp golden. Câu chỉ toàn lượt bỏ trống có 0%. Quiz chưa có lượt có `null`.
  - FR-81: `course.stagesComplete` khớp số `StageSummary.Complete`; JSON không có trường điểm tổng.
- Web: dialog validate lý do trước khi gửi; badge dưới 30% có chữ; thẻ toàn khóa render đủ cột.
- `e2e/quiz.spec.ts` thêm bước: giảng viên cấp 1 lượt cho học viên đã hết lượt, và học viên thấy nút "Làm lượt mới".

## Kiểm chứng

```bash
cd apps/api && make lint && TEST_DATABASE_URL=... make test
pnpm -C apps/web lint typecheck test lint:design
E2E_SPECS='quiz|summary|a11y' pnpm -C apps/web test:e2e
```

## Tiêu chí hoàn thành

- [x] FR-72, FR-73 và FR-81 có test phía server và phía UI.
- [x] Action `quiz.attempts_granted` có trong `actions.go`, `knownActions`, test hợp đồng và bảng nhãn; ghi audit thật thành công trong test integration.
