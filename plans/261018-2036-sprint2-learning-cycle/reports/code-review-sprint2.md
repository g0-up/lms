# Code review Sprint 2: chu trình học (bản chưa commit)

Ngày: 2026-10-19. Reviewer: code-reviewer. Chế độ: chỉ đọc, không sửa code.

## Phạm vi

- `git diff HEAD` cộng file chưa track, khoảng 280 thay đổi trong `apps/api` (migrations 0010–0014; features `stages`, `learning`, `media`, `homework`, `quizzes`, `grading`, `summary`, `reports`, `classes`, `mailer`; `platform/audit`; `app`; `cmd/lms/worker.go`) và `apps/web/src` (features `learning`, `grading`, `summary`, `stages`, `dashboard`, `reports`; `shared`).
- Đã chạy: `go build ./...` và `go vet ./...` đều sạch. Đã chạy `go list -deps` để kiểm ranh giới import.
- Chưa chạy: test Go (integration cần dev DB, bị cấm theo yêu cầu), web typecheck, lint và vitest (một agent khác đang sửa web, seed và e2e). Không có số liệu coverage.

## Kết luận chung

Backend không có lỗi Critical. Các bất biến của plan §4 đều giữ được, có bằng chứng ở code:

- Thứ tự khóa.
- `completed_at` ghi cùng transaction.
- Trả 404 khi id con thuộc lớp khác.
- Không lộ đáp án quiz đánh giá.
- Payload email không có khóa bí mật.
- Action audit được đăng ký đủ 4 nơi.
- Ranh giới import.

Có 1 lỗi High: học viên có thể bị khóa upload vĩnh viễn. Lỗi này nằm ở chỗ giao nhau giữa giới hạn 10 file và quyết định bỏ job dọn file mồ côi. Có 1 lỗi Medium về cache web và vài lỗi Low về hiệu năng truy vấn khi dữ liệu lớn.

## Critical

Không có.

## High

### H1. Học viên bị khóa upload vĩnh viễn sau 10 file không được nộp

**Vị trí:**
- `apps/api/internal/features/media/repository_pg.go:108-125` (`StudentUploadQuota`).
- `apps/api/internal/features/media/service.go:31,37,120-137`.
- `apps/web/src/features/learning/hooks/use-submit-homework.ts:41-46,81-83`.

**Bằng chứng:**
- Hạn mức đếm mọi `media_files` có `uploaded_by = học viên` và `kind = 'document'` mà chưa có dòng `submission_files`. Phép đếm không lọc theo `status`: file `pending` bị bỏ dở và file `failed` đều bị tính. Phép đếm cũng không lọc theo thời gian.
- Không có đường nào làm giảm con số này ngoài việc nộp:
  - Không có endpoint xóa file của học viên.
  - Grep toàn `apps/api` không thấy job dọn nào ngoài `media`.
  - `Cleanups` trong `cmd/lms/worker.go` chỉ có identity, quizzes và grading.
- `UsableFiles` chỉ nhận file `ready`, nên file `pending` hay `failed` không bao giờ được gắn vào bản nộp. Chúng chiếm chỗ mãi mãi.
- Web chỉ dùng lại file đã upload trong cùng một phiên form, qua `WeakMap<File, string>`. Các trường hợp sau đều để lại file chưa gắn:
  - Rời trang khi đang upload: `abort()` bỏ lại bản ghi `pending`.
  - Đổi file trong form.
  - Lỗi `mediaIds`: map bị xóa nên lần sau upload lại file mới.
  - Submit bị 409, ví dụ lớp vừa kết thúc.
  - Mở lại trang.

**Kịch bản lỗi:** Học viên chọn 5 file rồi rời trang trước khi bấm nộp, và lặp lại một lần nữa. Lúc đó đã có 10 file chưa gắn. Từ lần sau, mọi `POST /me/uploads` đều trả 409 với câu "Bạn có quá nhiều file chưa nộp. Hãy nộp hoặc chờ trước khi tải thêm." Học viên không thể "nộp" vì nộp cần upload, và cũng không thể "chờ" vì không có cơ chế nào nhả hạn mức. Học viên bị khóa nộp mọi homework có file, ở mọi lớp, cho đến khi có người sửa DB tay. Dịch vụ của học viên đó bị chặn vĩnh viễn, chỉ từ thao tác bình thường.

