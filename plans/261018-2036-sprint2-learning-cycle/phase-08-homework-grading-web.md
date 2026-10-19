---
phase: 8
title: "Homework và chấm bài: web"
status: completed
priority: P0
effort: "3d"
dependencies: [5, 7]
---

# Phase 08: Homework và chấm bài (web)

## Mục tiêu

Phase này làm bốn màn hình:

- Học viên: trang homework để xem đề, nộp, nộp lại, xem kết quả và lịch sử.
- Giảng viên và Admin: hàng chờ "Chấm bài", trang chấm theo tiêu chí, tab "Bài tập" của lớp (đặt hạn nộp, xem trạng thái học viên).

## Context

- Spec FR-61, FR-62, FR-63, FR-64.
- Prototype ([scout-prototype](reports/scout-prototype.md) §4, §5, §6): `homeworkView` 1694-1722, `pageGrading` 1399-1431, `pageGradeSubmission` 1433-1467, `renderHomeworkTab` 1272-1288.
- API của phase 06 và 07, cùng golden JSON tương ứng.

## Files

- Học viên (`src/features/learning/`):
  - `api/homework-api.ts` và `msw-handlers.ts`.
  - `hooks/use-submissions.ts` và `hooks/use-submit-homework.ts`.
  - `model/submission-form.ts` cùng test: zod gồm regex repo, ràng buộc commit đi kèm repo, tối đa 5 file, đuôi và kích thước file, copy lỗi theo prototype.
  - `components/homework-view.tsx`: thẻ đề bài, chip "Tài liệu kèm theo", danh sách tiêu chí.
  - `components/submission-form.tsx`: tải file qua `@/shared/api/upload-media` kind `document`.
  - `components/submission-feedback.tsx` và `components/submission-history.tsx`.
  - Sửa `components/lesson-viewer.tsx` để render theo loại.
  - Quy tắc tích theo loại (FR-91) đặt trong `model/tick-rule.ts`: thêm `tickable(type)` đúng với `video` và `markdown`. `complete-bar.tsx` chỉ ẩn nút "Đã học xong" khi `!tickable`, vẫn giữ `LessonNav` (`complete-bar.tsx:33`). `lesson-row.tsx` (`TickCheckbox` ở dòng 41) không render ô tích với quiz và homework. Test trong `tick-rule.test.ts`, `complete-bar` và `lesson-row`.
  - Mở rộng enum: `lessonTypeSchema` ở `learning/model/schemas.ts:10` thành `["markdown","video","quiz","homework"]`; `progress.ts` `lessonSub` (khoảng dòng 28) thêm "Quiz" và "Bài tập" thay vì rơi vào "Bài đọc". Test hai nhánh mới.
- Chấm (feature mới `src/features/grading/`):
  - `api/grading-api.ts` và `msw-handlers.ts`.
  - `model/schemas.ts`, `model/grade-form.ts` cùng test: tổng điểm, %, Đạt hoặc Cần làm lại, số tiêu chí chưa nhập.
  - `model/queue-filter.ts` cùng test: tham số URL `type`, `class`, `lesson`, `overdue`.
  - `pages/grading-page.tsx`, `pages/grade-submission-page.tsx`.
  - `components/queue-table.tsx`, `queue-filter-bar.tsx`, `recent-list.tsx`, `submission-facts.tsx`, `grade-form.tsx`, `grade-history.tsx`.
  - `routes.tsx`, `index.ts`, `test/render-grading.tsx`, `test/golden.ts`.
- Tab "Bài tập" của lớp:
  - Component nằm trong `src/features/grading/`: `components/homework-tab.tsx`, `components/deadline-dialog.tsx`, cùng API `classHomework`, `setDeadline`, `clearDeadline` trong `grading-api.ts`. `HomeworkTab({classId, basePath})` được export qua `grading/index.ts`.
  - Lý do đặt ở `grading`: `classes` đã import `ReportPanel` từ `reports`. Nếu tab nằm trong `classes` thì `reports` (trang lớp của giảng viên) phải import ngược `classes` và tạo vòng. `grading` không import `classes` hay `reports`, nên cả hai cùng import nó được.
  - Thanh tab dùng chung: chuyển `classes/components/class-tabs.tsx` sang `src/shared/ui/tab-bar.tsx` (việc nền), nhận `tabs[{key, label}]`, `active` và `hrefFor(key)` qua props. `classes` và `reports` cùng dùng, nên không có hai bản markup và không có import vòng.
  - `classes/model/class-tabs.ts`: thêm `homework` (label "Bài tập"); sửa `class-tabs.test.ts:5`. Phase 11 thêm `quiz`, phase 13 thêm `summary`. Mặc định tab Admin giữ nguyên.
  - Sửa `classes/pages/class-detail-page.tsx` để render tab mới qua `TabBar`.
  - `reports/pages/teach-class-page.tsx` hiện chỉ render `ReportPanel` (dòng 67). Thêm `reports/model/teach-class-tabs.ts` (`report | homework`, mặc định `report` như prototype `app.js:1394`, đọc `?tab=`) cùng test, và render `TabBar`. Phase 11 thêm `quiz`, phase 13 thêm `summary`; mặc định luôn là `report`, nên `report-panel.tsx:75,109`, `e2e/reports.spec.ts:149-151` và `render-check.spec.ts:88` không phải đổi.
