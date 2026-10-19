# Báo cáo kiểm chứng Sprint 2: vòng học khép kín

- Ngày: 2026-10-19
- Kết luận: mọi mục ở [plan.md](../plan.md) §6 có bằng chứng xanh. Mọi FR P0 và P1 đều có ít nhất một test tự động.
- Không commit. Không đụng container nào ngoài stack lms. Không chạy `e2e-stack.sh down`.

## 1. Lệnh đã chạy

Tất cả các lệnh chạy trên cây làm việc hiện tại, sau khi seed, golden và tài liệu đã ổn định.

| Lệnh | Kết quả |
|---|---|
| `cd apps/api && make lint GOLANGCI_LINT="go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0"` | `go vet` (thường và `-tags integration`) sạch; golangci-lint `0 issues.` |
| `cd apps/api && make test` (`TEST_DATABASE_URL` → `lms_test` trên 5433, đọc từ `apps/api/.env`) | Mọi package `ok`, gồm `seed`, `migrations` và các feature `grading`, `homework`, `quizzes`, `summary`, `stages`, `reports`; `exit=0` |
| `make seed-reset` hai lần liên tiếp (dev DB `lms` trên 5433, đã backup bằng `pg_dump` trước) | Cả hai lần `exit=0`; số bản ghi giống nhau: 17 users, 6 stage_versions, 39 lessons, 10 submissions, 33 quiz_attempts, 2 stage_reviews, 33 audit_logs, 97 lesson_progress, 15 email_outbox |
| `pnpm -C apps/web lint` / `typecheck` | Sạch |
| `pnpm -C apps/web test` | 98 file, 1018 test, tất cả pass |
| `pnpm -C apps/web lint:design` | `design-audit: 0 violations` |
| `pnpm -C apps/web build` | Build xong; chỉ có cảnh báo kích thước chunk > 500 kB, có từ trước |
| Playwright `--project=e2e --project=a11y` trên stack Docker (web :8081, Mailpit :8035) | `78 passed (11.9m)`, `exit=0`; gồm mọi spec MVP và Sprint 2 |

Các lần chạy trên đã gồm các sửa đổi review của controller, vì các file này sửa lúc 06:49–06:51, trước mọi lần chạy. Image `lms-api`, `lms-web` và `lms-worker` của stack E2E build lúc 07:24, bộ Playwright chạy 07:32–07:44, `make test` kết thúc 07:47, bộ Vitest chạy sau đó. Các sửa đổi gồm:

- Hạn mức 10 file chưa gắn của học viên chỉ tính file ready trong 24 giờ và file pending trong 15 phút (`media/*`, test `TestStudentUnattachedLimitForgetsStaleUploads`). Đã mô tả ở `docs/architecture.md`, phần file học viên nộp.
- Invalidate cache tổng kết sau khi chấm, xác nhận và nộp bài (`summaryKeys` export với `matrices`, `memberStages`). Có các test mới trong `homework-lesson.test.tsx`, `grade-submission-page.test.tsx`, `review-attempt-page.test.tsx`.
- Query danh sách quiz/homework của lớp dùng `LEFT JOIN LATERAL`. Kết quả không đổi; golden `class-quizzes.json` vẫn khớp.

Controller báo hai test hỏng, nhưng cả hai đã được đối chiếu với seed mới từ trước và đều pass khi chạy riêng (`go test -tags integration -run 'TestRecentGradedAndConfirmed|TestClassQuizzesMatchesSeed'`):

- `TestRecentGradedAndConfirmed`: basic02 có bài `ds-hw` Cần làm lại của Nhàn và lượt `db-test` của Phong, đều do Bảo xử lý. Khẳng định vẫn so khớp đúng danh sách hai mục.
- `TestClassQuizzesMatchesSeed`: golden đã tạo lại theo seed mới. Test web dùng golden này (`quiz-tab.test.tsx`) đã sửa `lessonId` theo golden mới và vẫn xanh.

Ghi chú về lệnh:

- golangci-lint: binary cục bộ `~/go/bin/golangci-lint` được build bằng go1.26, còn module nhắm 1.27, nên phải ghim phiên bản qua `go run`. Phase 14 đã dùng cách này. Đây là vấn đề môi trường, không phải lỗi code.
- `E2E_SPECS`: ví dụ `E2E_SPECS='…' pnpm test:e2e` trong phase file không phải biến môi trường thật. `E2E_SPECS` là hằng regex trong `apps/web/e2e/playwright.config.ts`, và không tài liệu nào nhắc tới nó. Phase này đã thêm `homework|quiz|summary|learning-cycle` vào regex đó (các spec `units`, `quiz-homework-authoring` đã có sẵn). Bộ chạy ở trên dùng `--project=e2e --project=a11y`, nên chạy đủ mọi spec khớp regex.