**Liên quan quyết định đã chốt:** Plan (`plan.md:167`, `reports/red-team-review.md:43`, `phase-06-homework-backend.md:163`) chấp nhận file mồ côi là "rác storage, nợ kỹ thuật". Quyết định đó đánh giá chi phí lưu trữ. Nó không xét việc giới hạn 10 file biến rác thành khóa tài khoản vĩnh viễn. Đây là bằng chứng mới, không phải mối lo trừu tượng, nhưng vì chạm vào quyết định đã chốt nên lead cần chọn hướng:

- **(a) Rẻ nhất, không cần job:** chỉ đếm file còn có thể dùng. Đó là `status = 'ready'` cộng `status = 'pending' AND created_at > now() - UploadTTL`. Cần kiểm tên cột thật của `media_files`.
  - Hệ quả: file `pending` hết hạn và file `failed` thôi chiếm chỗ.
  - Hạn chế: 10 file `ready` chưa nộp vẫn khóa người dùng.
- **(b) Thêm cửa sổ thời gian cho file `ready`:** chỉ đếm file `ready` chưa gắn trong N giờ gần nhất, ví dụ 24 giờ. Câu "hoặc chờ" sẽ đúng nghĩa. Mục đích chống lạm dụng storage vẫn giữ: tối đa khoảng 10 file mỗi ngày mỗi học viên.
- **(c) Thêm `DELETE /me/uploads/:id`** cho file chưa gắn. Cần thêm UI.

Khuyến nghị: làm (a) + (b) trong `StudentUploadQuota`, kèm test `TestStudentUnattachedLimit…` cho file `pending` hết TTL và file `ready` cũ hơn N giờ. Lỗi `ErrTooManyUnattached` giữ nguyên.

## Medium

### M1. Nộp homework không làm mới lộ trình, danh sách bên và tổng kết của học viên

**Vị trí:** `apps/web/src/features/learning/hooks/use-submit-homework.ts:75-80`.

**Bằng chứng:**
- `onSuccess` chỉ `setQueryData(learningKeys.submissions(...))`.
- Server có trả trạng thái homework trên lộ trình (`learning/roadmap.go:171` gắn `ItemStatus` cho quiz và homework), và web hiển thị nó ở `components/lesson-side-list.tsx:41-46` và `components/lesson-row.tsx:42-44`, lấy từ `learningKeys.myClass`.
- Đường quiz thì có làm mới: `invalidateAfterSubmit` trong `use-quiz-attempt.ts` invalidate `myClass`, `lesson` và `myClasses`.
- `staleTime` mặc định là 30 giây (`app/query-client.ts:64`).

**Kịch bản lỗi:** Học viên nộp bài. Khung chính hiện "Chờ chấm", nhưng danh sách bên trái trên cùng trang vẫn hiện "Chưa nộp" hoặc "Làm lại". Lý do là `myClass` không bị invalidate nên không refetch, cho đến khi cửa sổ lấy lại focus hoặc người dùng sang trang khác sau 30 giây. Nếu quay về lộ trình hoặc trang tổng kết trong vòng 30 giây, trạng thái vẫn cũ. Khi nộp lại sau "Làm lại", học viên thấy hai trạng thái mâu thuẫn trên cùng một màn hình.

**Cách sửa:** Trong `onSuccess`, invalidate thêm `learningKeys.myClass(classId)` và `learningKeys.myClasses`, giống `invalidateAfterSubmit`. Nếu trang tổng kết học viên dùng cache riêng thì invalidate thêm `summaryKeys.learner(classId)`. Thêm assertion vào `pages/homework-lesson.test.tsx`.

## Low

### L1. Thiếu index trên cột FK `ON DELETE RESTRICT` mà các thao tác soạn draft xóa vào

