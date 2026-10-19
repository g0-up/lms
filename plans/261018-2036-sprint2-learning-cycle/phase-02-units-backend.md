---
phase: 2
title: "Bài học: backend (soạn, nhân bản, lộ trình)"
status: completed
priority: P0
effort: "2.5d"
dependencies: [1]
---

# Phase 02: Bài học (backend)

## Mục tiêu

Admin thêm, đổi tên, xóa và sắp xếp bài học trong bản nháp, đưa học liệu vào bài học và chuyển học liệu giữa các bài học. Phát hành bị chặn khi chưa có bài học hoặc có bài học rỗng. Nhân bản giữ `unit_key`. Lộ trình và trang học liệu của học viên nhóm theo bài học; nút trước và tiếp đi qua ranh giới bài học.

## Context

- Spec: FR-78, FR-90 (phần bài học), mục 5 "Mô hình chặng", trường hợp biên "Admin xóa bài học còn học liệu" và "phát hành chặng có bài học chưa có học liệu".
- Code: `internal/features/stages/{entity,service,repository_pg,dto,handler}.go` ([scout-api](reports/scout-api.md) §2), `internal/features/learning/{roadmap,service,repository_pg,dto}.go` (§3).
- Prototype: `moveLesson` 238-244, `deleteUnit` 258-260, `stagePublishBlockers` 696-697, `pageLesson` 1577-1604 ([scout-prototype](reports/scout-prototype.md) §1).

## Yêu cầu

1. `Unit` thuộc aggregate `StageVersion` với các trường `id`, `key` (`domain.LessonKey`, sinh bằng `uniqueKey(title)` như học liệu), `title` (trim, 1–120 ký tự, không trùng trong phiên bản và so sánh không phân biệt hoa thường) và `position`.
2. Bất biến của aggregate: `lessons` luôn xếp theo (vị trí bài học, vị trí trong bài học), và `position` của học liệu là thứ tự phẳng 1..n. Hàm `renumber()` duy nhất khôi phục bất biến sau mọi thao tác.
3. Thao tác domain, mọi thao tác gọi `ensureMutable()`:
   - `AddUnit(id, title)` thêm vào cuối. Lỗi `ErrUnitTitleTaken` ("Tên bài học đã có trong phiên bản này.", `CONFLICT`).
   - `RenameUnit(id, title)`.
   - `RemoveUnit(id)` xóa cả học liệu trong bài học và trả về số học liệu bị xóa để UI báo cho người soạn.
   - `ReorderUnits(ids)`: tập id phải khớp đủ, nếu không trả `ErrReorderMismatch` có sẵn (`stages/entity.go:25`).
   - `AddLesson(id, unitID, …)` thêm vào cuối bài học được chọn. Thiếu hoặc sai `unitID` trả `ErrUnitNotFound` (`VALIDATION_FAILED` "Chọn bài học cho học liệu.").
   - `MoveLesson(lessonID, unitID)` đưa học liệu xuống cuối bài học mới. `UpdateLesson` gọi hàm này khi `unitId` đổi.
   - `ReorderLessons(layout []UnitLessons)` nhận bố cục đầy đủ `{unitId, lessonIds}`. Mọi bài học và học liệu phải xuất hiện đúng một lần.
4. `Publish` bổ sung kiểm tra theo thứ tự của prototype:
   - Thay `ErrNoLessons` bằng `ErrNoUnits` "Chặng cần ít nhất một bài học trước khi phát hành.".
   - Bài học rỗng trả `ErrEmptyUnit(title)` với thông điệp `Bài học "X" chưa có học liệu.`.
   - Cả hai lỗi dùng `domain.ErrInvalid` như hiện tại.
