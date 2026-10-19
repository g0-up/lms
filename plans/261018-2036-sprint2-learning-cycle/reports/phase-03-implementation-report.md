# Báo cáo triển khai: Bài học (web)

- Phase: [phase-03-units-web.md](../phase-03-units-web.md)
- Trạng thái: hoàn thành, có lưu ý (Playwright chưa chạy được ở máy này)
- Ngày: 2026-10-18. Chưa commit.

## Kết quả

Trình soạn chặng nhóm học liệu theo bài học. Mỗi bài học có số thứ tự, tiêu đề, dòng "{n} học liệu · {m} bắt buộc", key và các nút "Thêm học liệu", chuyển lên, chuyển xuống, "Đổi tên bài học", "Xóa bài học". Mọi `aria-label` đều kèm tên bài học. Học liệu được đánh số `i.j` và di chuyển được qua ranh giới bài học: học liệu cuối đi xuống thành học liệu đầu của bài học sau, và ngược lại. Hai nút chỉ bị vô hiệu ở vị trí đầu và cuối của cả phiên bản.

Trang chặng có nút "Thêm bài học" và khối cảnh báo "Chưa phát hành được v{n}:", liệt kê lý do chặn phát hành đúng lời và thứ tự của server: chưa có bài học, hoặc `Bài học "{title}" chưa có học liệu.`. Hộp xác nhận xóa bài học nêu số học liệu sẽ bị xóa. Form học liệu (cả dialog lẫn trang soạn markdown) có select "Bài học" chọn sẵn bài học vừa bấm, kèm help text tạo/sửa theo spec.

Phía học viên:

- Thẻ chặng hiện "{u} bài học · {l} học liệu · x/y bắt buộc đã xong".
- Mỗi bài học có tiêu đề "Bài i.j · tiêu đề", dòng "x/y bắt buộc" hoặc "Không bắt buộc", và dấu "Hoàn thành".
- Trang học liệu có breadcrumb năm cấp. Cấp bài học dẫn tới `#unit-{id}` trên lộ trình, và trang cuộn tới nhóm đó.
- Danh sách bên cạnh nhóm theo `h3` bài học, giữ `aria-current`.
- Nút điều hướng là "Học liệu trước" và "Học liệu tiếp". Khi học liệu sau thuộc bài học khác, nút đổi thành "Sang bài học tiếp", với title là tên bài học đó.

## File đã sửa

`apps/web/src/features/stages/`:

- `model/schemas.ts` (`unitSchema`, `units`, `lessonSchema.unitId`), `model/order.ts` (`layoutOf`, `moveLessonAcross`, `moveUnit`, `applyLayout`), `model/lesson-form.ts` (`unitId` bắt buộc, help text, `unitMeta`, `requiredSplitLabel`), `model/stage-rules.ts` (`stagePublishBlockers`, `emptyUnitMessage`).
- Mới: `model/unit-form.ts` (+ test).
- `api/stages-api.ts`: `createUnit`, `renameUnit`, `removeUnit`, `reorderUnits`, `reorderLessons(vid, units)`. `api/msw-handlers.ts` có `twoUnitWorld()` và handler bài học.
- `hooks/use-lesson-mutations.ts`: optimistic theo bố cục hai cấp, rollback bằng invalidate. Mới: `hooks/use-unit-mutations.ts`.
- `components/lesson-list.tsx`, `list.tsx`, `order-buttons.tsx` (`labels`), `stage-actions-bar.tsx`, `lesson-form-dialog.tsx`.
- Mới: `components/unit-section.tsx`, `unit-form-dialog.tsx`, `unit-select.tsx`.
- `pages/stage-detail-page.tsx`, `pages/lesson-editor-page.tsx`.
- Test: `stage-detail-page.test.tsx`, `lesson-editor-page.test.tsx`, `schemas.test.ts`, `order.test.ts`, `lesson-form.test.ts`, `stage-rules.test.ts`, `test/golden.ts` (`stageVersionDraftTwoUnits`).

`apps/web/src/features/learning/`:

- `model/schemas.ts` (`roadmapUnitSchema`, `units` của chặng, `lessonPage.unit`/`stage.position`/`crossesUnit`/`nextUnitTitle`).
- `model/neighbors.ts`: `flattenLessons` đi theo bài học và gắn `unit` cho mỗi học liệu.
- `model/progress.ts`: `unitProgress`, cùng quy tắc với API.
- `hooks/use-my-class.ts`: `updateRoadmapLesson` cập nhật cả danh sách phẳng lẫn bài học.
- Mới: `components/unit-group.tsx`.
- Sửa: `stage-section.tsx`, `roadmap.tsx` (chỉ comment), `lesson-side-list.tsx`, `lesson-nav.tsx`, `complete-bar.tsx`, `pages/lesson-page.tsx`.
- Test: `lesson-page.test.tsx`, `learn-class-page.test.tsx`, `neighbors.test.ts`, `progress.test.ts`, `schemas.test.ts`.
- `test/golden.ts`: `roadmapTwoUnits` và `lessonMarkdownCrossing` (chặng Database tách thành "Nội dung chặng" + "Nâng cao").