**Vị trí:**
- `migrations/0013_quiz_attempts.up.sql:34`: `quiz_attempt_answers.question_id`. PK là `(attempt_id, question_id)`, cột đầu là `attempt_id` nên không dùng được cho tra cứu theo `question_id`.
- `migrations/0013_quiz_attempts.up.sql:55`: `quiz_review_notes.question_id`. Unique `(review_id, question_id)` có cùng vấn đề.
- `migrations/0012_homework_submissions.up.sql:56`: `grade_criterion_scores.criterion_id`. PK `(grade_id, criterion_id)` có cùng vấn đề.

**Kịch bản lỗi:** Có nhiều chỗ xóa câu hỏi hoặc tiêu chí của bản draft, mỗi dòng bị xóa lại kích hoạt RI check:
- `stages/repository_pg_content.go:240,252,300,406`.
- Xóa cả phiên bản draft, cascade xuống lessons, questions và rubric_criteria.

Mỗi RI check là một seq scan trên `quiz_attempt_answers`, `quiz_review_notes` hoặc `grade_criterion_scores`. Bảng `quiz_attempt_answers` tăng theo số lượt × số câu. Khi hệ thống đã chạy vài khóa, xóa một quiz draft 30 câu thành 30 lần full scan trong transaction soạn bài.

**Cách sửa:** Thêm migration `CREATE INDEX ix_quiz_attempt_answers_question ON quiz_attempt_answers (question_id)` và làm tương tự cho hai bảng còn lại. `homework_attachments.media_id` (0011:83) chỉ cần khi có thao tác xóa media, hiện chưa có.

### L2. Truy vấn danh sách quiz và homework của lớp gom toàn bảng ở mỗi request

**Vị trí:**
- `apps/api/internal/features/quizzes/repository_pg.go:595`: `LEFT JOIN (SELECT lesson_id, count(*) FROM questions GROUP BY lesson_id)`.
- `apps/api/internal/features/homework/repository_pg.go:376`: tương tự, với `sum(max_points)` trên `rubric_criteria`.

**Kịch bản lỗi:** Subquery không có điều kiện lọc. Postgres có thể không đẩy điều kiện `lesson_id` vào trong `GROUP BY`, nên nó gom mọi câu hỏi hoặc tiêu chí của mọi phiên bản, kể cả draft và archived, ở mỗi lần mở tab Quiz hoặc Homework của lớp.

**Cách sửa:** Đổi sang `LEFT JOIN LATERAL (SELECT count(*) FROM questions q WHERE q.lesson_id = l.id)`, hoặc thêm `WHERE lesson_id = ANY(...)` giới hạn trong các lesson của course version.

### L3. Comment cũ trong DTO

**Vị trí:** `apps/api/internal/features/quizzes/dto.go:578`.

**Vấn đề:** Comment nói `grants` "chưa có use case cấp lượt nên luôn là mảng rỗng". Nhưng `grant_service.go` đã có use case cấp lượt và `dto.go:708-711` đã điền dữ liệu. Comment sai sẽ đánh lừa người sửa sau.

**Cách sửa:** Cập nhật comment.

### L4. Làm mới cache phía staff chưa phủ trang tổng kết

**Vị trí:** `apps/web/src/features/grading/hooks/use-submission.ts:23-28` và `use-quiz-review.ts:24-29`.

**Vấn đề:** Sau khi chấm hoặc xác nhận, các hook invalidate queue, recent và tab lớp, nhưng không invalidate `summaryKeys.matrix(classId)` hay `summaryKeys.memberStage`. Nếu giảng viên mở ma trận tổng kết trong vòng 30 giây sau khi chấm, ô "Làm lại" hoặc "Hoàn thành" vẫn là giá trị cũ.

**Cách sửa:** Invalidate theo prefix `["class-summary"]` và `["member-stage-summary"]`.

## Đã kiểm, không có lỗi

