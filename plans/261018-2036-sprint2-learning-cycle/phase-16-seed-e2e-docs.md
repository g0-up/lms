---
phase: 16
title: "Seed, E2E đầu cuối, tài liệu"
status: completed
priority: P0
effort: "2d"
dependencies: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15]
---

# Phase 16: Seed, E2E đầu cuối, tài liệu

## Mục tiêu

- Seed đủ dữ liệu để mọi màn hình Sprint 2 có trạng thái thật, khớp `prototype/seed.js`.
- Một kịch bản Playwright đầu cuối chứng minh vòng học khép kín, theo [plan.md](plan.md) §6.
- Kiểm a11y mọi route mới.
- Tài liệu kỹ thuật mô tả feature, route, bảng và việc nền mới.
- Chạy kiểm chứng toàn bộ trước khi đóng plan.

## Context

- [plan.md](plan.md) §6 (tiêu chí chấp nhận toàn plan), §8 (API), §9 (routes).
- `prototype/seed.js`: bài học, quiz, homework, bản nộp, lượt quiz, nhận xét chặng (599-631), hạn nộp.
- Code: `internal/seed/data.go`, `apps/web/e2e/support/` (seed, db, auth, mailpit, media, ui), `e2e/a11y.spec.ts`, `playwright.config.ts` (`E2E_SPECS`), `docs/architecture.md`, `docs/database.md`, `docs/README.md`.

## Các bước

1. **Rà seed.** Mỗi phase backend đã thêm seed cho dữ liệu của nó. Phase này chỉ đối chiếu `seed/data.go` với `seed.js` và bù chỗ thiếu, sao cho:
   - Hàng chờ có ít nhất một bài nộp và một lượt quiz bài tập quá 3 ngày (FR-65), và có mục chưa quá hạn.
   - Có đủ trạng thái bản nộp: chờ chấm, Cần làm lại, Đạt, nộp muộn, bị thay thế.
   - Có đủ trạng thái lượt quiz: chờ xác nhận, đã xác nhận, bị thay thế, tự nộp khi hết giờ.
   - Có một homework có hạn trong 24 giờ tới, tính tương đối theo thời điểm seed, để chạy thử email nhắc.
   - Có nhận xét chặng, và có ô ma trận ở mỗi trạng thái "Hoàn thành / Đang học / Cần làm lại / Quá hạn".
   - Mốc thời gian tương đối tính theo `now`, như `seed.js`, để golden ổn định khi dùng `clock.Fake`.
   - `make seed-reset` chạy hai lần liên tiếp đều thành công, và golden của các phase trước vẫn khớp.
2. **Hỗ trợ E2E.**
   - `e2e/support/`: helper Mailpit tìm email theo tiêu đề. Không có helper tua đồng hồ server: `page.clock` chỉ tác động trình duyệt.
   - Thêm các spec mới vào ví dụ `E2E_SPECS` trong tài liệu chạy test.
3. **Kịch bản đầu cuối** `e2e/learning-cycle.spec.ts`, chạy tuần tự với các storage state sẵn có:
   1. Admin tạo bản nháp chặng, thêm bài học, thêm quiz luyện tập, quiz bài tập và homework có tiêu chí, rồi phát hành.
   2. Admin đặt lớp dùng phiên bản mới, hoặc dùng lớp seed đã gắn phiên bản đó.
   3. Giảng viên đặt hạn nộp cho homework.
   4. Học viên làm quiz luyện tập (thấy đáp án), làm quiz bài tập (chờ xác nhận), và nộp homework.
   5. Giảng viên mở "Chấm bài", chấm homework Đạt, rồi xác nhận lượt quiz kèm nhận xét, đi tuần tự qua `nextQueueItem`.
   6. Mailpit có email "Bài đã được chấm" và "Giảng viên đã xác nhận kết quả".
   7. Học viên mở "Tổng kết khóa học" và thấy chặng "Hoàn thành chặng". Giảng viên mở tab "Tổng kết" và thấy ô tương ứng là "Hoàn thành".
   8. Giảng viên viết nhận xét chặng; học viên thấy nhận xét đó.
