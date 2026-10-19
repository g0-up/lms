---
phase: 10
title: "Xác nhận và nhận xét quiz: backend"
status: completed
priority: P0
effort: "1.5d"
dependencies: [7, 9]
---

# Phase 10: Xác nhận và nhận xét quiz (backend)

## Mục tiêu

Giảng viên phụ trách lớp, hoặc Admin, mở một lượt quiz đã nộp và xem từng câu theo thứ tự học viên thấy, kèm đáp án đúng. Với lượt quiz bài tập, giảng viên xác nhận kèm nhận xét chung bắt buộc và ghi chú từng câu không bắt buộc. Với lượt quiz học liệu, giảng viên nhận xét tùy ý. Sửa nhận xét giữ lịch sử. Học viên nhận email và thấy nhận xét trong trang kết quả mà không lộ đáp án quiz bài tập. Quiz tab của lớp có dữ liệu tổng quan.

## Context

- Spec: FR-75, FR-76, FR-92 (xác nhận quiz bài tập, nhận xét quiz học liệu, sửa nhận xét quiz).
- Prototype: `reviewAttempt` 548-564, `pageReviewAttempt` 1476-1521, `renderQuizTab` 1294-1316 ([scout-prototype](reports/scout-prototype.md) §5, §6).
- Bảng `quiz_attempt_reviews` và `quiz_review_notes` ([phase 01](phase-01-database-migrations.md)). `grading.NextFinder` ([phase 07](phase-07-grading-queue-backend.md)).

## Thiết kế

Code nằm trong package `quizzes` (`review.go`, `review_service.go`, cùng file repository và handler hiện có), vì vòng đời review gắn với lượt. Quyền staff dùng chung quy tắc `staffCanAccessClass` như phase 06.

### API (staff phụ trách lớp)

- `GET /quiz-attempts/:aid`:
  - Facts: học viên, lớp, chặng, học liệu, chế độ, `startedAt`, `submittedAt`, `autoSubmitted`, thời gian làm (phút) trên `timeLimitMinutes`, `score`, `maxScore`, `percent`, số câu đúng trên tổng, `passed`, `no` trên `limit`.
  - `questions[]` theo `question_order` và `option_order` của lượt. Mỗi câu có `bodyHtml`, `points`, `pointsAwarded`, trạng thái `correct | wrong | blank`, và `options[{id, body, isCorrect, selected}]`. Giảng viên được thấy đáp án.
  - `reviewStatus`, `isLatest`, `latestAttemptId`.
  - `reviews[]` (mới nhất trước), mỗi lần gồm `reviewer`, `createdAt`, `comment`, `notes[{questionId, note}]`. Dòng đầu là nhận xét hiện hành; các dòng sau là lịch sử.
  - `attempts[]`: các lượt đã nộp cùng (thành viên, học liệu), kèm `percent`, `reviewStatus` và nhận xét hiện hành.
  - Lượt chưa nộp trả `NOT_FOUND`, vì giảng viên không xem lượt đang làm.
- `POST /quiz-attempts/:aid/reviews {comment, notes:[{questionId, note}]}`. Trong một transaction:
  1. Đọc lượt không khóa để lấy lớp và thành viên, kiểm quyền staff. Teacher không phụ trách trả `FORBIDDEN` (`classes.ErrNotOwnClass`). Khóa `class_members` rồi khóa lượt `FOR UPDATE` ([plan.md](plan.md) §4 mục 9).
  2. `comment` phải khác rỗng; nếu không trả "Nhập nhận xét chung cho học viên.". Mỗi `questionId` phải thuộc lượt; ghi chú rỗng được bỏ qua.
  3. Assessment `superseded` trả `INVALID_TRANSITION` "Học viên đã nộp lượt mới hơn. Hãy xác nhận lượt mới nhất.".
  4. Chèn `quiz_attempt_reviews` và `quiz_review_notes`. Bảng append-only: dòng mới nhất là hiện hành, dòng cũ là lịch sử (gồm người, thời điểm, nội dung).
  5. Đặt `review_status`:
     - Assessment `pending` chuyển sang `confirmed`.
     - Practice chuyển từ NULL sang `confirmed`, nghĩa là "Đã nhận xét".
     - Lượt đã `confirmed` giữ nguyên.
     - Điểm, `passed` và hoàn thành không đổi (FR-75: "Xác nhận không đổi điểm").
  6. Audit, mỗi lần một dòng:
     - Lần đầu của lượt assessment: `quiz_attempt.confirmed`.
     - Lần đầu của lượt practice: `quiz_attempt.commented`.
     - Các lần sau của cả hai loại: `quiz_attempt.review_updated`, kèm `before` là nhận xét và ghi chú trước đó.
  7. Đưa email `quiz_reviewed` vào outbox. Tiêu đề lấy từ prototype:
     - "Giảng viên đã xác nhận kết quả: {quiz} lượt #n · x% · Đạt|Chưa đạt"
     - "Giảng viên đã nhận xét: …"
     - "Nhận xét đã được cập nhật: …"
     Nội dung có link ở khóa payload `ResultURL` = `PublicBaseURL + "/learn/classes/{id}/lessons/{lid}?result=<attemptId>"` (không dùng khóa `Link`, [plan.md](plan.md) §4 mục 5). Email quiz bài tập không chứa đáp án: payload chỉ có tên quiz, số lượt, %, kết quả và nhận xét chung; không có nội dung câu, phương án hay giải thích.
  8. Response gồm review mới và `nextQueueItem`. Chỉ lần xác nhận đầu của lượt assessment mới điền `nextQueueItem` qua `grading.NextFinder`; practice và lần sửa trả null.