1. **Migrations 0010–0014:**
   - Backfill bài học "Nội dung chặng" (`noi-dung-chang`) cho mọi `stage_version`, sau đó `unit_id NOT NULL` và FK ghép `(unit_id, stage_version_id)`, nên lesson không thể trỏ sang unit của phiên bản khác.
   - Có partial unique index cho các cặp sau:
     - Một lượt mở: `uq_quiz_attempts_one_open`.
     - Một bản chờ cho quiz attempt: `uq_quiz_attempts_one_pending`.
     - Một bản chờ cho submission: `uq_submissions_one_pending`.
   - Có `uq submission_files.media_id` để chặn gắn một file vào hai bản nộp.
   - Bảng outbox có CHECK cho 4 template mới.
   - Down migration đầy đủ. Down của 0011 có ghi rõ sẽ thất bại khi đã có dữ liệu quiz hoặc homework.
2. **Plan §4, mục completed_at:**
   - `quizzes/service.go` `autoOrSubmit` gọi `MarkCompleted` trong cùng transaction với `Finish`.
   - `homework/service.go` `Grade` gọi `MarkCompleted` hoặc `ClearCompleted` trong cùng transaction với `InsertGrade` và `SetResult`.
   - `learning.SetCompletion` từ chối quiz và homework bằng `ErrCompletionByResult`.
3. **404 khi id thuộc lớp khác:**
   - `AttemptRef` join `class_members` để kiểm chủ sở hữu.
   - `LockClassMember(classID, memberID)` cho grant.
   - `QuizInCourseVersion` và `lessonOf` giới hạn lesson trong course version của lớp.
   - `summary` dùng `Member` hoặc `LockMember WHERE cm.class_id = $1 AND cm.id = $2`, và trả `ErrStageNotInClass` (NotFound).
4. **Thứ tự khóa:** Các luồng sau đều khóa `class_members FOR UPDATE OF cm` trước, rồi mới khóa attempt hoặc submission:
   - Quiz: Start, SaveAnswer, Submit, CloseExpired, Review, Grant.
   - Homework: Submit, Grade.
   - Summary: SaveReview.

   Các luồng nhạy cảm còn có ghi có điều kiện:
   - `Finish` chỉ ghi khi `submitted_at IS NULL`.
   - `SetResult` chỉ ghi khi `status <> 'superseded'`.
   - `ConfirmReview` dùng `IS NOT DISTINCT FROM`.
5. **Email:**
   - `mailer/service.go:16` từ chối các khóa `TempPassword`, `Link` và `Token`.
   - Payload của 4 template mới dùng `ResultURL`, `QueueURL` và `LessonURL`, không chứa nội dung câu hỏi hay đáp án.
   - Template dùng `missingkey=error`, và mọi khóa được tham chiếu đều có trong payload.
6. **Audit:** 8 action mới được đăng ký đủ 4 nơi:
   - `audit/actions.go` và `actions_test.go`.
   - `audit/audit.go` `knownActions`.
   - `reports/model.go`: nhãn và tóm tắt.
   - `reports/model_test.go`.

   Payload thật có `lessonTitle`, `studentName` và `stageName` khớp với `summarize`.
7. **Không lộ đáp án:**
   - Bản tóm tắt học liệu (`learning/dto.go` `QuizDTO`) không có câu hỏi.
   - `OpenAttemptDTO` của lượt đang mở chỉ có id và body của phương án.
   - `AssessmentResultDTO` không có phương án, `isCorrect` hay giải thích. Ghi chú chỉ có body câu hỏi.
   - Zod `assessmentResultSchema` cũng không có các trường đó và strip khóa lạ.
8. **Phân quyền:**
   - Mọi route mới đều gắn guard staff hoặc student.
   - `staffCanAccessClass`: admin truy cập mọi lớp, teacher chỉ lớp có `teacher_id` là mình.
   - `grading` giới hạn theo `actor.scope()`.
   - `/me/uploads` có `requireStudent`, chỉ nhận `kind = document`, và người dùng phải là thành viên active của một lớp active.
   - `CanUserAccess` thêm quyền cho người upload và cho giảng viên của lớp có bản nộp chứa file. File đính kèm homework đi qua `lesson_media`.
