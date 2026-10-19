# Phase 13: Tổng kết (web), báo cáo triển khai

- Ngày: 2026-10-19
- Trạng thái: hoàn thành. `summary` và `a11y` chạy xanh 13/13 trên stack thật (web rebuild, api và worker giữ nguyên).
- Không commit.

## Kết quả

- Học viên có trang "Tổng kết khóa học" (`/learn/classes/:classId/summary`).
  - Thẻ "Toàn khóa": x/y học liệu bắt buộc và thanh %.
  - Mỗi chặng một thẻ với badge "Hoàn thành chặng" / "Có bài quá hạn" / "Có bài cần làm lại" / "Đang học".
  - Danh sách "Còn thiếu" (FR-80).
  - Học liệu nhóm theo bài học, mỗi dòng có chi tiết theo loại, "Không bắt buộc" và badge trạng thái.
  - Nhận xét của giảng viên, hoặc "Chưa có nhận xét của giảng viên cho chặng này.".
- Lộ trình có link "Tổng kết khóa học". Quiz và homework hiện badge trạng thái của API trên `lesson-row` và danh sách bên (FR-91).
- Admin (`/admin/classes/:id?tab=summary`) và giảng viên (`/teach/classes/:id?tab=summary`) có tab "Tổng kết" (FR-82).
  - Ma trận học viên × chặng với cột "Toàn khóa" và legend.
  - Bộ lọc theo URL, nút "Lọc" và "Xóa bộ lọc", câu đếm "x/y học viên · Bấm một ô…".
  - Mỗi ô là một nút: trạng thái ghi bằng chữ, `aria-label` có tên học viên, chặng, trạng thái, cờ quá hạn hoặc làm lại, và có nhận xét hay chưa.
  - Bảng nằm trong vùng cuộn ngang có `tabIndex=0` và `aria-label`.
- Drawer tổng kết chặng:
  - Header "{tên} · {chặng}", "Đã hoàn thành chặng" hoặc "Còn N học liệu bắt buộc", danh sách học liệu.
  - Form "Nhận xét tổng kết chặng": "Lưu nhận xét" / "Lưu thay đổi", dòng "Cập nhật … bởi … · đã sửa n lần".
  - Lưu xong thì invalidate drawer và ma trận. Lỗi 422 hiện ở field, lỗi khác (409, 403) hiện ở alert của form.
  - Khi 409, drawer tải lại nhưng giữ chữ đang gõ.
  - Khi 404, drawer hiện toast rồi đóng; focus quay về ô.

## File

### Feature mới `apps/web/src/features/summary/` (không import `classes`, `reports` hay `learning`)

- `api/summary-api.ts`, `api/msw-handlers.ts` (chỉ dùng cho test).
- `model/`:
  - `schemas.ts`, `matrix-filter.ts`, `item-detail.ts`, `stage-status.ts`, `review-form.ts`, `messages.ts`.
  - Test: `schemas`, `matrix-filter`, `item-detail`, `stage-status`, `review-form`.
- `hooks/`: `use-learner-summary.ts`, `use-class-matrix.ts`, `use-member-stage.ts`.
- `components/`:
  - `summary-tab.tsx` cùng `summary-tab.test.tsx`.
  - `summary-matrix.tsx`, `matrix-filter-bar.tsx`, `stage-summary-drawer.tsx`, `stage-review-form.tsx`, `stage-items.tsx`.
- `pages/learn-summary-page.tsx` cùng `learn-summary-page.test.tsx`.
- `routes.tsx`, `index.ts`.
- `test/golden.ts` (đọc golden JSON của API qua schema Zod), `test/render-summary.tsx`.

### Sửa file có sẵn

- `features/learning/`:
  - `model/schemas.ts`: `status` (label, tone) của học liệu trên lộ trình và trang học liệu.
  - `components/lesson-row.tsx` và `components/lesson-side-list.tsx`: badge trạng thái.
  - `pages/learn-class-page.tsx`: link "Tổng kết khóa học".
  - `test/golden.ts`: fixture `roadmapHomeworkRework`.
  - Test hồi quy trong `pages/learn-class-page.test.tsx` và `pages/lesson-page.test.tsx`.
- Tab "Tổng kết", đứng sau tab báo cáo; tab mặc định giữ nguyên:
  - `features/classes/model/class-tabs.ts`, `pages/class-detail-page.tsx` và test.
  - `features/reports/model/teach-class-tabs.ts`, `pages/teach-class-page.tsx` và test.
- `src/app/router.tsx`: route học viên `summary.routes`. Test mới `src/app/summary-routes.test.tsx`.
- `features/reports/components/report-panel.tsx` (lượt sửa sau E2E): badge ghi chú cho phép xuống dòng (`whitespace-normal`, icon `shrink-0`).
- `src/shared/ui/badge.tsx` (ngoài feature, sửa tối thiểu): thêm `StatusTone` và `ToneBadge`, map tone của API (`ok|info|warn|danger|optional`) sang variant của `Badge`. Learning và summary cùng dùng.
- `e2e/summary.spec.ts` (mới) và `e2e/playwright.config.ts` (thêm `summary` vào `E2E_SPECS`).

## Test đã thêm

