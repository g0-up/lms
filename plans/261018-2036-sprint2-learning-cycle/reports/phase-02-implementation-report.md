# Báo cáo triển khai: Bài học (backend)

- Phase: [phase-02-units-backend.md](../phase-02-units-backend.md)
- Trạng thái: hoàn thành, có lưu ý (breaking change `PUT lessons/order` chờ web phase 03; seed chưa có Go v2)
- Ngày: 2026-10-18. Chưa commit.

## Kết quả

Aggregate `StageVersion` giờ sở hữu bài học. Mọi thao tác (thêm, đổi tên, xóa, sắp xếp bài học, thêm học liệu vào bài học, chuyển học liệu, sắp xếp theo bố cục đầy đủ) đều gọi `ensureMutable()` rồi `renumber()` để `lessons.position` luôn là thứ tự phẳng 1..n theo (vị trí bài học, thứ tự trong bài học). Phiên bản đã phát hành bị chặn hai lớp: ở domain và ở SQL có điều kiện `status = 'draft'`, cả hai trả `VERSION_IMMUTABLE`. Bản vá tối thiểu của phase 01 đã gỡ:

- `CreateStage` tạo v1 không có bài học, như prototype.
- `SaveDraft` không tự gán học liệu vào bài học đầu.
- Clone chép bài học từ aggregate (id mới, giữ `unit_key`, tiêu đề, thứ tự) và ánh xạ học liệu sang bài học mới.

Learning trả lộ trình nhóm theo bài học, kèm tiến độ từng bài học. Trang học liệu có `unit`, `stage.position`, `crossesUnit` và `nextUnitTitle`. Toàn bộ suite API và lint xanh.

## File đã sửa

`apps/api/internal/features/stages/`:

- `unit.go` (mới, 237 dòng): `Unit`, `UnitLessons`, lỗi nghiệp vụ, `AddUnit`, `RenameUnit`, `RemoveUnit` (trả số học liệu bị xóa), `ReorderUnits`, `MoveLesson`, `ReorderLessons(layout)`, kiểm tên trùng không phân biệt hoa thường.
- `entity.go`: `units` trong aggregate, `renumber()`, `AddLesson` bắt buộc `unitID`, `UpdateLesson` gọi `MoveLesson`, `Publish` kiểm `ErrNoUnits` rồi `ErrEmptyUnit(title)`, `CloneAsDraft` ánh xạ bài học.
- `service.go`: use case `AddUnit`, `RenameUnit`, `RemoveUnit`, `ReorderUnits`, `ReorderLessons(layout)` qua `mutateVersion`. `LessonCmd.UnitID`.
- `repository_pg.go`:
  - `loadUnits`.
  - `SaveDraft` diff `stage_units`, với mọi câu có điều kiện draft và cả hai unique vị trí DEFERRED.
  - `Create` chèn bài học trước học liệu.
  - Ánh xạ unique tên bài học ra `ErrUnitTitleTaken`.
- `dto.go`, `handler.go`:
  - Bốn route bài học: `POST /units` → 201, `PUT /units/order`, `PATCH /units/:uid`, `DELETE /units/:uid` → 204. Route tĩnh đứng trước `/:uid`.
  - Body mới của `PUT lessons/order`.
  - `units[]` trong `StageVersionDTO` và `unitId` trong `LessonDTO`.
- Test: `entity_test.go`, `service_test.go`, `repository_pg_test.go`.
- Golden: `testdata/{stage_version,stage_version_draft,lesson_markdown,lesson_video}.json`.

`apps/api/internal/features/learning/`:

- `roadmap.go`:
  - `UnitView` (tiến độ bắt buộc, `Complete`, `Optional`) và `StageView.Units`, vẫn giữ `Lessons` phẳng.
  - `Locate` trả struct `Location` có `Unit`, `CrossesUnit` và `NextUnitTitle`.