9. **Dedupe job:** `ClaimRun` chạy `INSERT … ON CONFLICT DO NOTHING` trong cùng transaction với `Enqueue`. Nếu enqueue lỗi thì khóa bị rollback. Khi hai replica chạy cùng lúc, replica thứ hai chờ unique index rồi nhận conflict.
10. **Ranh giới import:**
    - `grading` chỉ import `mailer`, không import homework, quizzes hay reports.
    - `summary` không import learning, reports, homework hay quizzes.
11. **Zod và DTO:** Đã đối chiếu `quiz-schemas.ts` với `quizzes/dto.go`, và `learning/model/schemas.ts` (submission, grade, mySubmissions) với `homework/dto.go`. Tên trường, nullable và optional khớp nhau.

## Việc nên làm

1. **H1:** Lead chọn hướng (a), (b) hoặc (c). Khuyến nghị (a) + (b) trong `StudentUploadQuota`, kèm test.
2. **M1:** Invalidate `myClass` và `myClasses` sau khi nộp homework.
3. **L1:** Thêm migration index cho 3 cột FK.
4. **L2, L3 và L4:** Làm khi tiện.
5. Chạy đủ `go test ./...` (cần DB test), web typecheck, lint và vitest sau khi agent kia xong. Review này chưa chạy các bước đó.

## Câu hỏi còn mở

- Worker chỉ tự nộp lượt quiz hết giờ mỗi giờ một lần. Trong lúc chờ, lượt đó chưa vào hàng chờ chấm, trừ khi học viên mở lại quiz và kích hoạt đóng lười. Plan đã chấp nhận điều này nên không tính là lỗi. Cần xác nhận rằng chậm tối đa khoảng một giờ trong hàng chờ là chấp nhận được.

Status: DONE_WITH_CONCERNS
Summary: Backend giữ đủ các bất biến của plan §4 (khóa, completed_at, 404, không lộ đáp án, email, audit, import), build và vet sạch. Còn một lỗi High: học viên bị khóa upload vĩnh viễn vì file chưa gắn không bao giờ được nhả; thêm một lỗi Medium về cache web và vài lỗi Low về index và truy vấn.

## Xử lý (lead, 2026-10-19)

| Mục | Quyết định | Bằng chứng |
|---|---|---|
| H1 | Đã sửa theo lựa chọn của người dùng: hạn mức 10 file chưa gắn chỉ tính file `ready` tạo trong 24 giờ (`UnattachedWindow`) và file `pending` tạo trong `UploadTTL` (15 phút). Không thêm job dọn file, không thêm endpoint xóa. | `TestStudentUnattachedLimitForgetsStaleUploads` (đỏ khi bỏ điều kiện cửa sổ, xanh khi có), `TestStudentUploadRules` kiểm cửa sổ tính từ đồng hồ; `go test ./internal/features/media/...` xanh. |
| M1 | Đã sửa: nộp homework invalidate `myClass`, `myClasses` và tổng kết học viên; nộp quiz cũng invalidate tổng kết học viên. | Test "relabels the homework in the side list after a resubmission" (đỏ khi bỏ invalidate, xanh khi có). |
| L1 | Hoãn, ghi nợ kỹ thuật. RI check chỉ chạy khi xóa câu hỏi/tiêu chí của bản nháp; ở quy mô hiện tại chi phí nhỏ, còn thêm migration lúc này phải đổi chuỗi migration đang dùng chung với seed và E2E. | — |
| L2 | Đã sửa: đổi sang `LEFT JOIN LATERAL` lọc theo `lesson_id`, kết quả giữ nguyên. | `go test` gói quizzes, homework. |
| L3 | Đã sửa comment `QuizItemDTO`. | — |
| L4 | Đã sửa: chấm bài và xác nhận quiz invalidate prefix `class-summary` và `member-stage-summary`. | Assertion mới trong `grade-submission-page.test.tsx`, `review-attempt-page.test.tsx`. |
| Câu hỏi mở (tự nộp mỗi giờ) | Giữ như plan đã chấp nhận: lượt hết giờ vào hàng chờ chậm tối đa khoảng một giờ, hoặc ngay khi học viên mở lại quiz. | plan §4 |