5. `CloneAsDraft` tạo bài học mới (id mới, giữ key, title, position) và ánh xạ `unitID` cũ sang mới cho từng học liệu.
6. Repository `stages`:
   - `Create` chèn `stage_units` trước `lessons`.
   - Gỡ bản vá tối thiểu của phase 01: `CreateStage` trở lại tạo bản nháp v1 không có bài học nào, như prototype (`app.js:165`, `units: []`); `SaveDraft` không còn tự gán học liệu vào bài học đầu tiên, vì `AddLesson` bắt buộc `unitID`. `Clone` giữ việc sao chép bài học.
   - `SaveDraft` diff `stage_units` giống học liệu: thêm, sửa tên hoặc vị trí, xóa. Mọi lệnh đều `WHERE stage_version_id=$ AND EXISTS (… status='draft')`, và `SET CONSTRAINTS uq_stage_units_version_position, uq_lessons_version_position DEFERRED`.
   - Thứ tự ghi: chèn bài học mới, cập nhật học liệu (kể cả `unit_id`), xóa học liệu, rồi xóa bài học.
   - `ByID` và `ByIDForUpdate` nạp bài học kèm học liệu.
7. API:
   - `POST /stage-versions/:vid/units` `{title}` → 201 `UnitDTO`.
   - `PATCH /stage-versions/:vid/units/:uid` `{title}`.
   - `DELETE /stage-versions/:vid/units/:uid` → 204.
   - `PUT /stage-versions/:vid/units/order` `{unitIds}`.
   - `PUT /stage-versions/:vid/lessons/order` đổi body thành `{units:[{unitId, lessonIds}]}`. Đây là breaking change nội bộ; web đổi cùng ở phase 03.
   - `LessonCmd` thêm `UnitID *uuid.UUID`, bắt buộc khi tạo.
   - `StageVersionDTO` thêm `units: [{id, unitKey, title, position, lessonCount, requiredCount}]`, và `LessonDTO` thêm `unitId`.
   - Mọi route đăng ký trong `Register(rg, requireAdmin)` có sẵn. Route tĩnh `/units/order` đặt trước `/:uid`.
8. Không thêm audit action. MVP không ghi nhật ký cho thao tác sửa bản nháp, chỉ ghi phát hành, nhân bản, lưu trữ và xóa phiên bản; FR-92 cũng không liệt kê bài học.
9. Learning:
   - `StagesOfCourseVersion` join `stage_units`. `StageView` thêm `Units []UnitView{ID, Key, Title, Position, Lessons []LessonView}` và giữ `Lessons` phẳng cho các chỗ đang dùng.
   - `BuildRoadmap` tính theo từng bài học: `RequiredTotal`, `RequiredDone`, `Complete` (chỉ khi `RequiredTotal > 0` và đã xong hết) và `Optional` (khi `RequiredTotal == 0`, hiện "Không bắt buộc").
   - Thẻ chặng có `unitCount`, `lessonCount`, `requiredDone/requiredTotal`.
   - `Locate` trả thêm `unit` của học liệu hiện tại, cờ `crossesUnit` khi học liệu tiếp thuộc bài học khác, và `nextUnitTitle` cho nút "Sang bài học tiếp". Prev/next vẫn đi trong cả khóa như MVP.
   - DTO lộ trình và trang học liệu thêm `units` cùng `unitId/unitTitle/unitPosition`, đủ cho breadcrumb "Lớp của tôi / lớp / chặng / bài học / học liệu".
   - Nhãn "Bài i.j" do web tính từ `stagePosition` và `unitPosition`.

## Files

- Sửa `internal/features/stages/`:
  - `entity.go`, hoặc tách `unit.go` nếu `entity.go` vượt quá ~600 dòng.
  - `entity_test.go`, `service.go`, `service_test.go`, `repository.go`, `repository_pg.go`, `repository_pg_test.go`, `dto.go`, `handler.go`, `testdata/*.json`.
