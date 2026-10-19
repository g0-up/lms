# Phase 16: Seed, E2E đầu cuối, tài liệu

- Ngày: 2026-10-19
- Trạng thái: hoàn thành. Toàn bộ lệnh kiểm chứng xanh; chi tiết ở [phase-16-verification-report.md](phase-16-verification-report.md).
- Không commit. Không có migration. Chỉ đụng dev DB `lms` và `lms_test` trên 5433 (đã `pg_dump` dev DB vào scratchpad trước khi `seed-reset`).

## Kết quả

1. **Seed** (`apps/api/internal/seed/`): bù các chỗ còn thiếu so với `prototype/seed.js`.
   - Bản nháp Go v2 nhân bản từ Go v1 theo luật Clone. Dùng `stageDrafts` và `insertDraft`, kèm audit `stage_version.cloned`. Bài học và học liệu có id mới, giữ khóa và nội dung; video dùng chung media. `checkDrafts` kiểm: nguồn đã phát hành, mỗi chặng tối đa một bản nháp, số phiên bản kế tiếp, mốc thời gian hợp lệ.
   - Hai học liệu "Hello World từng bước" (markdown) và quiz luyện tập "Lệnh go và cấu trúc chương trình" trong bài học đầu của Go v1.
   - Bản nộp `ds-hw` của Nhàn bị chấm Cần làm lại và chưa nộp lại, để có ô "Cần làm lại" trong ma trận basic02.
   - Một lượt quiz bài tập tự nộp khi hết giờ (`auto_submitted`), đã được xác nhận. `checkData` kiểm lượt tự nộp phải là lượt assessment nộp đúng hạn của lượt.
   - `--upload-sample` tải video mẫu, và thêm một file mẫu đúng loại cho mỗi tài liệu seed (file kèm đề bài, file học viên nộp), để bấm tải được trên dev.
   - Test: `seed_test.go` (gồm `TestCheckDataRejectsBadDrafts` và các ca mới của luật quiz), `seed_integration_test.go` (`expectedCounts` tính cả bản nháp, khẳng định bản nháp chép trung thực Go v1, lượt tự nộp), `upload_sample_test.go`, `testdata/seed_markdown.json`.
2. **Hỗ trợ E2E** (`apps/web/e2e/support/`):
   - `mailpit.ts`: thêm `waitForSubject`, chờ thư mới có đúng tiêu đề.
   - `seed.ts`: `createClass` nhận giảng viên phụ trách.
   - `playwright.config.ts`: regex `E2E_SPECS` thêm `homework|quiz|summary|learning-cycle`.
3. **Kịch bản đầu cuối** `apps/web/e2e/learning-cycle.spec.ts`: 7 bước @serial theo plan.md §6, xanh.
4. **A11y** `apps/web/e2e/a11y.spec.ts`: axe trên 17 route mới (hàng chờ, trang chấm, trang xác nhận quiz, các tab Bài tập/Quiz/Tổng kết phía Admin và giảng viên, trang homework, kết quả quiz, tổng kết của học viên, trình soạn bài học/quiz/homework), cộng một test riêng cho màn làm quiz. Xanh.
5. **Tài liệu**:
   - `docs/architecture.md`: hướng import giữa các feature của vòng học; mục việc định kỳ của worker (thứ tự `Cleanups`, một cleanup lỗi không chặn cleanup sau); "chín feature"; bảng route có `grading.routes`, `grading.teachRoutes`, `summary.routes`; danh sách route web mới.
   - `docs/database.md`: số bản ghi seed, bản nháp Go v2, cờ `--upload-sample`.
   - Các phase trước đã viết sẵn định nghĩa hoàn thành, quy tắc lộ đáp án, khóa `scheduled_job_runs` và migration 0010–0014. Phase này chỉ rà để không trùng lặp và không mâu thuẫn. `docs/README.md` không đổi.

## File

Trong danh sách của phase:

- `apps/api/internal/seed/`: `data.go`, `markdown.go`, `seed.go`, `seed_test.go`, `seed_integration_test.go`, `upload_sample.go`, `upload_sample_test.go`, `testdata/seed_markdown.json`.
- Golden bị seed ảnh hưởng, tạo lại bằng `-update` rồi đã xem diff: `quizzes/testdata/class-quizzes.json`, `reports/testdata/{dashboard,member-report,class-report}.json`, `summary/testdata/learner-summary.json`.
- `apps/web/e2e/`: `learning-cycle.spec.ts` (mới), `a11y.spec.ts`, `support/mailpit.ts`, `support/seed.ts`, `playwright.config.ts`.
- `docs/architecture.md`, `docs/database.md`.
- `plans/261018-2036-sprint2-learning-cycle/`: phase file, `plan.md` (chỉ tick §6), hai báo cáo này.

Ngoài danh sách. Các sửa đổi này là tối thiểu và đều do seed đổi dữ liệu:

- `apps/web/src/features/grading/components/quiz-tab.test.tsx`: `lessonId` cứng `…057` thành `…061`. Seed thêm học liệu nên UUID dịch chỗ, và `…061` là id của "Kiểm tra cuối chặng Go" trong golden mới.
- `apps/api/internal/features/grading/service_integration_test.go`: danh sách gần đây của Bảo có thêm bản nộp Cần làm lại của Nhàn và lượt `db-test` của Phong.
- `apps/api/internal/features/reports/repository_pg_test.go`: 28→30 học liệu, `not_opened` 8→10, theo hai học liệu mới của Go v1.
- `apps/web/e2e/lesson-editor.spec.ts`: dùng bản nháp Go v2 có sẵn trong seed, vì Clone sẽ bị từ chối khi chặng đã có bản nháp. Bỏ một khẳng định trùng.
- `apps/web/e2e/summary.spec.ts`: sửa câu chú thích đầu file cho đúng dữ liệu seed.

`git diff` còn hiện `content-rules.spec.ts`, `content-versioning.spec.ts`, `learning.spec.ts`, `reports.spec.ts` và `support/db.ts`. Phase này không sửa các file đó; chúng thuộc các phase trước của sprint (cả sprint chưa commit). `reports.spec.ts` vẫn xanh với seed mới, nên không cần suy lại phần trăm.

## Kiểm chứng

- API: lint (golangci-lint v2.14.0 ghim qua `go run`) `0 issues`; `make test` `exit=0`.
- `make seed-reset` hai lần liên tiếp: đều thành công, số bản ghi giống nhau.
- Web: lint, typecheck sạch; Vitest 98 file và 1018 test pass; `lint:design` 0 vi phạm; build xong.
- Playwright e2e và a11y: `78 passed`.

## Vấn đề và lệch so với phase file

- Ví dụ `E2E_SPECS='…' pnpm test:e2e` trong phase file không chạy được như viết. `E2E_SPECS` là hằng regex trong `playwright.config.ts`, không phải biến môi trường, và không tài liệu nào nhắc tới. Phase này cập nhật regex thay cho ví dụ.
- `~/go/bin/golangci-lint` cục bộ build bằng go1.26, nên không lint được module go1.27. Phải truyền `GOLANGCI_LINT` ghim v2.14.0, như phase 14.
- Khi Vitest chạy song song với bộ E2E (máy tải nặng), vài test `findBy` vượt timeout 1 s. Chạy lại khi E2E đã xong thì toàn bộ 1018 test pass. Không sửa hay nới test nào.
- Dev DB `lms` dùng chung với stack E2E. Sau `seed-reset` thủ công, storage state cũ mất hiệu lực, nhưng `globalSetup` của Playwright tự seed lại và đăng nhập lại.

## Bước tiếp theo

- Controller cập nhật cột trạng thái ở bảng phases của `plan.md`. Phase này không sửa bảng đó.
- Commit theo phạm vi do team-lead quyết.
