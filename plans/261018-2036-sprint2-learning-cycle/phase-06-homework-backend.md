---
phase: 6
title: "Homework: hạn nộp, nộp bài, chấm (backend)"
status: completed
priority: P0
effort: "3d"
dependencies: [4]
---

# Phase 06: Homework (backend)

## Mục tiêu

- Giảng viên phụ trách hoặc Admin đặt hạn nộp theo lớp.
- Học viên đang học trong lớp `active` nộp repo và/hoặc file. Nộp lại thì thay lần nộp đang chờ.
- Giảng viên chấm lần nộp mới nhất theo tiêu chí và sửa điểm được, có lịch sử.
- Học viên nhận email kết quả và hoàn thành học liệu khi được chấm Đạt.

## Context

- Spec: FR-61, FR-62, FR-63, FR-92 (đổi hạn nộp, chấm điểm, sửa điểm); trường hợp biên "nộp lại khi đang chờ chấm", "nộp sau hạn", "giảng viên sửa điểm", "repo private", "học viên dropped".
- Quyết định: OQ-1 (repo phải public; server chỉ kiểm định dạng URL), OQ-2, OQ-6 và nhãn `superseded` (xem [plan](plan.md) §3).
- Prototype: `submitHomework` 577-593, `homeworkView` 1694-1722, `pageGradeSubmission` 1433-1467, grade handler 1850-1859 ([scout-prototype](reports/scout-prototype.md) §4, §5).
- Code mẫu: `classes/membership_reader.go`, quy tắc phụ trách lớp ở `reports/service.go:71` (`classHeader`), `learning/service.go` (`SetCompletion`), `mailer/{entity,templates}.go`.

## Thiết kế

### Package `internal/features/homework`

Gồm `entity.go`, `service.go`, `repository.go`, `repository_pg.go`, `dto.go`, `handler.go` và test, theo khuôn feature hiện có.

Phụ thuộc qua interface hẹp:

- `classes.MembershipReader`.
- `ClassReader`, đọc lớp, `teacher_id`, `status` và `course_version_id`.
- `LessonReader`, đọc học liệu homework cùng tiêu chí, ngưỡng và phiên bản chặng trong course version của lớp; dùng lại SQL của `learning.LessonInCourseVersion` nếu khớp.
- `learning.CompletionWriter` (mới, xem dưới), `mailer.Enqueuer`, `audit.Recorder`, `clock.Clock`, `ids`.

Quyền staff dùng hàm chung `staffCanAccessClass(actor, teacherID)`. Admin luôn được; teacher chỉ khi `teacherID == actor.ID`, ngược lại trả `classes.ErrNotOwnClass`. Đây là cùng quy tắc với `reports.classHeader` và không thêm biến thể mới.

### Hạn nộp (FR-61)

- `PUT /classes/:id/deadlines/:lid {dueAt}` và `DELETE /classes/:id/deadlines/:lid`, quyền staff phụ trách.
- Lớp phải `draft` hoặc `active`. Lớp `ended` trả `INVALID_TRANSITION` "Lớp đã kết thúc, không đổi hạn nộp được.".
- Học liệu phải là homework thuộc course version của lớp, nếu không trả `NOT_FOUND`.
- Upsert vào `class_lesson_deadlines`. Audit `class.homework_deadline_changed` theo [plan.md](plan.md) §4 mục 7: `before`/`after` là `{classCode, lessonId, lessonTitle, dueAt|null}` (không có học viên).
- `classes.ChangeCourseVersion` (chỉ cho lớp `draft`, `classes/entity.go:192-201`) xóa mọi `class_lesson_deadlines` của lớp trong cùng transaction, vì học liệu của phiên bản cũ không còn thuộc lớp. Test khẳng định điều này.

### Nộp bài (FR-62)