- `GET /classes/:id/quizzes` (staff phụ trách), mỗi quiz của course version một mục:
  - `{lesson, stage, mode, maxAttempts?, timeLimitMinutes?, passPercent?, questionCount}`.
  - `stats{attempted, members, passed, pending, avgBestPercent, submittedAttempts}`.
  - `members[{memberId, name, used, limit, granted, bestPercent, status, latest{attemptId, no, percent, submittedAt, reviewStatus}}]`.
  - `grants[]` để rỗng ở phase này; phase 15 điền.
  - `questionStats`: phase 15 thêm vào cùng response.
  - SQL dạng tập hợp, không N+1.

### Phía học viên

`GET /me/attempts/:aid` của phase 09 thêm `review{reviewerName, createdAt, comment, notes[{questionId, questionBodyHtml, note}], history?}`. Nhận xét hiện hành là dòng mới nhất. Với assessment, `notes` chỉ có nội dung câu hỏi và ghi chú, không có phương án. Kiểu DTO học viên tách khỏi DTO staff của `GET /quiz-attempts/:aid`; không tái dùng struct câu hỏi có `pointsAwarded`, `correct` hay `options`.

### Mail và audit

- `mailer/entity.go`: thêm `TemplateQuizReviewed` (đã có trong CHECK từ phase 01). Thêm `templates/quiz_reviewed.{html,txt}.tmpl` và chuỗi tiêu đề.
- Audit cho ba action trên, đủ bốn chỗ theo [plan.md](plan.md) §4 mục 7: `audit/actions.go`, `actions_test.go`, `audit.go` `knownActions`, và `reports/model.go` (nhãn "Xác nhận quiz", "Nhận xét quiz", "Sửa nhận xét quiz"; `activityPayload`; `summarize`) cùng `model_test.go`. `after` có `classCode`, `lessonId`, `lessonTitle`, `memberId`, `studentName`, `attemptNo`, `comment`.

## Files

- Sửa `internal/features/quizzes/`: thêm `review.go`, `review_service.go`; sửa `repository*.go`, `dto.go`, `handler.go` (`Register` nhận thêm nhóm staff).
- Tạo `internal/features/mailer/templates/quiz_reviewed.{html,txt}.tmpl`; sửa `mailer/entity.go`, `templates.go`, `templates_test.go`.
- Sửa `internal/platform/audit/actions.go`, `actions_test.go`, `audit.go`, `internal/features/reports/model.go` và `model_test.go`.
- Sửa `internal/app/router.go`: truyền `PublicBaseURL` vào `quizzes`.
- Sửa `internal/seed/data.go`: review theo `seed.js` 560-589 (nhận xét đã xác nhận, ghi chú từng câu, reviewer `u-gv2` cho qa19).
- Thêm golden: `testdata/review-assessment.json`, `review-practice.json`, `class-quizzes.json`, `attempt-result-assessment-reviewed.json`.

## Test

- Teacher B không mở hay xác nhận được lượt của lớp teacher A (403); Admin được.
- Xác nhận lượt `pending` thì lượt rời hàng chờ (gọi `grading` queue trong test) và `nextQueueItem` trỏ đúng mục kế tiếp.
- Xác nhận lượt `superseded` bị từ chối với đúng copy.
- Sửa nhận xét: có hai dòng review, audit `quiz_attempt.review_updated` có `before`, và outbox có email "Nhận xét đã được cập nhật".
- Practice nhận xét được, nhưng không bao giờ có trong hàng chờ.
- Điểm, `passed` và `completed_at` không đổi sau khi xác nhận.
- JSON thô kết quả assessment phía học viên, sau khi có ghi chú từng câu, qua `assertKeysAllowed` của phase 09 và denylist dự phòng (`isCorrect`, `correct`, `explanation`, `pointsAwarded`, `selected`, `options`).
- Render email `quiz_reviewed` cho lượt assessment có ghi chú từng câu: HTML và text không chứa nội dung phương án hay giải thích của câu nào. Enqueue thành công (không bị chặn bởi khóa payload).
- Teacher dùng id lượt của lớp khác trên `GET /quiz-attempts/:aid` nhận 403 (lượt tự xác định lớp; không có `:id` trong route).
- Xác nhận và nộp lượt mới chạy song song không deadlock và không xác nhận lượt đã `superseded`.
- Golden `class-quizzes.json` khớp seed.

## Kiểm chứng

```bash
cd apps/api && make lint && TEST_DATABASE_URL=... make test
```

## Tiêu chí hoàn thành

- [x] FR-75 và FR-76 phía server có test, gồm vòng đời Chờ xác nhận, Đã xác nhận, Đã có lượt mới hơn và lịch sử sửa nhận xét.
- [x] FR-92: ba action quiz có trong `actions.go` và trong test hợp đồng.
- [x] Email `quiz_reviewed` render được, có test template.
