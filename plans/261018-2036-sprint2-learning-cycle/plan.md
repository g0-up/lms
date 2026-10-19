---
title: "Sprint 2: chu trình học tập theo chặng (bài học, homework, quiz, tổng kết)"
description: "Hiện thực toàn bộ spec-lms-v2.md (P0 FR-60→64, 70, 71, 74→78, 80, 82, 90→92 và P1 FR-65→68, 72, 73, 81) trên monorepo MVP apps/api + apps/web, lấy prototype/ làm chuẩn UX và copy."
status: completed
priority: P1
effort: "16 phase, ước lượng ~32 ngày công (xem bảng Phases)"
issue: ""
branch: master
tags: [feature, backend, frontend, database, api, sprint2]
blockedBy: []
blocks: []
created: 2026-10-18
---

# Sprint 2: chu trình học tập theo chặng

## 1. Kết quả mong muốn

Trên hệ thống MVP đang chạy, mỗi phiên bản chặng được chia thành **bài học**. Mỗi bài học chứa bốn loại học liệu: video, bài đọc, quiz (luyện tập hoặc đánh giá) và homework. Chu trình chạy đủ trong hệ thống:

- Admin soạn bài học, quiz và homework trong bản nháp, phát hành, nhân bản sâu.
- Học viên học theo bài học, làm quiz từng câu, nộp homework (repo và/hoặc file) và xem tổng kết chặng.
- Giảng viên chấm homework theo tiêu chí, xác nhận quiz bài tập trong một hàng chờ, nhận xét quiz học liệu, đặt hạn nộp, cấp thêm lượt, xem thống kê câu hỏi, xem ma trận tổng kết lớp và viết nhận xét chặng.
- Admin xem tồn đọng chấm theo giảng viên. Hệ thống gửi email kết quả, nhắc chấm trễ và nhắc hạn nộp.

Nguồn chân lý: `spec-lms-v2.md`. UX, copy tiếng Việt và hình dạng dữ liệu mẫu lấy từ `prototype/app.js`, `prototype/seed.js` và `docs/prototype.md`. Báo cáo scout: [scout-api](reports/scout-api.md), [scout-web](reports/scout-web.md), [scout-prototype](reports/scout-prototype.md).

## 2. Phạm vi (Scope Challenge: HOLD SCOPE)

Người dùng yêu cầu "mọi feature trong spec lms v2" và không bật `--yagni`, nên plan không cắt và không thêm. P1 nằm trong plan nhưng ở các phase riêng (14, 15). Nếu thiếu năng lực, điểm cắt theo spec §10 là dừng sau phase 13 và chuyển quiz (phase 09–11) sang sprint sau; tổng kết vẫn chạy được với homework và học liệu nội dung.

### Ràng buộc

- Giữ kiến trúc MVP: feature folder handler → service → repository; `db.Transact`; khóa hàng `ByIDForUpdate`; `pgerr` registry; `audit.Recorder`; outbox `mailer.Enqueuer`; `clock.Clock`; id uuid v7.
- D1: không trigger, function hay extension. Bất biến của phiên bản `published` (bài học, học liệu, câu hỏi, đáp án, tiêu chí) được bảo vệ ở domain cộng SQL có điều kiện, giống `lessons` hiện tại. Enum là `text` kèm CHECK có tên.
- Không thêm mã lỗi mới. Tái dùng `VALIDATION_FAILED`, `FORBIDDEN`, `NOT_FOUND`, `CONFLICT`, `INVALID_TRANSITION`, `VERSION_IMMUTABLE`, kèm thông điệp tiếng Việt cụ thể.
- Đáp án đúng không bao giờ rời server trước khi lượt quiz được nộp. Với quiz bài tập, học viên không thấy đáp án ở bất kỳ thời điểm nào.
- File học viên nộp chỉ tải xuống (`Content-Disposition: attachment`), không mở inline.
- Web: shared → features → app, import feature qua `index.ts`, và `pnpm lint:design` (không hex thô, không emoji, chữ ≥ 12px, hit area ≥ 44px).
- Mỗi migration bump `schemaVersion` và đăng ký tên ràng buộc mới vào `pgerr/constraints.go`. Sao lưu DB trước khi chạy migration trên môi trường có dữ liệu thật.

