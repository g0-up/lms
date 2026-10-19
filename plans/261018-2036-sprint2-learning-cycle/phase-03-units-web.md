---
phase: 3
title: "Bài học: web (trình soạn chặng, lộ trình, trang học liệu)"
status: completed
priority: P0
effort: "2d"
dependencies: [2]
---

# Phase 03: Bài học (web)

## Mục tiêu

Trình soạn chặng hiển thị học liệu theo bài học với các thao tác thêm, đổi tên, xóa và sắp xếp bài học. Học liệu được sắp xếp qua ranh giới bài học. Lộ trình học viên nhóm theo bài học, và trang học liệu có breadcrumb năm cấp, danh sách bên cạnh nhóm theo bài học và nút "Sang bài học tiếp".

## Context

- Spec FR-78. Prototype `pageStageDetail` 1054-1092, `unitForm` 1725, `lessonForm` 1730-1754, `moveLesson` 238-244, `pageLearnClass` 1548-1566 và `pageLesson` 1577-1604 ([scout-prototype](reports/scout-prototype.md) §1).
- Web: [scout-web](reports/scout-web.md) phần Stages và Learning.

## Files

- Sửa `src/features/stages/`:
  - `model/schemas.ts` (`unitSchema`, `stageVersionSchema.units`, `lessonSchema.unitId`), `model/order.ts` cùng `order.test.ts`: thêm `moveLessonAcross(layout, lessonId, dir)` và `moveUnit`.
  - `api/stages-api.ts`: `createUnit`, `renameUnit`, `deleteUnit`, `reorderUnits`, `reorderLessons(layout)`. Cập nhật `api/msw-handlers.ts`.
  - `hooks/use-lesson-mutations.ts` (optimistic theo bố cục bài học) và hook mới `hooks/use-unit-mutations.ts`.
  - `components/lesson-list.tsx`: render theo `UnitSection`.
  - Thêm `components/unit-section.tsx` và `components/unit-form-dialog.tsx`.
  - `pages/stage-detail-page.tsx`: nút "Thêm bài học", danh sách lý do chặn phát hành.
  - `model/lesson-form.ts` (trường `unitId` bắt buộc) và `pages/lesson-editor-page.tsx` (select "Bài học" cùng help text).
  - Test: `stage-detail-page.test.tsx`, `lesson-editor-page.test.tsx`, `model/stage-rules.ts` (lý do chặn phát hành ở client khớp server).
- Sửa `src/features/learning/`:
  - `model/schemas.ts` (`roadmapSchema.units`, `lessonPageSchema.unit`), `model/neighbors.ts` (`flattenLessons` theo bài học).
  - `components/roadmap.tsx` và `stage-section.tsx`: thêm `components/unit-group.tsx` với tiêu đề "Bài i.j · tiêu đề", "x/y bắt buộc" hoặc "Không bắt buộc", và badge "Hoàn thành".
  - `components/lesson-side-list.tsx` (nhóm `h3`, `aria-current`), `components/lesson-nav.tsx` ("Học liệu trước", "Học liệu tiếp", "Sang bài học tiếp" kèm title là tên bài học sau), `pages/lesson-page.tsx` (breadcrumb).
  - Test cho các component trên.
- Thêm `e2e/units.spec.ts`. Cập nhật `e2e/content-versioning.spec.ts` và `e2e/learning.spec.ts` theo bố cục mới.

## Các bước

1. Cập nhật schema zod theo golden JSON mới của phase 02 (`test/golden.ts` import thẳng từ `apps/api/.../testdata`), rồi chạy `pnpm typecheck` để lộ mọi chỗ vỡ.
2. Viết hàm thuần `moveLessonAcross` và test cho:
   - học liệu giữa bài học (hoán đổi);
   - học liệu đầu bài học đi lên (vào cuối bài học trước);
   - học liệu cuối bài học đi xuống (vào đầu bài học sau);
   - học liệu đầu bài học đầu tiên (nút bị vô hiệu).
3. Trình soạn: `UnitSection` hiển thị số thứ tự, tiêu đề, "{n} học liệu · {m} bắt buộc" và các nút "Thêm học liệu", lên, xuống, "Đổi tên bài học", "Xóa bài học". Mọi `aria-label` kèm tên bài học. Học liệu đánh số `i.j`. Copy khi trống lấy nguyên văn từ prototype.
4. Hộp xác nhận xóa bài học dùng `confirm-dialog` có sẵn:
   - Tiêu đề `Xóa bài học "{title}"?`, nút xác nhận "Xóa bài học".
   - Nội dung nêu số học liệu bị xóa và gợi ý chuyển học liệu đi trước nếu muốn giữ.
5. Form học liệu thêm select "Bài học", mặc định là bài học vừa bấm "Thêm học liệu". Help text: "Học liệu mới nằm cuối bài học đã chọn." khi tạo, "Đổi bài học sẽ đưa học liệu xuống cuối bài học mới." khi sửa.
6. Lộ trình và trang học liệu hiển thị theo copy của prototype. Thẻ chặng hiện số bài học, số học liệu và "x/y bắt buộc đã xong".
7. Thêm Playwright `units.spec.ts`:
   - Admin thêm hai bài học, chuyển học liệu qua ranh giới, phát hành bị chặn vì bài học rỗng (thấy đúng tên bài học), xóa bài học rỗng rồi phát hành.
   - Học viên thấy nhóm "Bài 1.1", bấm "Sang bài học tiếp" mở học liệu đầu của bài học sau.

## Kiểm chứng

```bash
pnpm -C apps/web lint typecheck test lint:design
E2E_SPECS='units|content-versioning|learning' pnpm -C apps/web test:e2e
```

## Tiêu chí hoàn thành

- [x] FR-78 phía UI có test: trình soạn, lý do chặn phát hành, lộ trình, breadcrumb, side list và điều hướng.
- [x] Phím tắt và vùng bấm của nút lên/xuống ≥ 44px. `lint:design` xanh.
- [x] Sau khi migrate, chặng MVP hiển thị một bài học "Nội dung chặng" trên mọi màn hình.

## Rủi ro

- Optimistic reorder trên bố cục hai cấp dễ lệch với server. Hook rollback bằng `invalidateQueries` khi lỗi, giống MVP.
