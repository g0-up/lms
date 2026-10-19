---
phase: 13
title: "Tổng kết: web"
status: completed
priority: P0
effort: "2d"
dependencies: [11, 12]
---

# Phase 13: Tổng kết (web)

## Mục tiêu

- Học viên có trang "Tổng kết khóa học" và nhãn trạng thái quiz và homework trên lộ trình.
- Giảng viên và Admin có tab "Tổng kết" của lớp: ma trận học viên × chặng, bộ lọc, và drawer tổng kết chặng để viết hoặc sửa nhận xét.

## Context

- Spec FR-80, FR-82, FR-91.
- Prototype ([scout-prototype](reports/scout-prototype.md) §6, §7): `renderSummaryTab` 1210-1241, `stageSummaryDrawer` 1255-1271, `lessonDetail` 1243-1254, `pageLearnSummary` 1567-1576.
- API và golden của phase 12. Drawer có sẵn: `reports/components/student-detail-drawer.tsx`.

## Files

- Feature mới `src/features/summary/` (không import `classes` hay `reports`, nên cả hai cùng import được):
  - `api/summary-api.ts`, `msw-handlers.ts`, `model/schemas.ts`.
  - `model/matrix-filter.ts` cùng test: tham số URL `stage`, `noReview`, `rework`, `overdue`, câu đếm "x/y học viên".
  - `model/item-detail.ts` cùng test: dòng chi tiết từng loại học liệu theo `lessonDetail` của prototype.
  - `components/summary-tab.tsx` (export `SummaryTab({classId, basePath})`).
  - `components/summary-matrix.tsx`: bảng có `<th scope>`, ô là nút mở drawer, legend "Hoàn thành / Đang học / Cần làm lại / Quá hạn".
  - `components/matrix-filter-bar.tsx`.
  - `components/stage-summary-drawer.tsx`, `stage-review-form.tsx`, `stage-items.tsx` (dùng chung với trang học viên).
  - `pages/learn-summary-page.tsx` và `routes.tsx` (route học viên).
  - `index.ts`.
- `src/features/learning/`: `model/schemas.ts` thêm `status` của học liệu; `components/lesson-row.tsx` và `lesson-side-list.tsx` hiện badge trạng thái; `pages/learn-class-page.tsx` thêm link "Tổng kết khóa học".
- `classes/model/class-tabs.ts` và `reports/model/teach-class-tabs.ts` thêm `summary` (label "Tổng kết"), render qua `TabBar`. Mặc định giữ nguyên: giảng viên `report` như prototype (`app.js:1394`), Admin như hiện có.
- `src/app/router.tsx`: thêm `/learn/classes/:classId/summary` (việc nền, [plan.md](plan.md) §4 mục 10; phase 08, 11 và 13 sửa file này tuần tự).
- Thêm `e2e/summary.spec.ts`.

## Các bước

1. Schema từ golden, rồi viết `matrix-filter` và `item-detail` cùng test.
2. Trang học viên:
   - Thẻ "Toàn khóa" với "x/y học liệu bắt buộc" (chỉ %, không điểm tổng).
   - Mỗi chặng một thẻ với badge "Hoàn thành chặng" / "Có bài quá hạn" / "Có bài cần làm lại" / "Đang học".
   - Học liệu nhóm theo bài học, mỗi dòng có chi tiết theo loại, "không bắt buộc" và badge trạng thái.
   - Danh sách "Còn thiếu" khi chặng chưa hoàn thành. Prototype chỉ có số đếm, nhưng FR-80 yêu cầu danh sách.
   - Hộp "Nhận xét của giảng viên" hoặc "Chưa có nhận xét của giảng viên cho chặng này.".
3. Tab "Tổng kết":
   - Lọc theo URL; nút "Lọc" và "Xóa bộ lọc"; câu "x/y học viên · Bấm một ô để xem tổng kết chặng và viết nhận xét".
   - Ô ma trận có trạng thái bằng chữ, không chỉ màu (a11y). Nhãn `aria-label` của ô gồm tên học viên, chặng và trạng thái.
   - Cột "Toàn khóa".
4. Drawer:
   - Header "{tên} · {chặng}", "Đã hoàn thành chặng" hoặc "Còn N học liệu bắt buộc", danh sách học liệu.
   - Form "Nhận xét tổng kết chặng" với help "Học viên thấy nhận xét này trong trang tổng kết của mình."; nút "Lưu nhận xét" hoặc "Lưu thay đổi"; dòng "đã sửa n lần" khi `revision > 1`.
   - Lưu xong thì invalidate ma trận để cờ "chưa có nhận xét" cập nhật.
5. Lộ trình: badge trạng thái cho quiz và homework trên `lesson-row` và danh sách bên. Nút "Đã học xong" chỉ còn với video và markdown (đã làm ở phase 08; ở đây thêm test hồi quy).
6. Playwright `summary.spec.ts`:
   - Giảng viên lọc "Có bài cần làm lại", mở ô, viết nhận xét, sửa nhận xét (thấy "đã sửa 1 lần").
   - Học viên mở "Tổng kết khóa học" và thấy nhận xét.
   - Axe trên tab và trang.

## Kiểm chứng

```bash
pnpm -C apps/web lint typecheck test lint:design
E2E_SPECS='summary|a11y' pnpm -C apps/web test:e2e
```

## Tiêu chí hoàn thành

- [x] FR-80 và FR-82 phía UI có test; FR-91 có test hồi quy UI. (Vitest: trang học viên, tab, ma trận, bộ lọc, drawer, form nhận xét; badge trạng thái trên `lesson-row` và danh sách bên; homework và quiz không có nút tick.)
- [x] Ma trận dùng được bằng bàn phím và trình đọc màn hình (axe sạch, focus quay về ô sau khi đóng drawer). (Vitest phủ phần bàn phím, tên truy cập của ô và focus quay về ô. `e2e/summary.spec.ts` chạy xanh trên stack thật, gồm axe trên tab và trang, mở ô bằng Enter, focus về ô sau Escape. `a11y.spec.ts` xanh 11/11.)

## Rủi ro

- Ma trận rộng khi khóa có nhiều chặng: bọc bảng trong vùng cuộn ngang có `tabIndex=0` và `aria-label`, giống bảng báo cáo hiện có.