- `repository_pg.go`: SQL cấu trúc join `stage_units`, `ORDER BY cvs.position, u.position, l.position`.
- `dto.go`, `service.go`:
  - Lộ trình: `units[]`, `unitCount`, `lessonCount`, `requiredDone/requiredTotal`, và `unitId` trên học liệu.
  - Trang học liệu: `lesson.unit{id,title,position}`, `lesson.stage.position`, `crossesUnit`, `nextUnitTitle`.
- Test: `roadmap_test.go`, `service_test.go`.
- Golden: `testdata/{my-class-basic01,my-class-draft,lesson-video,lesson-markdown,lesson-ended}.json`.

`apps/api/internal/seed/`:

- `data.go`: `unitSeed`, `stageVersionSeed.units`, `lessonSeed.unit` và helper `.in(unit)`. Chia 11 bài học theo nhóm của `prototype/seed.js`.
- `seed.go`: chèn bài học theo dữ liệu, thay bài học mặc định của phase 01. `checkData` kiểm học liệu thuộc bài học của chính phiên bản, xếp đúng thứ tự bài học, và không có bài học rỗng.
- `seed_test.go`, `seed_integration_test.go`: đếm `stage_units`, bố cục Go, bất biến position phẳng.

Ngoài danh sách File của phase (sửa tối thiểu):

- `apps/api/internal/features/reports/testdata/member-report.json`: golden đổi vì seed Go xếp theo bài học của prototype. "Bài tập tổng hợp" nằm trong bài học "Kiểu dữ liệu và hàm", nên lên vị trí 3, còn "Goroutine cơ bản" xuống 4. Đã rà diff: chỉ hoán vị hai học liệu này.
- `docs/database.md`: sửa câu "Chặng tạo mới cũng nhận bài học mặc định" (không còn đúng) và số liệu seed.
- `docs/architecture.md` (có trong danh sách): mục stages, learning và bất biến thứ tự ghi.

## Test đã thêm

- `stages/entity_test.go` (unit): `TestUnitLifecycle`, `TestAddUnitValidation`, `TestAddLessonRequiresUnit`, `TestAddLessonAppendsToChosenUnit`, `TestRemoveUnitCascadesLessons`, `TestMoveLessonGoesToEndOfNewUnit`, `TestReorderUnitsCarriesLessons`, `TestReorderLessonsLayout` (thiếu, thừa, bài học lạ), `TestRemoveLessonCompactsPositions`, `TestCloneAsDraftKeepsUnitsAndLessons`. Ngoài ra `TestMutationsOnPublishedAreImmutable` và `TestPublishRules` được mở rộng cho bài học.
- `stages/service_test.go` (unit, handler qua gin):
  - `TestUnitUseCases`.
  - `TestHandlerUnitRoutes`: mọi status và envelope. Trùng tên → 409 `CONFLICT`, body `{lessonIds}` cũ → 400.
  - `TestHandlerUnitRoutesNeedAdmin`: 403 trên cả 5 route.
  - Bản published trả `VERSION_IMMUTABLE` cho mọi thao tác bài học.
- `stages/repository_pg_test.go` (integration):
  - `TestImmutability_InsertUnit`, `TestImmutability_RenameUnit`, `TestImmutability_DeleteUnit`: gọi thẳng repository, nhận `VERSION_IMMUTABLE` và DB không đổi.
  - Bảng `ServiceMutations` thêm các thao tác bài học.
  - `TestCreateStageStartsWithoutUnits`.
  - `TestMigratedVersionKeepsOrderInDefaultUnit`.
  - `TestUnitOperationsPersist`.
  - `TestReorderAcrossUnitsWithoutUniqueViolation` (DEFERRED).
  - `TestRemoveUnitCascadesLessonsAndMedia`.
  - `TestUnitTitleUniqueOnRealSchema`: ghi thẳng `"  NỘI DUNG CHẶNG "` thì nhận `CONFLICT`.
  - `TestCloneDeepCopiesUnits`.
  - `TestUnitRoutesRequireAdmin`: identity thật, teacher nhận `FORBIDDEN` trên mọi route, admin đi trọn vòng.
  - Hai test vá tạm của phase 01 đã được thay.