4. **A11y.** `e2e/a11y.spec.ts` thêm các route sau, không có vi phạm axe mới:
   - `/admin/grading`, trang chấm, trang xác nhận quiz;
   - tab "Bài tập", "Quiz", "Tổng kết" của lớp, cả phía Admin và giảng viên;
   - trang homework, màn làm quiz, trang kết quả quiz, trang tổng kết của học viên;
   - trình soạn bài học, quiz và homework.
5. **Tài liệu** (chỉ sửa đúng phần thay đổi; đọc file trước khi sửa):
   - `docs/architecture.md`: các feature `homework`, `quizzes`, `grading`, `summary`; hướng import (`homework → grading`, `quizzes → grading`, `learning → summary`, `reports → grading`); định nghĩa hoàn thành ghi xuống `lesson_progress`; quy tắc lộ đáp án; việc định kỳ chạy trong `Cleanups` mỗi giờ (tự nộp lượt quá hạn, email tổng hợp, nhắc hạn) và khóa `scheduled_job_runs`; danh sách route web mới.
   - `docs/database.md`: các bảng và ràng buộc mới của migration 0010–0014, cách migration dữ liệu cũ vào bài học "Nội dung chặng", bảng `scheduled_job_runs`.
   - `docs/README.md`: chỉ cập nhật nếu mục lục hoặc lệnh chạy thay đổi.
   - Các phase trước có thể đã sửa tài liệu; phase này rà để không trùng lặp và không mâu thuẫn.
6. **Kiểm chứng toàn bộ.** Chạy đủ lệnh ở dưới. Sau đó đối chiếu từng mục ở [plan.md](plan.md) §6 và từng FR ở bảng phases với test tương ứng, rồi ghi kết quả vào `reports/`.

## Files

- Sửa `apps/api/internal/seed/data.go` và golden bị ảnh hưởng.
- Tạo `apps/web/e2e/learning-cycle.spec.ts`; sửa `e2e/a11y.spec.ts`, `e2e/support/*`.
- Sửa `docs/architecture.md`, `docs/database.md` và có thể `docs/README.md`.
- Tạo báo cáo kiểm chứng trong `plans/261018-2036-sprint2-learning-cycle/reports/`.

## Kiểm chứng

```bash
cd apps/api && make lint && TEST_DATABASE_URL=... make test && make seed-reset && make seed-reset
pnpm -C apps/web lint typecheck test lint:design build
E2E_SPECS='learning-cycle|homework|quiz|summary|units|quiz-homework-authoring|a11y|learning|reports|content-versioning' pnpm -C apps/web test:e2e
```

Tên spec trong `E2E_SPECS` phải khớp tên file mà các phase 03, 05, 08, 11 và 13 thực sự tạo.

## Tiêu chí hoàn thành

- [x] Mọi mục ở [plan.md](plan.md) §6 đều được đánh dấu, mỗi mục có bằng chứng (test hoặc lệnh) trong báo cáo kiểm chứng.
- [x] Mọi FR P0 và P1 ứng với ít nhất một test tự động.
- [x] Các spec Playwright có sẵn của MVP vẫn xanh.

## Rủi ro

- Kịch bản đầu cuối dài và dễ chập chờn: dùng locator theo role và nhãn, chờ theo response thay vì chờ theo thời gian, và không phụ thuộc đồng hồ quiz trong kịch bản đầu cuối.
- Golden lệch vì seed thay đổi: chạy `-update` một lần sau khi seed ổn định, và xem diff trước khi commit.

## Trạng thái triển khai

Hoàn thành 2026-10-19. Bằng chứng: [reports/phase-16-verification-report.md](reports/phase-16-verification-report.md), [reports/phase-16-implementation-report.md](reports/phase-16-implementation-report.md).