- Báo cáo (`src/features/reports/`):
  - `model/schemas.ts:111`: `type` của học liệu nhận thêm `quiz` và `homework`. `selfReported` giữ `z.literal(true)`; server không đổi.
  - `components/report-panel.tsx:90` đổi câu ghi chú thành nguyên văn prototype (`app.js:1368`): "Video và bài đọc do học viên tự xác nhận; quiz và bài tập tính theo kết quả". Sửa hai assertion ở `report-panel.test.tsx:49,233`.
- App (việc nền, [plan.md](plan.md) §4 mục 10):
  - `src/app/router.tsx`: thêm `/admin/grading`, `/admin/grading/:submissionId`, `/teach/grading`, `/teach/grading/:submissionId`.
  - `src/shared/layout/nav.tsx`: NAV giảng viên và Admin thêm link thường "Chấm bài", không badge, như prototype (`app.js:853-854`).
  - Sửa test của nav nếu có snapshot danh sách link.
- Thêm `e2e/homework.spec.ts`.

## Các bước

1. Viết schema zod từ golden, rồi viết các hàm thuần (`submission-form`, `grade-form`, `queue-filter`) cùng test.
2. Trang homework của học viên:
   - Copy, cảnh báo và trạng thái khóa lấy theo prototype ("Lần nộp #n đang chờ chấm…", "Đã quá hạn nộp…", "Bài đã được chấm Đạt nên không cần nộp lại.").
   - Help dưới trường repo theo OQ-1 đã chốt: "GitHub, GitLab hoặc Bitbucket. Repo phải để public để giảng viên mở được." Đây là chỗ duy nhất lệch copy prototype (`app.js:1707`), vì người dùng đã chọn bắt buộc public. Test component khẳng định câu này.
   - Nhãn lịch sử: "Đạt", "Cần làm lại", "Chờ chấm", "Đã thay bằng lần nộp sau".
   - File chỉ là link tải về, không có preview.
3. Hàng chờ:
   - Lede, badge "N mục quá 3 ngày", bộ lọc, câu đếm và bảng với các cột và nhãn của prototype.
   - Dòng quá hạn có viền hoặc nền dùng token màu sẵn có, không dùng hex thô.
   - Trạng thái trống: "Không còn bài chờ xử lý" và "Không có mục khớp bộ lọc".
4. Trang chấm:
   - Hai cột: facts và lịch sử bên trái, form chấm bên phải.
   - Tổng điểm cập nhật trực tiếp, đặt trong `aria-live="polite"`.
   - Fieldset bị khóa khi bản nộp không phải mới nhất, kèm link tới bản mới.
   - Sau khi lưu: lần chấm đầu chuyển sang `nextQueueItem` với toast "Chuyển sang bài tiếp theo.", hoặc "Đã chấm hết hàng chờ.". Sửa điểm thì ở lại trang.
   - Lịch sử chấm hiện điểm từng tiêu chí của mỗi lần chấm.
5. Tab "Bài tập" của lớp:
   - Mỗi homework một thẻ, gồm thống kê và bảng học viên.
   - Dialog "Đặt hạn nộp" hoặc "Đổi hạn nộp" dùng `input type="datetime-local"` theo giờ Việt Nam; gửi ISO có offset.
   - Lớp `ended` vô hiệu nút đặt hạn và có tooltip lý do.
6. Playwright `homework.spec.ts`:
   - Học viên nộp repo kèm một file.
   - Nộp lại khi đang chờ chấm: bản cũ chuyển sang "Đã thay bằng lần nộp sau".
   - Giảng viên đặt hạn nộp; chấm từ hàng chờ, được Cần làm lại; học viên nộp lại; giảng viên chấm Đạt.
   - Học viên thấy điểm từng tiêu chí và không còn nút nộp.
   - Mailpit có hai email "Bài đã được chấm".
   - Tải file trả header `content-disposition: attachment`.

## Kiểm chứng

```bash
pnpm -C apps/web lint typecheck test lint:design
E2E_SPECS='homework|a11y' pnpm -C apps/web test:e2e
```

## Tiêu chí hoàn thành

- [x] FR-61 đến FR-64 phía UI có test.
- [x] `a11y.spec.ts` được mở rộng cho `/teach/grading` và trang homework, không có vi phạm axe mới. (Spec đã viết, chưa chạy trên stack: stack e2e còn image cũ.)
- [x] Feature `grading` chỉ import feature khác qua `index.ts`.

## Rủi ro

- Giờ địa phương của hạn nộp: hiển thị và nhập theo `Asia/Ho_Chi_Minh`. `day.ts` hiện chỉ có trong `features/classes/model` và `features/reports/model`, không có ở `shared`; thêm `grading/model/deadline-time.ts` nhỏ (đổi `datetime-local` ↔ ISO có offset `+07:00`) cùng test giờ biên, không import chéo feature.