- Model (40 test): bộ lọc URL, chi tiết từng loại học liệu, badge chặng và trạng thái ô, kiểm tra nhận xét theo rune.
- `summary-tab.test.tsx` (15 test):
  - Bảng, header, legend, vùng cuộn; ô ghi trạng thái bằng chữ và có `aria-label` đầy đủ.
  - Lọc từ URL (`overdue=1` cho 3/7). Lọc qua Select rồi bấm "Lọc", URL giữ `tab`. Lọc "Có bài cần làm lại". Stage lạ trong URL bị bỏ qua. Hai empty state.
  - Lỗi 403 có nút thử lại.
  - Drawer:
    - Mở bằng Enter, Escape trả focus về ô.
    - "Còn N học liệu bắt buộc".
    - Viết nhận xét: ô thành "đã nhận xét" nhờ invalidate. Sửa thì thấy "đã sửa 1 lần".
    - Kiểm tra phía client không gửi request. 422 hiện ở field, 409 giữ chữ đang gõ và tải lại.
    - 404: toast, đóng drawer, focus về ô. 403: alert có nút thử lại.
- `learn-summary-page.test.tsx` (8 test):
  - Tiêu đề, crumbs, document title, 16/25, các thẻ chặng và badge.
  - "Còn thiếu"; nhóm theo bài học; link học liệu; "Không bắt buộc".
  - Nhận xét, hoặc "Chưa có nhận xét…". Lớp nháp không có link.
  - 403 và 404 về `/learn` kèm toast. 500 hiện alert có nút thử lại.
- `app/summary-routes.test.tsx` (2 test): học viên mở được trang; giảng viên bị đưa về `/teach`.
- Learning (3 test): badge "Cần làm lại" trên hàng lộ trình (vẫn không có tick) và trên danh sách bên; link "Tổng kết khóa học".
- Tab: class-detail (mở tab tổng kết, lọc vẫn giữ tab) và teach-class (danh sách tab, mở tab tổng kết, 403 trong tab).

## Lệnh đã chạy

| Lệnh (trong `apps/web`) | Kết quả |
|---|---|
| `corepack pnpm lint` | sạch |
| `corepack pnpm typecheck` | sạch (gồm cả `e2e/**`) |
| `corepack pnpm test` | 95 file, 987 test xanh. Chạy 6 lần: 5 lần xanh hết, 1 lần lỗi 1 test ngoài lane (xem dưới). |
| `corepack pnpm lint:design` | 0 vi phạm |

Lượt đầu không chạy Playwright và không đụng container, theo chỉ đạo của lead.

### Lượt sửa sau E2E (lead chạy `summary` và `a11y`: 11 pass, 2 fail)

1. `summary.spec.ts`, học viên: badge "Cần làm lại" có hiện cạnh link bài tập. Locator sai vì tên truy cập của link mở đầu bằng nhãn icon ("Bài tập Bài tập: Đọc CSV…"), nên regex `^Bài tập: Đọc CSV` không khớp. Đã đổi sang khớp chuỗi con theo tên bài tập; assertion giữ nguyên.
2. `a11y.spec.ts` E26 reflow 320px, route `admin-class-report` (scrollWidth 459):
   - Đo từng phần tử ở 320px thì phần tràn là badge ghi chú của tab Tiến độ (`whitespace-nowrap`). Badge này mang câu dài "Video và bài đọc do học viên tự xác nhận; quiz và bài tập tính theo kết quả", do phase khác đổi.
   - Tab "Tổng kết" và tab Học viên đều vừa 320px, vì `TabBar` tự cuộn ngang.
   - Sửa: badge này được xuống dòng. Đo lại ra 320.

| Lệnh | Kết quả |
|---|---|
| rebuild riêng `web` (`up -d --build --wait --no-deps web`), không build api/worker | healthy |
| `playwright test --project=e2e --project=a11y summary a11y` | 13/13 pass (1.6m) |
| `corepack pnpm lint`, `typecheck`, `lint:design` | sạch, 0 vi phạm |
| `corepack pnpm test` | 984/987 pass. 3 test lỗi do golden API của lane khác (xem Việc còn mở). |
| `vitest run` summary, learning, reports, classes, app | 461/462. Test lỗi duy nhất là test tab quiz, cùng nguyên nhân. |

## Sai khác so với plan hoặc prototype

1. Tham số URL của bộ lọc là `stage`, `noReview`, `rework=1`, `overdue=1`, theo đúng mục Files của phase. Prototype dùng `incomplete` và `nocomment`.
2. Thêm `ToneBadge` vào `shared/ui/badge.tsx` (ngoài feature). Lý do: learning và summary cùng cần map tone của API sang badge, và một feature không được import feature khác.
3. Học liệu trong drawer nhóm theo bài học, giống trang học viên. Drawer của prototype là danh sách phẳng.
4. Trong drawer, học liệu không phải link. Nhân viên mở học liệu từ trình sửa khóa học, không qua link dành cho học viên.
5. `lesson-side-list.tsx` có thêm khoảng trắng giữa tiêu đề và badge trong link. Không có nó, tên truy cập bị dính liền thành "SELECTCần làm lại".
6. Test cũ "draft class không có link học liệu" nay liệt kê thêm link "Tổng kết khóa học". Trang tổng kết của lớp nháp vẫn mở được và không link học liệu. Assertion vẫn so khớp chính xác.

## Việc còn mở

- Ngoài lane: golden API `apps/api/internal/features/quizzes/testdata/class-quizzes.json` vừa được sinh lại (phase 14/15 sửa apps/api). Id chuẩn hóa của lượt làm bị lệch, nên 3 test web gắn cứng id cũ `…0014` bị đỏ:
  - `grading/components/quiz-tab.test.tsx`: 2 test.
  - `classes/pages/class-detail-page.test.tsx` › "opens the quiz tab after the homework tab with review links in the admin area".
  - Tôi không sửa. Cần lane đang đổi golden đó cập nhật test web hoặc golden.
- Test chập chờn ngoài lane (đã báo ở lượt đầu): `grading/pages/grade-submission-page.test.tsx`, `findByRole` timeout khi máy bận.