### Ngoài phạm vi

Toàn bộ spec §3 và §7.3 (P2): chấm code tự động, điểm tổng có trọng số, chặn chặng, ngân hàng câu hỏi, câu tự luận, chống gian lận, chấm chéo, giảng viên sửa điểm quiz, quiz hoặc hạn quiz theo lớp. Ngoài ra: loại câu hỏi khỏi tính điểm (OQ-7), báo cáo theo bài học (OQ-14), quét mã độc file nộp, và các điều khiển chỉ có trong bản mẫu ("Chạy tác vụ hằng ngày", `sprint2Guide`, thẻ "Email tự động (giả lập)").

## 3. Giả định cho câu hỏi mở

| Câu hỏi | Giả định trong plan | Phase chịu ảnh hưởng |
|---|---|---|
| OQ-1 repo private (chặn FR-62) | **Đã chốt:** repo phải để public. Form ghi rõ yêu cầu này; server chỉ kiểm định dạng URL, không gọi API Git hay host repo. Giảng viên không mở được repo thì chấm "Cần làm lại" kèm nhận xét (trường hợp biên spec dòng 161) | 06, 08 |
| OQ-2 homework hoàn thành (chặn FR-80) | **Đã chốt:** hoàn thành khi bài nộp mới nhất được chấm **Đạt** | 06, 12 |
| OQ-3 lượt đánh giá | Mặc định 2 lượt, tính lượt cao nhất | 04, 09 |
| OQ-4 đáp án quiz bài tập | Học viên không bao giờ thấy đáp án; giảng viên thấy | 09, 10 |
| OQ-5 thời hạn chấm | 3 ngày lịch (72 giờ) tính từ `submitted_at`. Email nhắc gửi giảng viên phụ trách lớp; Admin xem tồn đọng trên dashboard | 14 |
| OQ-6 file nộp | Tối đa 5 file, mỗi file ≤ 20 MB; zip, pdf, md, txt, sql, png, jpg/jpeg; không quét mã độc | 06 |
| OQ-7 đáp án sai khi lớp chạy | Chỉ xử lý bằng cấp thêm lượt (FR-72) | 15 |
| OQ-9 hạn quiz theo lớp | Không có | — |
| OQ-12 xóa bài học còn học liệu | Xóa luôn học liệu sau hộp xác nhận nêu số học liệu | 02, 03 |
| OQ-13 bài học chỉ có học liệu không bắt buộc | Không có trạng thái hoàn thành, hiện "Không bắt buộc" | 02, 03 |
| OQ-15 dữ liệu cũ (chặn migration FR-78) | **Đã chốt:** migration gán một bài học mặc định "Nội dung chặng" (`unit_key = noi-dung-chang`) cho mọi phiên bản chặng đã có, chứa toàn bộ học liệu theo thứ tự cũ. Admin chia lại qua bản nháp mới | 01 |
| Nhãn "Quá hạn" homework | Trạng thái hiển thị suy ra (có hạn, đã qua hạn, chưa nộp), không lưu | 12 |
| Nhãn bản nộp bị thay thế | Copy UI theo prototype: "Đã thay bằng lần nộp sau"; giá trị kỹ thuật `superseded` | 06, 08 |

## 4. Quyết định thiết kế chính