`apps/web/e2e/`:

- Mới: `units.spec.ts`, gồm hai kịch bản:
  - Admin thêm hai bài học, phát hành bị chặn và thấy đúng tên bài học, chuyển hai học liệu qua ranh giới (kiểm cả DB), xóa bài học rỗng rồi phát hành.
  - Học viên thấy "Bài 1.1"/"Bài 1.2", breadcrumb và side list theo bài học. Học viên bấm "Sang bài học tiếp" (title "Nâng cao") và mở học liệu đầu của bài học sau. Trong cùng bài học, nút là "Học liệu tiếp". Breadcrumb bài học cuộn lộ trình tới nhóm đó.
- `content-versioning.spec.ts`:
  - E01 kiểm thêm việc clone chép `stage_units` cùng `unit_key`.
  - E02 bấm "Thêm học liệu" của bài học cuối. Seed DB nay có hai bài học nên nút không còn duy nhất.
- `lesson-editor.spec.ts`: `beforeAll` tạo bài học cho chặng E2EMD. E33 kiểm select "Bài học" được chọn sẵn.
- `learning.spec.ts`: E22 tạo bài học trước rồi gửi `unitId` khi tạo học liệu.
- `content-rules.spec.ts`: E06 dùng thông điệp mới "Chặng cần ít nhất một bài học trước khi phát hành." và kiểm thêm lỗi bài học rỗng có tên. Học liệu tạo kèm `unitId`.
- `support/seed.ts`: helper `createUnit`.
- `playwright.config.ts`: thêm `units` vào `E2E_SPECS`.

## Kiểm chứng

Các lệnh chạy trong `apps/web`:

| Lệnh | Kết quả |
|------|---------|
| `corepack pnpm lint` | sạch |
| `corepack pnpm typecheck` | sạch (gồm cả `e2e/**`) |
| `corepack pnpm test` | 60 file, 602 test pass (baseline 567, thêm 35) |
| `corepack pnpm lint:design` | 0 vi phạm |
| `corepack pnpm exec playwright test -c e2e/playwright.config.ts --project=e2e --list units content-rules content-versioning learning lesson-editor` | liệt kê được; `units.spec.ts` có 2 test |

**Playwright chưa chạy.** `globalSetup` và `seedReset` gọi `make e2e-seed-reset` trên container api của profile `full`. Brief cấm đụng container ngoài `lms-postgres-1`, và các cổng 9000/1025/8025 đang bị container khác giữ. Các spec mới và spec đã sửa mới chỉ được typecheck, lint và `--list`.

## Lệch so với plan

- API client dùng tên `removeUnit` thay cho `deleteUnit` để thống nhất với `removeLesson`.
- Có thêm hai file mới:
  - `unit-select.tsx`: dialog và trang soạn markdown dùng chung (DRY).
  - `unit-form.ts`: schema form tên bài học.
- Trang soạn markdown mở thẳng bằng URL (không qua dialog) thì không tự chọn bài học. Người soạn phải chọn, và lỗi "Chọn bài học cho học liệu." hiện trước lỗi nội dung.
- "Hoàn thành" ở lộ trình dùng `StatusDot variant="ok"` thay cho badge xanh. Design system chưa có biến thể badge ok, và tôi không sửa `shared/ui`.
- Ở đầu và cuối khóa, nút trước/tiếp vẫn giữ mẫu `aria-disabled` kèm title như MVP ("Đây là học liệu đầu tiên" / "Đây là học liệu cuối cùng"), chứ không ẩn như prototype.
- Tiến độ bài học tính ở client từ danh sách học liệu (cùng quy tắc với API) để tích optimistic hiện ngay. Test đối chiếu số liệu này với golden của API.
- Ngoài danh sách file của plan (vẫn trong lane `apps/web`), tôi sửa thêm:
  - `e2e/content-rules.spec.ts`: E06 vỡ vì chặng mới không có bài học.
  - `e2e/support/seed.ts` và `e2e/playwright.config.ts`. Biến `E2E_SPECS=` trong phần kiểm chứng của plan không được config đọc, nên tôi thêm `units` thẳng vào regex.
  - Hai lỗi lint nhỏ trong `stages/api/msw-handlers.ts` và `stages/model/order.ts`.
- Ảnh của `render-check` (admin-stage, learn-class, lesson) sẽ khác baseline. Spec này chỉ cảnh báo, và tôi không tạo lại ảnh.

## Câu hỏi mở

- Có muốn trang soạn markdown mở thẳng tự chọn bài học đầu tiên, thay vì bắt người soạn chọn?
- Có nên thêm biến thể badge "ok" vào design system để "Hoàn thành" khớp prototype hoàn toàn?