- Tải file:
  - Học viên gọi `POST /me/uploads {kind:"document", fileName, contentType, sizeBytes}` rồi `POST /me/uploads/:id/complete`.
  - Route: `media` thêm `RegisterStudent(rg)` và router gắn nó vào `authed.Group("/me/uploads", RateLimit(...))` với `requireStudent`. Nhóm `/media` hiện có giữ nguyên `requireAdmin` (`app/router.go:65`).
  - `InitUpload` và `CompleteUpload` hiện chặn mọi người không phải Admin (`media/service.go:74,98`). Đổi thành: Admin như cũ; học viên chỉ khi `kind = document`, là thành viên `active` của ít nhất một lớp `active` (đọc qua `classes.MembershipReader`), và có ít hơn 10 `document` chưa gắn bản nộp nào. Vượt giới hạn thì trả `INVALID_TRANSITION` "Bạn có quá nhiều file chưa nộp. Hãy nộp hoặc chờ trước khi tải thêm.".
  - `CompleteUpload` cho học viên kiểm `uploaded_by = actor`.
  - Giới hạn 20 MB. Header đã ký và allow-list theo đuôi file của `document` theo phase 04.
- Nộp: `POST /me/classes/:id/lessons/:lid/submissions {repoUrl?, commitRef?, mediaIds[]}`. Trong một transaction:
  1. Đọc `MemberContext`. Chặn khi không phải thành viên (`FORBIDDEN`), khi `dropped` (`FORBIDDEN` "Bạn không còn học lớp này.") và khi lớp không `active` (`INVALID_TRANSITION`, dùng "Lớp chưa bắt đầu." hoặc "Lớp đã kết thúc, không nộp bài được nữa.").
  2. Validate, dùng nguyên văn copy prototype:
     - Cần ít nhất repo hoặc một file.
     - `repoUrl` phải khớp `^https://(github\.com|gitlab\.com|bitbucket\.org)/[^/\s]+/[^/\s]+`.
     - `commitRef` (tối đa 100 ký tự, mẫu `^[A-Za-z0-9._/-]+$`) chỉ đi kèm repo.
     - Tối đa 5 file. Mỗi file phải là `document` `ready`, do chính học viên tải lên và chưa gắn bản nộp nào.
  3. `SELECT … FROM class_members WHERE id=$ FOR UPDATE` để tuần tự hóa với lần nộp khác và với việc chấm (chấm cũng khóa dòng này trước).
  4. Đọc lần nộp mới nhất **sau khi khóa**. Nếu là `passed`, trả `INVALID_TRANSITION` "Bài đã Đạt nên không nộp lại được.". Nếu là `pending`, chạy `UPDATE submissions SET status='superseded' WHERE id=$1 AND status='pending'` với `ExecAffectOne`; không đúng một dòng thì trả `CONFLICT` "Xung đột dữ liệu, vui lòng thử lại".
  5. Chèn bản nộp với `attempt_no = max+1` và `is_late = (deadline có và now > due_at)`, rồi chèn `submission_files`.
- Mở rộng `CanUserAccess` của media: người tải lên luôn xem được file của mình; giảng viên lớp có bản nộp chứa file xem được. Admin đã được bỏ qua kiểm tra.
- `GET /me/classes/:id/lessons/:lid/submissions` trả:
  - `deadline`.
  - `status` hiện hành, gồm `notSubmitted | pending | rework | passed`.
  - `canSubmit` kèm `lockedReason`.
  - `submissions[]` sắp mới nhất trước, mỗi dòng có `no`, `submittedAt`, `isLate`, `repoUrl`, `commitRef`, `files[{mediaId, fileName, sizeBytes}]`, `status` và `grade` hiện hành `{totalPoints, maxPoints, result, comment, gradedBy, gradedAt, scores[{criterionId, description, points, maxPoints, comment}]}`.

### Chấm (FR-63)

- `GET /submissions/:sid` (staff phụ trách) trả:
  - Bản nộp, học viên, lớp, chặng và học liệu (đề bài, ngưỡng, tiêu chí), hạn nộp.
  - `isLatest` và `latestSubmissionId`.
  - Toàn bộ bản nộp cùng (thành viên, học liệu), mỗi bản kèm **mọi** dòng `submission_grades` có điểm từng tiêu chí, tức lịch sử đầy đủ điểm cũ và mới.
