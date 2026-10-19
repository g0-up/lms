---
title: "Sprint 2: chu trình học tập theo chặng hoàn tất"
date: 2026-10-19
summary: "16 phase Sprint 2 xong, mọi cổng xanh, chưa commit"
---

# Sprint 2: chu trình học tập theo chặng hoàn tất

## Kết quả

Đã hiện thực đủ 16 phase của plan `261018-2036-sprint2-learning-cycle` (spec-lms-v2 P0 + P1). Mọi cổng đều xanh trên cây cuối: API lint 0 issues, `make test` exit 0; web 1018/1018 test, lint:design 0 vi phạm, build OK; Playwright e2e + a11y 78 passed (stack build lúc 07:24, sau mọi sửa mã nguồn).

## Quyết định đáng nhớ

- Hạn mức upload của học viên tính theo cửa sổ thời gian: file `ready` trong 24 giờ (`UnattachedWindow`) và `pending` trong 15 phút (`UploadTTL`). Upload bỏ dở tự nhả chỗ, không cần job dọn hay endpoint DELETE. Người dùng chọn phương án này.
- Đếm câu hỏi và điểm rubric chuyển sang `LEFT JOIN LATERAL` để không quét cả bảng con khi chỉ lấy vài bài học.
- Chấm bài và xác nhận quiz invalidate cache tổng kết lớp (ma trận, chặng thành viên); nộp homework và nộp quiz invalidate tổng kết của học viên.

## Nợ kỹ thuật còn lại

- Chưa có index cho FK `quiz_attempt_answers.question_id`, `quiz_review_notes.question_id`, `grade_criterion_scores.criterion_id`. Hiện chỉ ảnh hưởng thao tác xóa câu hỏi hoặc tiêu chí ở bản nháp.
- Email nhắc FR-66 và FR-68 chỉ có test ở tầng API, vì Playwright không tua được đồng hồ server.
- Ví dụ `E2E_SPECS=... pnpm test:e2e` trong phase file không đúng: `E2E_SPECS` là regex trong `playwright.config.ts`, không phải biến môi trường.

> Historical work record — not durable authority. Prefer docs/specs/ADRs for current decisions.