1. **Hoàn thành ghi xuống `lesson_progress.completed_at` trong cùng giao dịch** với sự kiện quyết định nó: nộp lượt luyện tập đầu tiên; lượt đánh giá đạt ngưỡng; chấm Đạt (và xóa mốc khi sửa điểm thành Cần làm lại). Công thức % (`percentOfRequired`, `memberProgress` CTE) giữ nguyên, nên không đụng SQL báo cáo của MVP. `PUT …/completion` từ chối với quiz và homework (FR-91).
2. **`lessons.position` giữ thứ tự phẳng toàn phiên bản**, luôn khớp thứ tự (bài học, học liệu trong bài học). Domain đánh lại số sau mỗi thao tác, nên các truy vấn `ORDER BY l.position` của learning và reports vẫn đúng. `unit_id` nằm trong FK ghép `(unit_id, stage_version_id)`.
3. **`StageVersion` vẫn là aggregate duy nhất của nội dung**: bài học, học liệu, `QuizContent` (cấu hình, câu hỏi, phương án) và `HomeworkContent` (đề bài, ngưỡng, tiêu chí, file kèm). `SaveDraft` diff thêm các bảng con với cùng điều kiện `status='draft'`.
4. **Feature backend mới**: `homework` (hạn nộp, bài nộp, chấm), `quizzes` (lượt làm, chấm tự động, xác nhận, nhận xét, cấp lượt, thống kê), `grading` (hàng chờ hợp nhất, tồn đọng, việc hằng ngày) và `summary` (trạng thái học liệu theo học viên, tổng kết chặng, khóa và lớp, nhận xét chặng). `learning` gọi `summary.StatusReader` để hiện nhãn trạng thái trên lộ trình. `reports` gọi `grading` qua interface `BacklogReader` cho thẻ tồn đọng trên dashboard. `grading` không import `homework`, `quizzes` hay `reports`.
5. **Việc hằng ngày và tự nộp lượt quá giờ** chạy trong `lms worker`, đăng ký vào `Cleanups` hiện có (mỗi giờ). Tự nộp cũng được làm lười khi đọc lượt. Email hằng ngày khử trùng lặp bằng bảng `scheduled_job_runs (run_key PK)` với `INSERT … ON CONFLICT DO NOTHING`: mỗi khóa được enqueue một lần, kể cả khi nhiều replica; outbox giao ít nhất một lần, nên hiếm khi có thể trùng email khi SMTP lỗi giữa chừng.
   - Payload email không dùng khóa `Link`, `Token` hay `TempPassword` (`mailer/service.go:16` từ chối). URL trong email đặt ở khóa riêng (`ResultURL`, `QueueURL`, `LessonURL`), dựng từ `PublicBaseURL` truyền vào service như `classes` đang làm với `LoginURL`.
6. **Media**: thêm kind `document`. Admin tải file kèm homework (qua `lesson_media`, nên quyền xem như học liệu). Học viên tải file nộp qua route riêng; quyền xem gồm người nộp, giảng viên lớp và Admin. Mọi `document` được ký URL với `attachment`.
7. **Audit Sprint 2** (FR-92): mọi action mới dùng `targetType=class`, `targetID` là lớp. `after` (và `before` khi sửa) chứa `classCode`, `lessonId`, `lessonTitle`, `memberId`, `studentName` cùng chi tiết của thao tác. Nhờ vậy nhật ký trên dashboard hiện nhãn lớp mà không thêm loại tra cứu. Mỗi action mới phải sửa đủ bốn chỗ trong cùng phase:
   - `audit/actions.go` (hằng và comment đếm số action) cùng `actions_test.go`;
   - `audit/audit.go` `knownActions`, vì `PG.Record` từ chối action lạ;
   - `reports/model.go`: nhãn trong `actionLabels`, các khóa mới trong `activityPayload` (`lessonTitle`, `studentName`) và một nhánh `summarize` ra dạng "{lessonTitle} · {studentName}";
   - `reports/model_test.go` cho nhãn và câu tóm tắt.
