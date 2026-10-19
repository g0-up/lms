---
phase: 7
title: "Hàng chờ chấm: backend"
status: completed
priority: P0
effort: "1d"
dependencies: [6]
---

# Phase 07: Hàng chờ chấm (backend)

## Mục tiêu

Một read model hợp nhất cho các mục chờ xử lý: bài homework `pending` và lượt quiz bài tập `review_status='pending'`. Giảng viên chỉ thấy mục của lớp mình phụ trách, Admin thấy mọi lớp. Mục cũ nhất đứng đầu. Lọc được theo loại, lớp và học liệu. Ở phase này nhánh quiz trả rỗng cho tới khi phase 09 tạo dữ liệu, nhưng SQL đã viết sẵn vì bảng có từ phase 01.

## Context

- Spec: FR-64 (và FR-65 cho cờ quá hạn, dùng sẵn ở P1).
- Prototype: `gradingQueue` 633-644 và `pageGrading` 1399-1431 ([scout-prototype](reports/scout-prototype.md) §5).

## Thiết kế

- Package `internal/features/grading`: `service.go`, `repository.go`, `repository_pg.go`, `dto.go`, `handler.go`. Chỉ đọc; không import `homework` hay `quizzes`, chỉ đọc bảng bằng SQL.
- `GET /grading/queue?kind=homework|quiz&classId=&lessonId=&overdue=1` (staff). SQL là một `UNION ALL`:
  - Nhánh homework: `submissions s WHERE s.status='pending'`.
  - Nhánh quiz: `quiz_attempts a WHERE a.review_status='pending'`.
  - Hai nhánh join `class_members`, `classes`, `users` và `lessons`, lấy chặng qua `stage_versions` và `stages`.
  - Cả hai nhánh lọc `cm.status = 'active'`. Học viên đã rời lớp không còn trong hàng chờ, số đếm và tồn đọng; bài cũ vẫn xem được qua link trực tiếp. Phase 14 dùng lại truy vấn này nên tự nhất quán.
  - Lọc `classes.teacher_id = $actor` khi actor là teacher.
  - Mỗi dòng có `{kind, itemId, submittedAt, no, isLate, autoSubmitted, scorePercent?, student{id,name}, class{id,code,name}, lesson{id,title}, stage{code,name}, waitedHours, overdue}`, với `overdue = submittedAt < now - 72h`.
  - Hằng `GradingSLA = 72 * time.Hour` khai báo một lần trong `grading`. Phase 14 dùng lại.
  - Sắp `ORDER BY submitted_at, item_id`.
- Response: `{items[], total, quizPending, overdueCount, filters{classes[], lessons[{id,title,kind}]}}`. Các option lọc chỉ lấy từ lớp mà actor được thấy.
- `GET /grading/recent?limit=6` (staff) gom lần chấm (`submission_grades`) và lần nhận xét quiz (`quiz_attempt_reviews`) mới nhất của các lớp actor thấy, sắp `created_at DESC`.
- `Service.Next(ctx, actor, after QueueKey)` trả mục kế tiếp trong hàng chờ, bỏ qua mục vừa xử lý. `homework` gọi hàm này qua interface `grading.NextFinder` để điền `nextQueueItem` sau khi chấm, và `quizzes` cũng dùng ở phase 10. Hướng phụ thuộc là `homework → grading`; `grading` không import ngược.
- Index: SQL dùng `ix_submissions_pending` và `ix_quiz_attempts_pending` đã tạo ở phase 01. Chạy `EXPLAIN` trên seed để kiểm.

## Files

- Tạo `internal/features/grading/*`, cùng test và `testdata/queue-*.json`.
- Sửa `internal/features/homework/service.go`: điền `nextQueueItem`.
- Sửa `internal/app/router.go`: thêm `newGrading(d)` và `Register(authed, requireStaff)`. `/grading/*` dùng header `noStore` như reports, vì dữ liệu cá nhân.
- Sửa `docs/architecture.md`: mục grading.

## Test

- Integration với seed của phase 06:
  - Teacher A không thấy mục của lớp teacher B; Admin thấy cả hai.
  - Thứ tự cũ nhất trước; `overdue` đúng ở biên 72 giờ (dùng `clock.Fake`).
  - Lọc `kind`, `classId`, `lessonId` và `overdue`.
  - Bản nộp `superseded` không có trong hàng chờ.
  - Bài của thành viên đã `dropped` không có trong hàng chờ và không tính vào `overdueCount`.
  - Chèn tay một lượt quiz `pending` bằng SQL fixture để khẳng định nhánh quiz chạy.
- Golden `queue-teacher.json` và `queue-admin.json`.

## Kiểm chứng

```bash
cd apps/api && make lint && TEST_DATABASE_URL=... make test
```

## Tiêu chí hoàn thành

- [x] FR-64 phía server có test, gồm cả thứ tự hợp nhất hai loại mục.
- [x] Response không chứa đáp án hay điểm từng câu; chỉ có % tổng của lượt quiz.