## 2. Đối chiếu plan.md §6

| Tiêu chí | Bằng chứng |
|---|---|
| Mọi FR P0 và P1 có ít nhất một test tự động | Bảng ở mục 3. Không FR nào thiếu test. |
| `make -C apps/api lint test` và `pnpm -C apps/web lint typecheck test lint:design` xanh; các spec Playwright mới xanh | Mục 1: tất cả xanh. Các spec mới gồm `units`, `quiz-homework-authoring`, `homework`, `quiz`, `summary`, `learning-cycle` và các route mới trong `a11y`. |
| Ghi vào bài học, học liệu, câu hỏi, đáp án hoặc tiêu chí của phiên bản `published`, qua API hay repository, đều bị từ chối `VERSION_IMMUTABLE` | Qua repository: `TestImmutability_QuizHomeworkContent` và các test `TestImmutability_*` khác (`stages/repository_pg_test.go`, trên schema thật). Qua service/API: `TestQuizHomeworkMutationsOnPublishedReturnImmutableFirst` và `TestMutationsOnPublishedReturnImmutableFirst` (`stages/service_test.go`). Ở entity: `TestMutationsOnPublishedAreImmutable`. |
| `GET` lượt đang làm và kết quả quiz bài tập không chứa trường đúng/sai; có test trên JSON thô | API: `assertHidesAnswers` trên JSON đã marshal, dùng trong `TestPracticeAttemptLifecycle` và `TestAssessmentResultHidesAnswers` (`quizzes/service_integration_test.go`); danh sách cấm gồm `isCorrect`, `"correct"`, `explanation`, `"selected"`, `pointsAwarded`. Trình duyệt: `quiz.spec.ts` "bài tập: không response nào lộ đáp án…" ghi body mọi response API và khẳng định không có `"isCorrect"`, cả trước và sau khi nộp. |
| Hai request "bắt đầu lượt" đồng thời chỉ tạo một lượt; nộp sau `deadline_at + 30s` bị từ chối; lượt quá giờ được tự nộp với các câu đã lưu | `TestConcurrentStartsCreateOneAttempt`, `TestDeadlineGraceAndSubmitAfterTimeUp`, `TestExpiredAttemptIsAutoSubmittedOnRead`, `TestCloseExpired` (`quizzes/service_integration_test.go`). Thêm `TestGrantRacesStart` cho trường hợp cấp lượt chạy song song với bắt đầu lượt. |
| Kịch bản đầu cuối Playwright | `apps/web/e2e/learning-cycle.spec.ts`, 7 bước @serial, xanh:<br>1. Admin tạo chặng có quiz luyện tập, quiz bài tập, homework kèm tiêu chí rồi phát hành.<br>2. Admin gắn chặng vào khóa học mới, mở lớp do Bảo dạy và mời An.<br>3. Giảng viên đặt hạn nộp ở tab Bài tập.<br>4. Học viên làm quiz luyện tập, quiz bài tập và nộp homework.<br>5. Giảng viên chấm Đạt, được `nextQueueItem` chuyển sang xác nhận quiz, rồi về hàng chờ trống. Mailpit có đúng thư "Bài đã được chấm: … · Đạt" và "Giảng viên đã xác nhận kết quả: … · 100% · Đạt".<br>6. Học viên thấy "Hoàn thành chặng"; ô của học viên ở tab Tổng kết là "Hoàn thành".<br>7. Giảng viên viết nhận xét chặng; học viên thấy nhận xét đó. |
| Phiên bản chặng phát hành trước migration hiện đủ học liệu trong "Nội dung chặng"; % tiến độ không đổi | `TestStageUnitsBackfill` (`migrations/migrations_test.go`) và `TestMigratedVersionKeepsOrderInDefaultUnit` (`stages/repository_pg_test.go`). Thêm `TestCompletionAgreesAcrossSources` (`summary`): % ở mọi nguồn đều tính từ cùng định nghĩa hoàn thành. |
| `docs/architecture.md` và `docs/database.md` mô tả feature, route và bảng mới | Phase này đã rà và bổ sung (xem báo cáo triển khai). `docs/README.md` không đổi, vì mục lục và lệnh chạy không đổi. |

## 3. Đối chiếu FR với test