8. **Phạm vi truy cập theo lớp**: route nào có cả `:id` (lớp) và `:mid` (thành viên), `:sid`, `:aid` hay `:svid` thì truy vấn luôn ràng theo lớp đó, như `reports.MemberReport` (`reports/service.go:126`). Không khớp thì trả `NOT_FOUND`, không lộ dữ liệu lớp khác. Mỗi route như vậy có test "giảng viên dùng id lớp mình với id thành viên lớp khác nhận 404".
9. **Thứ tự khóa**: mọi đường ghi lên một thành viên (nộp bài, chấm, bắt đầu, lưu, nộp và xác nhận quiz, cấp lượt) khóa `class_members` trước, rồi mới khóa bản nộp hay lượt làm bài. Lệnh chuyển trạng thái luôn có điều kiện trạng thái cũ trong `WHERE` và dùng `ExecAffectOne`.
10. **Web: sửa ở `src/app` và `src/shared`** là việc nền theo `docs/architecture.md` (mục quyền sở hữu). Plan này tuần tự nên được phép sửa, nhưng mỗi phase liệt kê rõ các file đó ở mục Files, và các phase cùng sửa `src/app/router.tsx` (08, 11, 13) chạy nối tiếp, không song song.

## 5. Mô hình dữ liệu (migration 0010–0014)

Chi tiết DDL ở [phase 01](phase-01-database-migrations.md). Tóm tắt theo Phụ lục A của spec:

| Migration | Bảng / thay đổi |
|---|---|
| 0010_stage_units | `stage_units`; `lessons.unit_id` (backfill theo OQ-15, rồi NOT NULL, FK ghép) |
| 0011_quiz_homework_content | `lessons.type` thêm `quiz`, `homework`; `quiz_settings`, `questions`, `question_options`, `homework_settings`, `rubric_criteria`; `media_files.kind` thêm `document` |
| 0012_homework_submissions | `class_lesson_deadlines`, `submissions`, `submission_files`, `submission_grades`, `grade_criterion_scores` |
| 0013_quiz_attempts | `quiz_attempts`, `quiz_attempt_answers`, `quiz_attempt_reviews`, `quiz_review_notes`, `quiz_attempt_grants` |
| 0014_stage_reviews_jobs_emails | `stage_reviews`, `scheduled_job_runs`; `ck_email_outbox_template` thêm `homework_graded`, `quiz_reviewed`, `grading_digest`, `deadline_reminder` |

## 6. Tiêu chí chấp nhận toàn plan

- [x] Mọi FR P0 và P1 của spec có ít nhất một test tự động (unit hoặc integration ở API, Vitest hoặc Playwright ở web), được liệt kê trong phase tương ứng.
- [x] `make -C apps/api lint test` (có `TEST_DATABASE_URL`) và `pnpm -C apps/web lint typecheck test lint:design` xanh. Các spec Playwright mới chạy xanh với `E2E_SPECS`.
- [x] Ghi vào bài học, học liệu, câu hỏi, đáp án hoặc tiêu chí của phiên bản `published`, qua API hay qua repository gọi thẳng, đều bị từ chối `VERSION_IMMUTABLE`.
- [x] Response của `GET` lượt quiz đang làm và kết quả quiz bài tập không chứa trường đúng/sai của phương án. Có test khẳng định điều này trên JSON thô.
- [x] Hai request "bắt đầu lượt" đồng thời chỉ tạo một lượt đang làm. Nộp sau `deadline_at + 30s` bị từ chối, và lượt quá giờ được tự nộp với các câu đã lưu.
- [x] Kịch bản đầu cuối Playwright: Admin soạn bài học, quiz và homework rồi phát hành. Học viên làm quiz luyện tập, làm quiz bài tập, nộp homework. Giảng viên chấm và xác nhận từ hàng chờ. Học viên nhận email (Mailpit) và thấy tổng kết chặng Hoàn thành. Ma trận lớp hiện đúng ô.
- [x] Phiên bản chặng đã phát hành trước migration hiện đủ học liệu trong bài học "Nội dung chặng"; % tiến độ học viên không đổi trước và sau migration.
- [x] `docs/architecture.md` và `docs/database.md` mô tả feature, route và bảng mới.