- `POST /submissions/:sid/grades {scores:[{criterionId, points, comment?}], comment}`. Trong một transaction:
  1. Đọc bản nộp không khóa để lấy lớp và thành viên, kiểm quyền staff. Khóa `class_members` `FOR UPDATE`, rồi khóa bản nộp `FOR UPDATE` (thứ tự khóa ở [plan.md](plan.md) §4 mục 9).
  2. Sau khi khóa, bản nộp phải là mới nhất và không `superseded`. Nếu không, trả `INVALID_TRANSITION` "Chỉ chấm bài nộp mới nhất. Học viên đã nộp lần sau.". Lệnh đổi `submissions.status` có `WHERE id=$1 AND status <> 'superseded'` và dùng `ExecAffectOne`.
  3. Đủ điểm cho mọi tiêu chí của học liệu, mỗi điểm trong khoảng 0..max, `comment` khác rỗng. Lỗi trả `VALIDATION_FAILED` kèm `fields` như form hiện có.
  4. Tính `total`, `max` và `result` (Đạt khi `total*100 >= passPercent*max`). Chèn `submission_grades` và `grade_criterion_scores`, rồi đặt `submissions.status = result`.
  5. Hoàn thành: Đạt thì `CompletionWriter.MarkCompleted(member, lesson, gradedAt)`. Cần làm lại thì `ClearCompleted`.
  6. Audit: lần chấm đầu là `homework.graded`. Các lần sau là `homework.grade_updated`, có `before` (điểm cũ từng tiêu chí, tổng, kết quả) và `after`.
  7. Đưa email `homework_graded` vào outbox. Tiêu đề "Bài đã được chấm: {học liệu} · Đạt|Cần làm lại", hoặc "Điểm đã được cập nhật: …" khi sửa. Nội dung có tên lớp, tổng điểm, kết quả, nhận xét và link tới trang học liệu ở khóa payload `LessonURL` = `PublicBaseURL + "/learn/classes/{id}/lessons/{lid}"` ([plan.md](plan.md) §4 mục 5; khóa `Link` bị mailer từ chối).
  - Response gồm grade mới và `nextQueueItem` (để trống ở phase này; phase 07 điền).

### Ghi hoàn thành (dùng chung cho phase 09)

`learning.CompletionWriter`, cài trong `learning/repository_pg.go`:

- `MarkCompleted(ctx, ex, memberID, lessonID, at)` upsert `lesson_progress` với `first_opened_at = LEAST(existing, at)`, giữ `ck_lesson_progress_order`, và `completed_at = COALESCE(completed_at, at)`.
- `ClearCompleted` đặt `completed_at = NULL`.

`learning.SetCompletion` từ chối học liệu `quiz` hoặc `homework` bằng `VALIDATION_FAILED` "Học liệu này hoàn thành theo kết quả làm bài, không tích tay." (FR-91).

### Tổng quan homework của lớp

`GET /classes/:id/homework` (staff phụ trách) trả mỗi học liệu homework của course version một mục:

- `{lesson, stage, maxPoints, passPercent, deadline}`.
- `stats{submitted, pending, passed, rework, overdueNotSubmitted}`.
- `members[{memberId, name, status, latest{no, submittedAt, isLate, hasRepo, fileCount, totalPoints, maxPoints}, waitingDays}]`.

SQL dạng tập hợp, không N+1.

## Files

- Tạo `internal/features/homework/*`.
- Tạo `internal/features/mailer/templates/homework_graded.{html,txt}.tmpl`.
- Sửa `internal/features/mailer/entity.go` (`TemplateHomeworkGraded`), `templates.go` (subjects) và `templates_test.go`.
- Sửa `internal/features/media/`: `handler.go` (`RegisterStudent`), `service.go` (quyền học viên, giới hạn file chưa gắn), `repository_pg.go` (`CanUserAccess`, đếm file chưa gắn) và test.
- Sửa `internal/features/classes/`: `ChangeCourseVersion` xóa hạn nộp, cùng test.
- Sửa `internal/features/learning/`: `repository.go` và `repository_pg.go` (`CompletionWriter`), `service.go` (`SetCompletion` theo loại; `contentOf` cho homework trả đề bài, tiêu chí, ngưỡng và file kèm) cùng test.
- Audit, đủ bốn chỗ theo [plan.md](plan.md) §4 mục 7 cho `class.homework_deadline_changed`, `homework.graded`, `homework.grade_updated`:
  - `internal/platform/audit/actions.go`, `actions_test.go` (sửa comment "đúng 20 giá trị") và `audit.go` `knownActions`;
  - `internal/features/reports/model.go` (nhãn "Đặt hạn nộp", "Chấm bài", "Sửa điểm"; khóa `activityPayload`; nhánh `summarize`) và `model_test.go`.