Tên test Go là tên hàm, đã kiểm có thật trong repo. Test web nằm dưới `apps/web/src/features/**`; spec Playwright nằm trong `apps/web/e2e/`.

### P0

| FR | API (Go) | Web (Vitest) | Playwright |
|---|---|---|---|
| FR-60 Soạn homework | `TestHomeworkUseCases`, `TestCloneDeepCopiesQuizAndHomeworkOnRealSchema` | `graded-lesson-editor.test.tsx` | `quiz-homework-authoring.spec.ts`, `learning-cycle.spec.ts` |
| FR-61 Hạn nộp theo lớp | `TestDeadlineSetClearAndLateFlag` | `homework-tab.test.tsx` | `homework.spec.ts`, `learning-cycle.spec.ts` |
| FR-62 Nộp bài | `TestSubmitRepoAndFiles`, `TestSubmitValidation`, `TestPassedLocksResubmitAndOnlyLatestIsGraded`, `TestConcurrentSubmitsAreSerialized` | `homework-lesson.test.tsx` | `homework.spec.ts`, `learning-cycle.spec.ts` |
| FR-63 Chấm bài | `TestGradeThresholdCompletionAndHistory`, `TestGradeEnqueuesEmailAndRecordsAudit`, `TestConcurrentGradesAreSerialized`, `TestConcurrentGradeAndResubmit` | `grade-submission-page.test.tsx` | `homework.spec.ts`, `learning-cycle.spec.ts` |
| FR-64 Hàng đợi chấm | `TestQueueScopeUnifiedOrderAndCounts`, `TestQueueFilters`, `TestNextSkipsHandledItemWithinScope`, `TestGradeReturnsNextQueueItem` | `grading-page.test.tsx` | `learning-cycle.spec.ts` (đi qua `nextQueueItem`) |
| FR-70 Soạn quiz | `TestQuestionUseCases`, `TestPublishBlocksIncompleteQuizOnRealSchema` | `graded-lesson-editor.test.tsx` | `quiz-homework-authoring.spec.ts` |
| FR-71 Làm bài | `TestConcurrentStartsCreateOneAttempt`, `TestDeadlineGraceAndSubmitAfterTimeUp`, `TestExpiredAttemptIsAutoSubmittedOnRead`, `TestCloseExpired`, `TestAssessmentResultHidesAnswers`, `TestScore*` | `quiz-lesson.test.tsx` | `quiz.spec.ts` |
| FR-74 Quyền soạn quiz | `TestQuizRoutesRequireAdmin`, `TestHandlerQuizRoutesNeedAdmin` | `middleware.test.tsx` | `quiz-homework-authoring.spec.ts` ("giảng viên mở đường dẫn trình soạn bị đưa về /teach") |
| FR-75 Xác nhận quiz bài tập | `TestAssessmentReviewLifecycle`, `TestReviewValidation`, `TestReviewRacesSubmit` | `review-attempt-page.test.tsx` | `quiz.spec.ts`, `learning-cycle.spec.ts` |
| FR-76 Nhận xét quiz học liệu | `TestPracticeReviewIsNeverQueued`, `TestReviewValidation` | `review-attempt-page.test.tsx` | `quiz.spec.ts` |
| FR-77 Làm quiz từng câu | (luồng API do FR-71 phủ) | `quiz-lesson.test.tsx`, `quiz-session.test.ts` | `quiz.spec.ts`, `learning-cycle.spec.ts` |
| FR-78 Bài học | `TestUnitLifecycle`, `TestUnitUseCases`, `TestCloneDeepCopiesUnits`, `TestMigratedVersionKeepsOrderInDefaultUnit`, `TestStageUnitsBackfill` | (trình soạn chặng) | `units.spec.ts` |
| FR-80 Tổng kết chặng của học viên | `TestBuildStage`, `TestStatus`, `TestCompletionAgreesAcrossSources` | `learn-summary-page.test.tsx` | `summary.spec.ts`, `learning-cycle.spec.ts` |
| FR-82 Báo cáo tổng kết lớp | `TestClassMatrixQueryCountIndependentOfMembers`, `TestSaveReviewRevisionsAndAudit` | `summary-tab.test.tsx` | `summary.spec.ts`, `learning-cycle.spec.ts` |
| FR-90 Mở rộng versioning | `TestCloneDeepCopiesUnits`, `TestCloneDeepCopiesQuizAndHomeworkOnRealSchema`, `TestImmutability_*`, `TestQuizHomeworkMutationsOnPublishedReturnImmutableFirst` | `graded-lesson-editor.test.tsx` | `units.spec.ts`, `quiz-homework-authoring.spec.ts`, `lesson-editor.spec.ts` |
| FR-91 Tiến độ | `TestCompletionAgreesAcrossSources`, `TestGradeThresholdCompletionAndHistory`, `TestConcurrentCompletionIsSerialized` | `lesson-page.test.tsx`, `quiz-lesson.test.tsx`, `homework-lesson.test.tsx` (nút "Đã học xong" chỉ ở video/markdown) | `learning.spec.ts`, `summary.spec.ts` |
| FR-92 Nhật ký thao tác bổ sung | `TestActionsMatchContract`, `TestActionLabelsCoverEveryAuditAction`, cùng các khẳng định audit trong `TestGradeEnqueuesEmailAndRecordsAudit`, `TestAssessmentReviewLifecycle`, `TestSaveReviewRevisionsAndAudit`, `TestDeadlineSetClearAndLateFlag`, `TestGrantLetsStudentStartAgain` | — | — |