## 7. Ngôn ngữ chung bổ sung

| UI | Mã (Go / TS / SQL) | Ghi chú |
|---|---|---|
| Bài học | `Unit` / `stage_units` | `unit_key` giữ qua phiên bản |
| Quiz học liệu / Quiz bài tập | `QuizMode`: `practice` / `assessment` | `quiz_settings.mode` |
| Lượt làm bài | `Attempt` / `quiz_attempts` | `attempt_no` theo (thành viên, học liệu) |
| Chờ xác nhận / Đã xác nhận / Đã có lượt mới hơn | `ReviewStatus`: `pending` / `confirmed` / `superseded` | lượt practice chỉ có `confirmed` khi đã nhận xét |
| Bài nộp, lần nộp | `Submission` / `submissions` | `status`: `pending` / `passed` / `rework` / `superseded` |
| Chấm bài / kết quả chấm | `Grade` / `submission_grades` | append-only; dòng mới nhất là hiện hành |
| Tiêu chí chấm | `Criterion` / `rubric_criteria` | |
| Hạn nộp | `Deadline` / `class_lesson_deadlines` | theo lớp |
| Cấp thêm lượt | `Grant` / `quiz_attempt_grants` | |
| Tổng kết chặng / Nhận xét chặng | `StageSummary` / `stage_reviews` | tổng kết tính toán, nhận xét lưu |
| Hàng chờ chấm | `QueueItem` (`kind`: `homework` / `quiz`) | read model |

## 8. Bề mặt API mới (tóm tắt)

| Nhóm | Endpoint | Quyền |
|---|---|---|
| Bài học | `POST /stage-versions/:vid/units`, `PATCH|DELETE /stage-versions/:vid/units/:uid`, `PUT /stage-versions/:vid/units/order`; `PUT /stage-versions/:vid/lessons/order` đổi body thành `{units:[{unitId, lessonIds}]}` | admin |
| Soạn quiz/homework | `POST /stage-versions/:vid/lessons/:lid/questions`, `PATCH|DELETE …/questions/:qid`, `PUT …/questions/order`; `POST|PATCH|DELETE …/criteria[/:cid]`, `PUT …/criteria/order` | admin |
| Media | `POST /me/uploads`, `POST /me/uploads/:id/complete` (học viên, chỉ kind `document`, trong media handler) | student |
| Homework học viên | `GET|POST /me/classes/:id/lessons/:lid/submissions` | student |
| Quiz học viên | `POST /me/classes/:id/lessons/:lid/attempts`, `GET /me/classes/:id/lessons/:lid/attempts`, `GET /me/attempts/:aid`, `PUT /me/attempts/:aid/answers/:qid`, `POST /me/attempts/:aid/submit` | student |
| Tổng kết học viên | `GET /me/classes/:id/summary` | student |
| Chấm | `GET /grading/queue`, `GET /grading/recent`, `GET /submissions/:sid`, `POST /submissions/:sid/grades`, `GET /quiz-attempts/:aid`, `POST /quiz-attempts/:aid/reviews` | staff (teacher chỉ lớp mình) |
| Lớp | `PUT|DELETE /classes/:id/deadlines/:lid`, `GET /classes/:id/homework`, `GET /classes/:id/quizzes`, `POST /classes/:id/members/:mid/grants`, `GET /classes/:id/summary`, `GET /classes/:id/members/:mid/stages/:svid/summary`, `PUT /classes/:id/members/:mid/stages/:svid/review` | staff |
| Dashboard | `GET /dashboard` thêm `gradingBacklog`, `overdueCount` | admin |

## 9. Routes frontend mới