- `learning/roadmap_test.go`: `TestBuildRoadmapUnitProgress` (bài học hoàn thành, "Không bắt buộc", % không đổi), `TestLocateAcrossUnitBoundary`. `TestLocatePrevNextAcrossStages` viết lại: học liệu cuối chặng sang chặng sau có `crossesUnit`.
- `seed/seed_test.go`: `TestCheckDataRejectsBadUnitGrouping`.

## Lệnh đã chạy

- `go test -race -count=1 -p 1 -tags integration` trên `./internal/features/stages/... ./internal/features/learning/... ./internal/seed/... ./internal/features/reports/...`: tất cả `ok`.
- `make -C apps/api test` (đã nạp `.env`): exit 0.
- `make -C apps/api lint GOLANGCI_LINT="go run …golangci-lint@v2.14.0"`: `0 issues`. Lần đầu báo một lỗi S1016 trong `stages/dto.go`; đã sửa thành `UnitLessons(u)` và chạy lại test `stages`.
- `go build ./... && go vet -tags integration ./...`: sạch.
- Golden cập nhật bằng `-update` (stages, learning, reports), rồi rà bằng `git diff`. Mọi thay đổi đều đúng chủ ý: thêm `unitId` và `units`, cùng trường lộ trình và trang học liệu mới. Riêng golden reports chỉ hoán vị hai học liệu Go.
- `make -C apps/api seed-reset` trên DB `lms` (container `lms-postgres-1`): 11 `stage_units`, 16 học liệu. Truy vấn bố cục khớp prototype.

## Khác với plan

- Tên lỗi: plan gọi lỗi 422 "Chọn bài học cho học liệu." là `ErrUnitNotFound`. Code dùng `ErrLessonUnitRequired` cho lỗi này. `ErrUnitNotFound` là 404 khi `uid` của route không tồn tại.
- Thứ tự ghi của `SaveDraft`: xóa học liệu bị bỏ chạy trước, để `lesson_key` được nhả cho học liệu thêm lại. Sau đó là thêm/sửa bài học, ghi học liệu, rồi xóa bài học bị bỏ. Plan liệt kê "thêm bài học, sửa học liệu, xóa học liệu, xóa bài học".
- `PUT lessons/order` không giữ body `{lessonIds}` cũ; body cũ trả 400 vì handler cấm field lạ. Phase 03 phải merge cùng đợt, như mục Rủi ro của phase.
- `stage_detail.json` không đổi, vì response này không chứa học liệu hay bài học. Plan dự kiến golden này đổi.
- `Locate` trả struct `Location` thay vì thêm giá trị trả về.
- Trang học liệu trả `lesson.unit{id,title,position}` lồng nhau, cùng `lesson.stage.position`, thay cho ba trường phẳng `unitId/unitTitle/unitPosition`. Đủ thông tin cho breadcrumb và nhãn "Bài i.j".
- Test teacher 403 nằm trong `stages/repository_pg_test.go`, vì package `stages` không có `service_integration_test.go`. Test này dùng identity thật như các test integration khác.
- Seed:
  - Bài học "Kiểm tra cuối chặng" chưa được seed, vì nó chỉ chứa quiz và homework. Phiên bản đã phát hành không được có bài học rỗng, nên bài học này để phase 04 thêm cùng nội dung quiz.
  - Go có 3 bài học thay vì 4.
  - Chưa seed Go v2 (bản nháp clone). Seed bản nháp này sẽ vi phạm `uq_stage_versions_one_draft` khi e2e `lesson-editor.spec.ts` (E40) clone mọi v1 đã phát hành, kể cả GO.
  - Học liệu `go-hello-read` mới trong prototype cũng chưa chép.

## Câu hỏi mở

- Go v2 (bản nháp) và `go-hello-read`: để phase 16 bù khi rà seed so với `seed.js`, kèm sửa E40? Hay phase 04 thêm cùng bài học "Kiểm tra cuối chặng"?
- Web hiện tại vỡ ở `PUT lessons/order` và ở thêm học liệu, vì giờ bắt buộc `unitId`. Phase 03 cần merge cùng đợt với phase này.