- Sửa `internal/features/learning/`: `roadmap.go`, `roadmap_test.go`, `repository_pg.go`, `repository_pg_test.go`, `dto.go`, `service.go`, `testdata/*.json`.
- Sửa `internal/seed/data.go` và `seed.go`: `stageSeed` khai báo `units` và mỗi `lessonSeed` có `unit`. Chia theo prototype: DB 3 bài học (Nhập môn SQL, Thiết kế bảng, Kiểm tra cuối chặng); Go v1 và v2 mỗi bản 4 bài học, v2 clone giữ key.
- Sửa `internal/features/reports/repository_pg.go` chỉ khi SQL chi tiết học viên cần `unit` (báo cáo FR-40 giữ nguyên theo chặng; OQ-14 ngoài phạm vi).
- Sửa `docs/architecture.md`, mục stages và learning: thêm cấp bài học.

## Các bước

1. Viết test domain trước trong `entity_test.go`:
   - Thêm, đổi tên và xóa bài học, kể cả trường hợp tên trùng.
   - `MoveLesson` đưa học liệu xuống cuối bài học mới.
   - `ReorderLessons` với bố cục thiếu học liệu, thừa học liệu và bài học lạ.
   - `renumber` cho position phẳng đúng.
   - Publish bị chặn khi không có bài học và khi có bài học rỗng (đúng thông điệp).
   - Clone giữ key và ánh xạ unit.
   - Mọi thao tác trên bản `published` trả `ErrVersionImmutable`.
2. Hiện thực domain, rồi service (`AddUnit`, `RenameUnit`, `RemoveUnit`, `ReorderUnits`, cùng `ReorderLessons` theo hình mới) qua `mutateVersion` và `lockDraft` có sẵn.
3. Repository: `loadUnits`, diff trong `SaveDraft`, và `Create`. Test integration:
   - SaveDraft trên bản published không ghi được dòng nào.
   - Chuyển học liệu giữa bài học hoán đổi vị trí không vi phạm UNIQUE nhờ DEFERRED.
   - Xóa bài học xóa học liệu và `lesson_media`.
4. Thêm handler và DTO, cập nhật golden `stages/testdata/stage_version.json`, `stage_version_draft.json` và `stage_detail.json` bằng `-update`, rồi rà diff bằng mắt.
5. Learning: sửa SQL cấu trúc khóa (thêm `u.id, u.unit_key, u.title, u.position` và `ORDER BY cvs.position, u.position, l.position`), sửa `BuildRoadmap` và `Locate`, cập nhật golden `learning/testdata/my-class-basic01.json`, `my-class-draft.json` và `lesson-*.json`.
6. Seed: chia bài học, chạy `make seed-reset` và kiểm UI cũ không vỡ (web phase 03 sẽ hiển thị).
7. Bỏ bản vá tối thiểu "gán vào bài học đầu" của phase 01.

## Test

- Unit: `stages/entity_test.go` (bảng ở bước 1) và `learning/roadmap_test.go`. Roadmap cần các ca: bài học hoàn thành, bài học "Không bắt buộc", `Locate` qua ranh giới bài học, học liệu cuối chặng sang chặng sau.
- Integration: `stages/repository_pg_test.go` và `service_integration_test.go` cho các route mới (admin được; teacher nhận 403 do `requireAdmin`); golden learning.

## Kiểm chứng

```bash
cd apps/api && make lint && TEST_DATABASE_URL=... make test
```

## Tiêu chí hoàn thành

- [x] Toàn bộ checklist FR-78 phía server và FR-90 (bài học) có test.
- [x] Học liệu của phiên bản MVP đã migrate vẫn hiện đúng thứ tự trong bài học "Nội dung chặng".
- [x] Teacher gọi mọi route soạn bài học nhận `FORBIDDEN`.

## Rủi ro

- Đổi body `PUT /lessons/order` là breaking change cho web hiện tại. Phase 03 phải merge cùng đợt; nếu không, giữ handler chấp nhận cả `{lessonIds}` cũ tới khi phase 03 xong.
- `Locate` qua ranh giới chặng: giữ hành vi MVP (học liệu cuối chặng dẫn sang chặng sau), chỉ thêm cờ bài học.