- Admin: `/admin/grading`, `/admin/grading/:submissionId`, `/admin/grading/quiz/:attemptId`; `/admin/classes/:classId?tab=summary|homework|quiz|report|students|settings`.
- Giảng viên: `/teach/grading`, `/teach/grading/:submissionId`, `/teach/grading/quiz/:attemptId`; `/teach/classes/:classId?tab=summary|homework|quiz|report`. NAV thêm "Chấm bài".
- Học viên: `/learn/classes/:classId/lessons/:lessonId?result=<attemptId>`, `/learn/classes/:classId/summary`.
- Giảng viên mở route soạn chặng bị chuyển về `/teach` (đã có nhờ role middleware; thêm test).

## Phases

| # | Phase | FR | Ước lượng | Phụ thuộc | Trạng thái |
|---|---|---|---|---|---|
| 01 | [Migrations và ràng buộc](phase-01-database-migrations.md) | Phụ lục A, OQ-15 | 2.5d | — | completed |
| 02 | [Bài học: backend](phase-02-units-backend.md) | FR-78, FR-90 (bài học) | 2.5d | 01 | completed |
| 03 | [Bài học: web](phase-03-units-web.md) | FR-78 | 2d | 02 | completed |
| 04 | [Soạn quiz và homework: backend](phase-04-quiz-homework-authoring-backend.md) | FR-60, FR-70, FR-74, FR-90 | 3d | 02 | completed |
| 05 | [Soạn quiz và homework: web](phase-05-quiz-homework-authoring-web.md) | FR-60, FR-70, FR-74 | 2.5d | 03, 04 | completed |
| 06 | [Homework: hạn nộp, nộp, chấm (backend)](phase-06-homework-backend.md) | FR-61, FR-62, FR-63 | 3d | 04 | completed |
| 07 | [Hàng chờ chấm: backend](phase-07-grading-queue-backend.md) | FR-64 | 1d | 06 | completed |
| 08 | [Homework và chấm bài: web](phase-08-homework-grading-web.md) | FR-61→64 | 3d | 05, 07 | completed |
| 09 | [Quiz engine: backend](phase-09-quiz-engine-backend.md) | FR-71 | 3d | 04 | completed |
| 10 | [Xác nhận và nhận xét quiz: backend](phase-10-quiz-review-backend.md) | FR-75, FR-76 | 1.5d | 07, 09 | completed |
| 11 | [Quiz: web](phase-11-quiz-web.md) | FR-71, FR-75→77 | 3d | 08, 10 | completed |
| 12 | [Tiến độ và tổng kết: backend](phase-12-progress-summary-backend.md) | FR-80, FR-82, FR-91, FR-92 | 2d | 06, 09 | completed |
| 13 | [Tổng kết: web](phase-13-summary-web.md) | FR-80, FR-82, FR-91 | 2d | 11, 12 | completed |
| 14 | [P1 vận hành chấm: SLA, nhắc, tồn đọng](phase-14-p1-grading-operations.md) | FR-65, FR-66, FR-67, FR-68 | 2d | 10, 13 | completed |
| 15 | [P1 quiz và khóa: cấp lượt, thống kê, tổng kết khóa](phase-15-p1-grants-stats-course-summary.md) | FR-72, FR-73, FR-81 | 2d | 13 | completed |
| 16 | [Seed, E2E đầu cuối, tài liệu](phase-16-seed-e2e-docs.md) | toàn bộ | 2d | 01–15 | completed |

Các phase sửa chung `seed/data.go`, `audit/actions.go`, `app/router.go`, `src/app/router.tsx` và `shared/layout/nav.tsx`, nên chạy tuần tự theo bảng. Mỗi phase backend tự bổ sung seed cho dữ liệu của nó. Mỗi phase web tự thêm spec Playwright của nó.

## Red Team Review