- Sửa `internal/app/router.go`: tạo biến `requireStudent` nếu phase 09 chưa tạo (phase nào chạy trước thì tạo), thêm `newHomework(d, mail)` với `PublicBaseURL`, đăng ký với `requireStudent` và `requireStaff`; gắn `media.RegisterStudent` vào `/me/uploads`.
- Sửa `internal/seed/data.go`, theo prototype:
  - Hạn nộp: basic01 `ds-hw` đã quá hạn 20 ngày, `go-hw` còn 0.8 ngày; basic02 `ds-hw` còn 5 ngày.
  - Bản nộp đủ trạng thái `pending`, `passed`, `rework`, `superseded`, có một bản nộp muộn và một lần sửa điểm.
  - File mẫu nhỏ tải lên qua `storage.Uploader`.
- Sửa `docs/architecture.md`: feature homework, quyền và download attachment.

## Test

- Unit (`entity_test.go`):
  - Validate repo, commit và file.
  - Tính kết quả ở biên ngưỡng 70% (69.99 và 70).
  - Chuyển trạng thái nộp lại và chặn khi đã Đạt.
- Integration (`service_integration_test.go`):
  - Hai request nộp đồng thời tạo ra `attempt_no` 1 và 2, và chỉ một bản `pending`.
  - Chấm Đạt và nộp lại chạy song song (`errgroup`, `-race`): kết quả cuối hoặc là bài Đạt và lần nộp bị từ chối, hoặc là bản cũ `superseded` và lần chấm bị từ chối. Không bao giờ có bản `passed` bị đổi thành `superseded`.
  - Học viên `dropped` và lớp `ended` bị từ chối.
  - Teacher lớp khác gọi `GET /submissions/:sid` và `POST grades` nhận 403.
  - Hạn nộp (FR-61): teacher không phụ trách nhận 403; lớp `ended` nhận `INVALID_TRANSITION`; học liệu không phải homework của course version nhận 404; xóa hạn nộp thì `deadline = null`.
  - Nộp (FR-62): nộp sau hạn có `is_late = true`, nộp đúng `dueAt` thì `false`; lớp `draft` nhận "Lớp chưa bắt đầu."; file của học viên khác hoặc file đã gắn bản nộp khác bị từ chối.
  - Upload học viên: không phải thành viên `active` của lớp `active` nhận 403; file chưa gắn thứ 11 bị từ chối; `kind` khác `document` bị từ chối; Admin vẫn tải được qua `/media`.
  - Email `homework_graded` enqueue thành công (không bị chặn bởi khóa payload) và nội dung chứa `LessonURL`.
  - Chấm bản cũ bị từ chối.
  - Sửa điểm giữ lịch sử hai dòng, mỗi dòng có điểm từng tiêu chí.
  - Đạt thì đặt `completed_at`; sửa thành Cần làm lại thì xóa `completed_at`.
  - Outbox có `homework_graded`. Audit có đủ ba action.
  - `PUT completion` trên homework trả 422.
  - URL tải file submission có `attachment`. Học viên khác lớp không lấy được URL.
- Golden: `submission-detail.json`, `my-submissions.json`, `class-homework.json`.

## Kiểm chứng

```bash
cd apps/api && make lint && TEST_DATABASE_URL=... make test
```

## Tiêu chí hoàn thành

- [x] FR-61, FR-62, FR-63 (phía server) và FR-92 (ba hành động homework) có test.
- [x] `submissions` bất biến trừ `status`: không có lệnh UPDATE nào đổi cột khác. Kiểm bằng grep trong review.

## Rủi ro

- File tải lên nhưng không nộp thành rác trong storage. Giới hạn 10 file chưa gắn mỗi học viên chặn việc lạm dụng; việc dọn file mồ côi vẫn là nợ kỹ thuật, ngoài phạm vi.
- Học viên đổi repo sau khi nộp: đã có `commit_ref` để giảm rủi ro, như spec.