### P1

| FR | API (Go) | Web (Vitest) | Playwright |
|---|---|---|---|
| FR-65 Theo dõi thời hạn chấm | `TestQueueOverdueBoundary`, `TestWithClockOverdueBoundary` | `grading-page.test.tsx` | — |
| FR-66 Nhắc giảng viên chấm trễ | `TestDigestStartsAtEightLocalOncePerDay`, `TestDigestDayUsesLocalCalendar`, `TestDigestOncePerTeacherPerLocalDay`, `TestDigestSkipsTeacherWithoutOverdueAndDisabledTeacher`, `TestConcurrentRunsEnqueueEachKeyOnce` | — | — |
| FR-67 Tồn đọng chấm cho Admin | `TestBuildBacklogCountsPerTeacherAndWholeQueue`, `TestBacklogAvgHoursOnFixedData` | `dashboard-page.test.tsx` | — |
| FR-68 Nhắc hạn nộp cho học viên | `TestReminderWindowIncludesExactlyOneDayAheadButNotNow`, `TestReminderPayloadAndKeyFollowDeadline`, `TestReminderTargetsOnlyUnsubmittedActiveMembers`, `TestReminderWindowBoundaries`, `TestConcurrentRunsEnqueueEachKeyOnce` | — | — |
| FR-72 Cấp thêm lượt | `TestGrantLetsStudentStartAgain`, `TestGrantRejections`, `TestGrantRacesStart` | `quiz-tab.test.tsx` (phần cấp lượt) | `quiz.spec.ts` ("cấp thêm 1 lượt") |
| FR-73 Thống kê câu hỏi | `TestQuestionStats`, `TestQuestionStatsOfClass` | `quiz-tab.test.ts` ("flags a question below 30%"), `quiz-tab.test.tsx` | — |
| FR-81 Tổng kết khóa của học viên | `TestBuildCourseSumsRequiredAcrossStages`, `TestLearnerSummaryHasNoCourseTotalScore` | `course-summary.test.ts`, `learn-summary-page.test.tsx` | — |

FR thiếu test tự động: không có.

Các việc nền email của FR-66 và FR-68 chỉ có test ở mức API (cửa sổ thời gian, khóa `scheduled_job_runs`, chạy đồng thời). Không có spec Playwright đọc thư nhắc từ Mailpit, vì không thể tua đồng hồ server từ trình duyệt. Seed có sẵn một homework hạn trong 24 giờ để chạy thử bằng tay qua `make worker`.

## 4. Seed

Đã đối chiếu với `prototype/seed.js` và phase file bước 1. Số bản ghi kỳ vọng nằm trong `expectedCounts()` của `apps/api/internal/seed/seed_integration_test.go`, và `TestCheckDataRejectsBadDrafts` cùng các bảng luật `checkData` chặn dữ liệu sai.

- Hàng chờ có cả mục quá 3 ngày và mục chưa quá hạn.
- Trạng thái bản nộp: passed 2, pending 5, rework 2, superseded 1, và có nộp muộn.
- Trạng thái lượt quiz: chờ xác nhận, đã xác nhận, bị thay thế, và một lượt tự nộp khi hết giờ (`auto_submitted`).
- Có homework hạn trong 24 giờ tới, có nhận xét chặng, và ma trận có đủ "Hoàn thành / Đang học / Cần làm lại / Quá hạn".
- Có bản nháp Go v2 nhân bản từ Go v1, như luật Clone.