Phiên 2026-10-18, bốn persona (bảo mật, lỗi vận hành, giả định, phạm vi). Gộp thành 17 phát hiện: nhận 15, bỏ 2, và bỏ thêm phần job dọn file mồ côi của phát hiện 8. Chi tiết và bằng chứng ở [red-team-review](reports/red-team-review.md).

| Nhóm | Phát hiện chính | Phase |
|---|---|---|
| Audit | Thêm action vào đủ bốn chỗ, kể cả `knownActions` (§4 mục 7) | 06, 10, 12, 15 |
| Migration | Bài học mặc định trong `CreateStage` và `Clone`; unique tên theo `lower(btrim(title))` | 01, 02 |
| Đồng thời | Khóa `class_members` trước; đổi trạng thái có điều kiện và `ExecAffectOne` (§4 mục 9); tự nộp quiz không rollback | 06, 09, 15 |
| Quyền | Ràng `:mid`, `:svid` theo lớp (§4 mục 8); upload học viên qua `/me/uploads` có giới hạn; DTO học viên riêng và test allow-list | 06, 09, 10, 12, 15 |
| Email và job | Khóa URL riêng thay `Link`; việc định kỳ chạy trong `Cleanups` (§4 mục 5) | 06, 09, 10, 14 |
| Web | Giữ tab mặc định `report`; `TabBar` dùng chung; chỉ ẩn nút tích, giữ `LessonNav`; mở rộng enum; bỏ badge nav và `.strict()` | 05, 08, 11, 13 |
| Media | Type chuẩn theo đuôi file, client gửi đúng header đã ký, tải xuống luôn `attachment` | 04, 05 |

### Whole-Plan Consistency Sweep

- Đã rà mọi phase để tìm các cụm cũ: `Sweeps`, khóa `Link`, `.strict()`, `day.ts`, `ErrOrderMismatch`, "đúng một lần", tab mặc định `summary`, badge nav, và tên golden sai. Không còn chỗ nào mâu thuẫn.
- Tên golden đã khớp thư mục thật: `stages/testdata/*` dùng gạch dưới, `learning/testdata/*` dùng gạch ngang.
- Phase 08, 11 và 13 sửa `src/app/router.tsx` và chạy tuần tự. `src/shared/ui/tab-bar.tsx` (phase 08) và `src/shared` upload helper (phase 05) là việc nền.

## Validation Log

Phiên 2026-10-18, 7 câu, người dùng trả lời tất cả.

| Câu hỏi | Trả lời | Tác động |
|---|---|---|
| OQ-2: homework hoàn thành khi nào | Bài mới nhất được chấm Đạt | Giữ nguyên phase 06, 12 |
| OQ-15: dữ liệu cũ vào bài học nào | Một bài học "Nội dung chặng" | Giữ nguyên phase 01 |
| Chặng không có học liệu bắt buộc | "Đang học" (`Complete=false`) | Giữ nguyên phase 12 |
| Học viên rời lớp còn bài chờ | Ẩn khỏi hàng chờ | Giữ nguyên phase 07 (lọc `cm.status='active'`), phase 14 dùng lại |
| OQ-1: repo private | Bắt buộc repo public | Sửa §3, phase 06 (ghi quyết định), phase 08 (copy help và test) |
| Nhãn bản nộp bị thay thế | Theo prototype: "Đã thay bằng lần nộp sau" | Giữ nguyên phase 06, 08 |
| "Quá hạn" suy ra, 2–6 phương án, không audit bài học | Giữ cả ba | Giữ nguyên |

## Câu hỏi còn mở

Xem mục 3. Ba câu chặn theo spec (OQ-1, OQ-2, OQ-15) đã được chốt ở bước validate.

- Có tự động kiểm repo public không (gửi request ẩn danh tới github.com, gitlab.com, bitbucket.org lúc nộp)? Plan hiện không làm, vì cần gọi ra ngoài, có rate limit và rủi ro SSRF. Yêu cầu public chỉ được nêu trên form.
