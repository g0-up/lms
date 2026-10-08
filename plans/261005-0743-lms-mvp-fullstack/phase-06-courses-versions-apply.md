---
phase: 06
title: "Phase 6: Khóa học, phiên bản khóa học, áp dụng FR-17"
status: pending
priority: P1
effort: "3 ngày"
dependencies: [5]
---

# Phase 6: Khóa học, phiên bản khóa học, áp dụng FR-17

## Goal

Hoàn thành feature `courses`: khóa học với bản nháp v1, gắn phiên bản chặng đã phát hành theo thứ tự, phát hành/nhân bản nông/lưu trữ/xóa theo FR-14…FR-16, FR-19, và thao tác "Áp dụng phiên bản chặng mới" FR-17 trong một transaction mỗi khóa học. Phase 7 (lớp) chỉ cần `courses.VersionReader` để kiểm "phiên bản khóa học đã phát hành".

## Context & Requirements

FR phủ: FR-14, FR-15, FR-16, FR-17, FR-18 (nút áp dụng), FR-19 (phần khóa học). NFR §8: FR-17 transactional; audit `course.created`, `course_version.*`, `course.stage_version_applied`. <!-- Red Team: RT-07 - audit course.created -->

**Bất biến ghi (RT-01):** `course_version_stages` chỉ được ghi khi header `course_versions` còn `draft`; đổi trạng thái chỉ chạm header. DB không còn trigger (D1): repository ghi `course_version_stages` bằng `INSERT ... SELECT ... FROM course_versions WHERE id=$1 AND status='draft'` và `DELETE ... USING course_versions cv WHERE cv.status='draft'`, header `WHERE status='draft'`, transition `WHERE status=$from`, tất cả qua `db.ExecAffectOne` (0 dòng → `ErrVersionImmutable`/`ErrInvalidTransition`); `stage_id` điền từ `StageVersionRef.StageID`, FK ghép `fk_cvs_stage_version` bắt lệch. Vì INSERT con chỉ khớp khi header `draft`, FR-17 phải chèn header `draft` → chèn con → `TransitionStatus(draft→published)` trong cùng tx. Repository tách `SaveDraft` (con trước, header sau, không đổi `status`) và `TransitionStatus` (chỉ header). <!-- Red Team: RT-01 - write-order invariant --> <!-- Updated: Session 2 - D1 -->

Quy tắc spec:

- Tạo khóa học: mã + tên; tự tạo `course_versions` v1 `draft`.
- Bản nháp chọn danh sách `stage_versions` **đã phát hành** và thứ tự. Không được chứa hai phiên bản của cùng một chặng (`UNIQUE(course_version_id, stage_id)` Phase 2).
- Phát hành: ≥ 1 chặng, mọi phiên bản chặng đều `published`; sau đó bất biến.
- Nhân bản nông: từ `published` → bản nháp v(max+1) copy danh sách tham chiếu; một bản nháp mỗi khóa học.
- FR-17 (từ màn hình chặng, chọn nhiều khóa học): điều kiện (a) phiên bản chặng nguồn `published`; (b) khóa học đang chứa một phiên bản của cùng chặng trong phiên bản published mới nhất; (c) khóa học không có bản nháp. Thực hiện trong **một transaction cho mỗi khóa học**: clone bản published mới nhất → thay phiên bản chặng, giữ vị trí → publish ngay. Thất bại → khóa học đó không đổi + lý do; khóa học khác vẫn xử lý. Lớp đang chạy không bị ảnh hưởng (lớp trỏ `course_version_id` cũ).
- Xóa: chỉ `draft`; `published` được lớp tham chiếu → không xóa, trả nơi dùng (lớp). Lưu trữ: chỉ `published`. Xóa phiên bản nháp cuối cùng của khóa học thì xóa luôn khóa học (đối xứng với chặng, prototype `deleteCourseVersion`). <!-- Red Team: RT-07 - delete last version deletes parent course -->

Copy tiếng Việt từ `prototype/app.js`:

| Tình huống | Message |
|---|---|
| Thiếu mã/tên | "Nhập mã và tên khóa học." |
| Mã trùng | "Mã khóa học đã tồn tại." (mới) |
| Gắn phiên bản chặng chưa published | "Chỉ gắn được phiên bản chặng đã phát hành." |
| Khóa học đã có phiên bản khác của chặng | "Khóa học đã chứa một phiên bản của chặng này." |
| Publish không có chặng | "Khóa học cần ít nhất một chặng." |
| Publish bản không nháp | "Chỉ phát hành được bản nháp." |
| Clone từ bản không published | "Chỉ nhân bản từ phiên bản đã phát hành." |
| Đã có bản nháp | "Khóa học đang có bản nháp vN. Phát hành hoặc xóa bản nháp trước." + `details.draftVersionId` |
| Archive sai trạng thái | "Chỉ lưu trữ phiên bản đã phát hành." |
| FR-17 nguồn chưa published | "Phiên bản chặng chưa phát hành." |
| FR-17 khóa học không có chặng | "Khóa học không chứa chặng này." |
| FR-17 đã dùng bản này | "Khóa học đã dùng phiên bản này." |
| FR-17 khóa học chưa có bản published | "Khóa học chưa có phiên bản phát hành." |
| Xóa published đang dùng bởi lớp | "Đang được dùng bởi lớp basic01, basic02. Hãy lưu trữ thay vì xóa." (`details.usedBy`; fixture từ seed.js) | <!-- Red Team: RT-03 - fixtures from seed.js -->
| Toast tạo | "Đã tạo khóa học với bản nháp v1." |
| Toast clone | "Đã tạo bản nháp mới (nhân bản nông)." |
| Toast áp dụng | "Đã áp dụng cho N khóa học." / từng dòng lý do thất bại |

## Domain model

### Aggregate `Course`

```go
type Course struct { id ids.ID; code domain.Code; name, description string; createdAt time.Time }
func NewCourse(id ids.ID, code domain.Code, name, description string, now time.Time) (*Course, error) // "Nhập mã và tên khóa học."
```

### Aggregate `CourseVersion` (gốc của `CourseVersionStage`)

```go
type CourseVersion struct {
    id, courseID  ids.ID
    versionNo     domain.VersionNo
    status        domain.VersionStatus
    clonedFromID  *ids.ID
    stages        []CourseVersionStage   // value object, sắp theo position
    publishedAt   *time.Time
}
type CourseVersionStage struct { StageID, StageVersionID ids.ID; Position int }   // VO; StageID denormalized để kiểm trùng chặng trong bộ nhớ

// StageVersionRef là dữ liệu đọc từ stages (qua interface), không phải entity của courses
type StageVersionRef struct { ID, StageID ids.ID; VersionNo domain.VersionNo; Status domain.VersionStatus }

func NewDraftCourseVersion(id, courseID ids.ID, no domain.VersionNo, clonedFrom *ids.ID) *CourseVersion
func (v *CourseVersion) SetStages(refs []StageVersionRef) error      // chỉ draft; mỗi ref published ("Chỉ gắn được…"); không trùng StageID ("Khóa học đã chứa…"); position = index+1
func (v *CourseVersion) ReplaceStageVersion(old, new StageVersionRef) error // chỉ draft; old.StageID == new.StageID; giữ Position; dùng bởi FR-17
func (v *CourseVersion) Publish(now time.Time, lookup func(ids.ID) (StageVersionRef, error)) error // draft only; len>0 ("Khóa học cần ít nhất một chặng."); mọi ref published (re-check tại thời điểm publish); chỉ đổi status+publishedAt trong bộ nhớ, repo ghi bằng TransitionStatus <!-- Red Team: RT-01 -->
func (v *CourseVersion) Archive(now time.Time) error                  // published only; chỉ đổi status
func (v *CourseVersion) CloneAsDraft(newID ids.ID, nextNo domain.VersionNo) (*CourseVersion, error) // published only; copy slice stages (nhân bản nông)
func (v *CourseVersion) CanDelete() error
func (v *CourseVersion) StageVersionFor(stageID ids.ID) (CourseVersionStage, bool)
```

### Use case object FR-17 (Application Service, không phải entity)

```go
type ApplyResult struct { CourseID ids.ID; CourseCode string; NewVersionNo *domain.VersionNo; Error *apperr.Error } // JSON {courseId, courseCode, newVersionNo?, error?} <!-- Red Team: RT-07 - apply result shape -->
```

Lỗi domain → apperr: `ErrStageVersionNotPublished` → `422 VALIDATION_FAILED`; `ErrDuplicateStage` → `409 CONFLICT`; `ErrNoStages` → `422`; `ErrVersionImmutable` → `409 VERSION_IMMUTABLE`; `ErrDraftExists` → `409 DRAFT_EXISTS`; `ErrInvalidTransition` → `409 INVALID_TRANSITION`; `ErrInUse{UsedBy}` → `409 IN_USE`; FR-17: `ErrCourseLacksStage` → `422`, `ErrAlreadyUsingVersion` → `409 CONFLICT`, `ErrNoPublishedVersion` → `422`.

Mẫu: Repository, Unit of Work (mỗi khóa học một `Transact` trong FR-17), State machine, Factory (`CloneAsDraft`), Value Object (`CourseVersionStage`), Anti-corruption qua interface `StageVersionReader` thay vì import entity `stages`. SOLID: `courses` phụ thuộc interface khai báo **trong package courses** (DIP), impl ở `app/deps.go` adapter gọi `stages.Service`; thêm điều kiện FR-17 mới chỉ chạm `applyPreconditions` (SRP).

## Repository interfaces

```go
package courses
type CourseRepo interface {
    Create(ctx context.Context, ex db.Executor, c *Course) error              // 23505 pgerr.UqCoursesCode → ErrCodeTaken <!-- Red Team: RT-11 - constraint consts from Phase 2 -->
    ByID(ctx context.Context, ex db.Executor, id ids.ID) (*Course, error)
    ByIDs(ctx context.Context, ex db.Executor, ids []ids.ID) ([]*Course, error)
    Delete(ctx context.Context, ex db.Executor, id ids.ID) error              // chỉ gọi khi khóa học không còn version nào
    List(ctx context.Context, ex db.Executor, q ListQuery) ([]CourseListRow, error) // versions[] (stageCount, outdatedStageCount), classesUsing[], latestVersionNo, latestPublishedNo, draftVersionId
    LockForUpdate(ctx context.Context, ex db.Executor, id ids.ID) error      // SELECT 1 FROM courses WHERE id=$1 FOR UPDATE
}
type CourseVersionRepo interface {
    // Bất biến RT-01: Create/SaveDraft chỉ chạy khi header là draft (Create luôn INSERT status='draft'); TransitionStatus chỉ UPDATE header.
    Create(ctx context.Context, ex db.Executor, v *CourseVersion) error        // INSERT header status='draft' rồi course_version_stages; 23505 pgerr.UqCourseVersionsOneDraft → ErrDraftExists; 23505 pgerr.UqCvsVersionStage → ErrDuplicateStage <!-- Red Team: RT-01 - header always draft on insert -->
    SaveDraft(ctx context.Context, ex db.Executor, v *CourseVersion) error     // DELETE/INSERT course_version_stages rồi UPDATE header (không đổi status); SET CONSTRAINTS uq_cvs_version_position DEFERRED (pgerr.UqCvsVersionPosition) <!-- Red Team: RT-01 - SaveDraft children first --> <!-- Red Team: RT-11 -->
    TransitionStatus(ctx context.Context, ex db.Executor, id ids.ID, from, to domain.VersionStatus, publishedAt *time.Time) error // UPDATE course_versions SET status=$3, published_at=COALESCE($4, published_at) WHERE id=$1 AND status=$2; 0 dòng → ErrInvalidTransition <!-- Red Team: RT-01 - header-only transition -->
    Delete(ctx context.Context, ex db.Executor, id ids.ID) error              // 23503 từ classes → ErrInUse; CASCADE course_version_stages
    ByID(ctx context.Context, ex db.Executor, id ids.ID) (*CourseVersion, error)
    ByIDForUpdate(ctx context.Context, ex db.Executor, id ids.ID) (*CourseVersion, error) // SELECT ... FOR UPDATE header; mọi use case ghi <!-- Red Team: RT-01 -->
    CountByCourse(ctx context.Context, ex db.Executor, courseID ids.ID) (int, error)
    ListByCourse(ctx context.Context, ex db.Executor, courseID ids.ID) ([]*CourseVersion, error)
    LatestPublished(ctx context.Context, ex db.Executor, courseID ids.ID) (*CourseVersion, error)
    DraftOf(ctx context.Context, ex db.Executor, courseID ids.ID) (*CourseVersion, error)
    NextVersionNo(ctx context.Context, ex db.Executor, courseID ids.ID) (domain.VersionNo, error)
    UsedByClasses(ctx context.Context, ex db.Executor, versionID ids.ID) ([]UsedByClassRow, error) // classId, code, name, status, memberCount
}
// Interface sang feature khác (khai báo ở courses, impl adapter ở app/deps.go)
type StageVersionReader interface {
    Refs(ctx context.Context, ex db.Executor, ids []ids.ID) (map[ids.ID]StageVersionRef, error)
    LatestPublishedOfStage(ctx context.Context, ex db.Executor, stageID ids.ID) (StageVersionRef, error)
    DetailFor(ctx context.Context, ex db.Executor, ids []ids.ID) (map[ids.ID]StageSummary, error) // code, name, versionNo, lessonCount cho DTO
}
// Interface courses cung cấp cho classes (Phase 7)
type VersionReader interface {
    PublishedVersion(ctx context.Context, ex db.Executor, versionID ids.ID) (VersionSummary, error) // ErrNotFound hoặc ErrNotPublished
}
```

## Use cases / Service methods

`courses.Service` nhận `db.Tx`, `CourseRepo`, `CourseVersionRepo`, `StageVersionReader`, `clock.Clock`, `audit.Recorder`, `ids.Generator`.

1. `CreateCourse(ctx, actor, cmd{Code, Name, Description}) (CourseDetailDTO, error)`: Transact: `NewCourse` → `Create`; `NewDraftCourseVersion(no=1)` → `Create` → audit `course.created` `{courseId, code, name}` (prototype log "Tạo khóa học"). <!-- Red Team: RT-07 - audit course.created -->
2. `ListCourses(ctx, q)` → `CourseListItem` (xem HTTP); `GetCourse(ctx, id)` → course + versions DESC (mỗi version: `stageCount`, `outdatedStageCount`, `classCount`).
3. `GetVersion(ctx, versionID) (CourseVersionDTO, error)`: stages theo position với `StageSummary` + `latestPublishedNo` của chặng và `outdated` để UI đánh dấu "có bản mới"; `classes[]` (kèm `memberCount`); `newerPublished` = tồn tại phiên bản khóa học published có `versionNo` lớn hơn. <!-- Red Team: RT-07 - course version DTO -->
4. `SetStages(ctx, actor, versionID, stageVersionIDs []ids.ID) (CourseVersionDTO, error)`: Transact: `ByIDForUpdate` → `Refs(ids)` (thiếu id → 404 "Không tìm thấy phiên bản chặng.") → `v.SetStages(refs)` → `SaveDraft`. Header không draft → `ErrVersionImmutable` từ domain; lớp chặn thứ hai là SQL có điều kiện của `SaveDraft` (0 dòng → `ErrVersionImmutable`) → cũng `VERSION_IMMUTABLE`. <!-- Red Team: RT-01 -->
5. `Publish(ctx, actor, versionID)`: Transact: `ByIDForUpdate` → `v.Publish(now, lookup qua Refs)` → `TransitionStatus(draft→published, now)` (không gọi `SaveDraft`, con không đổi) → audit `course_version.published` `{courseId, versionNo, stageVersionIds}`. <!-- Red Team: RT-01 - publish header-only -->
6. `Clone(ctx, actor, versionID)`: Transact: `LockForUpdate(course)` → `DraftOf` ≠ nil → `ErrDraftExists` → `NextVersionNo` → `CloneAsDraft` → `Create` → audit `course_version.cloned`.
7. `Archive(ctx, actor, versionID)`: Transact: `ByIDForUpdate` → `v.Archive` → `TransitionStatus(published→archived, nil)` → audit `course_version.archived`. Lưu trữ không ảnh hưởng lớp đang trỏ tới (FK RESTRICT chỉ chặn xóa) và không chạm `course_version_stages`. <!-- Red Team: RT-01 - archive header-only -->
8. `Delete(ctx, actor, versionID)`: Transact: `ByIDForUpdate` → `CanDelete` (published → `UsedByClasses` → `ErrInUse` nếu có, ngược lại `INVALID_TRANSITION` "Chỉ xóa được bản nháp.") → `Delete` → `CountByCourse == 0` → `CourseRepo.Delete` → audit `course_version.deleted` `{courseId, versionNo, courseDeleted}`. <!-- Red Team: RT-07 - delete last version deletes course -->
9. `ApplyStageVersion(ctx, actor, stageVersionID ids.ID, courseIDs []ids.ID) ([]ApplyResult, error)` — FR-17:
   1. Ngoài tx: `Refs([stageVersionID])` → `src`; `src.Status != published` → trả lỗi toàn cục `422` "Phiên bản chặng chưa phát hành." (không xử lý khóa học nào).
   2. Khử trùng `courseIDs`, với **mỗi** `courseID` chạy riêng `Transact`:
      1. `LockForUpdate(courseID)`.
      2. `DraftOf(courseID)` ≠ nil → `ErrDraftExists` ("Khóa học đang có bản nháp vN. Phát hành hoặc xóa bản nháp trước.").
      3. `LatestPublished(courseID)` nil → `ErrNoPublishedVersion`.
      4. `cur, ok := latest.StageVersionFor(src.StageID)`; !ok → `ErrCourseLacksStage`; `cur.StageVersionID == src.ID` → `ErrAlreadyUsingVersion`.
      5. `next := NextVersionNo`; `draft := latest.CloneAsDraft(newID, next)`; `draft.ReplaceStageVersion(curRef, src)`; `Create(draft)` — INSERT header `draft` rồi `course_version_stages` qua `INSERT ... SELECT ... WHERE status='draft'` (khớp vì header còn nháp). <!-- Red Team: RT-01 - insert as draft first -->
      6. `draft.Publish(now, lookup)` (các chặng khác vẫn phải published; nếu một chặng khác đã archived → publish lỗi và tx rollback, lý do trả về) → `TransitionStatus(draft→published, now)`. Thứ tự "header draft → con → transition" là bắt buộc; không bao giờ INSERT header với `status='published'` rồi mới chèn con, vì câu INSERT con có điều kiện `status='draft'` sẽ trả 0 dòng → `ErrVersionImmutable`. <!-- Red Team: RT-01 - guarded child INSERT --> <!-- Updated: Session 2 - D1 -->
      7. Audit `course_version.cloned`, `course_version.published`, `course.stage_version_applied` `{courseId, fromVersionNo, toVersionNo, stageId, fromStageVersionId, toStageVersionId}`.
   3. Lỗi trong tx → rollback, `ApplyResult{Error}`; thành công → `NewVersionNo`. Trả toàn bộ danh sách, HTTP 200 kể cả khi có thất bại (multi-status trong body); UI chỉ toast thành công khi mọi dòng đều ok. <!-- Red Team: RT-07 -->
10. `PublishedVersion(ctx, ex, versionID)` (impl `VersionReader` cho classes): `ByID` → `status != published` → `ErrNotPublished` ("Chọn một phiên bản khóa học đã phát hành.").

## HTTP API

Tất cả: đăng nhập + `RequireRole(admin)`. Route phiên bản là route phẳng `/course-versions/{vid}/...` (không lồng dưới `/courses/{id}`), khớp plan.md §7 và Phase 11. <!-- Red Team: RT-07 - flat version routes -->

DTO (plan.md §7 là bản chuẩn; golden JSON của integration test commit tại `apps/api/internal/features/courses/testdata/*.json` cho Phase 11 MSW): <!-- Red Team: RT-07 - golden JSON shared with web -->

- `CourseListItem = {id, code, name, description?, latestVersionNo, latestPublishedNo?, draftVersionId?, versions:[{id, versionNo, status, publishedAt?, stageCount, outdatedStageCount}], classesUsing:[{classId, code, name, versionNo}]}`.
- `CourseVersionDTO = {id, courseId, courseCode, courseName, versionNo, status, publishedAt?, clonedFromVersionNo?, stages:[{position, stageId, stageCode, stageName, stageVersionId, stageVersionNo, lessonCount, requiredCount, latestPublishedNo?, outdated}], classes:[{classId, code, name, status, memberCount}], newerPublished}`.
- `ApplyResultDTO = {courseId, courseCode, newVersionNo?, error?:{code, message}}`.

| Endpoint | Request | Response | Lỗi |
|---|---|---|---|
| `GET /courses?q=` | – | `200 {items:[CourseListItem]}` | – |
| `POST /courses` | `{code,name,description?}` | `201 CourseDetailDTO` (kèm `versions[0]` là v1 draft) | 422; 409 `CONFLICT` |
| `GET /courses/{id}` | – | `200 {id,code,name,description?, versions:[{id,versionNo,status,stageCount,outdatedStageCount,classCount,publishedAt?,clonedFromVersionNo?}], classesUsing:[...]}` | 404 "Không tìm thấy khóa học." |
| `GET /course-versions/{vid}` | – | `200 CourseVersionDTO` | 404 |
| `PUT /course-versions/{vid}/stages` | `{stageVersionIds:[...]}` | `200 CourseVersionDTO` | 404; 422 "Chỉ gắn được phiên bản chặng đã phát hành."; 409 `CONFLICT` "Khóa học đã chứa một phiên bản của chặng này."; 409 `VERSION_IMMUTABLE` |
| `POST /course-versions/{vid}/publish` | – | `200 CourseVersionDTO` | 422 "Khóa học cần ít nhất một chặng."; 409 `INVALID_TRANSITION` |
| `POST /course-versions/{vid}/clone` | – | `201 CourseVersionDTO` (bản nháp mới) | 409 `INVALID_TRANSITION`; 409 `DRAFT_EXISTS` + details |
| `POST /course-versions/{vid}/archive` | – | `200 CourseVersionDTO` | 409 |
| `DELETE /course-versions/{vid}` | – | `200 {courseDeleted: bool}` | 409 `IN_USE` + `details:{usedBy:[{classId,code,name,status}]}`; 409 `INVALID_TRANSITION` |
| `POST /stage-versions/{vid}/apply` | `{courseIds:[...]}` | `200 {results:[ApplyResultDTO]}` | 404; 422 "Phiên bản chặng chưa phát hành."; 422 `courseIds` rỗng |

Route `/stage-versions/{vid}/apply` đăng ký bởi handler `courses` (hành vi là của khóa học) dù prefix là stage-versions; ghi chú trong `router.go`.

## Files to Create / Modify

```text
apps/api/internal/features/courses/
  entity.go          # Course, CourseVersion, CourseVersionStage, StageVersionRef, lỗi
  entity_test.go     # SetStages (dup stage, unpublished), Publish (empty, archived ref), CloneAsDraft, ReplaceStageVersion
  repository.go repository_pg.go   # Create (header draft + con), SaveDraft, TransitionStatus, ByIDForUpdate
  repository_pg_test.go   # //go:build integration: one_draft, uq_cvs_version_stage, FK ghép stage_id lệch, deferrable position, FK RESTRICT từ classes, TestImmutability_* (guarded writes trên bản published), archive published thành công
  service.go         # use case 1–10; apply.go tách riêng cho FR-17
  apply.go apply_test.go apply_integration_test.go
  handler.go dto.go
  testdata/*.json    # golden JSON CourseListItem/CourseVersionDTO/apply results cho Phase 11 MSW <!-- Red Team: RT-07 -->
apps/api/internal/app/deps.go                   # wire courses.Service với adapter stages (adapter StageVersionReader đặt ở đây để stages không import courses)
apps/api/internal/app/router.go
```

Không sửa trong phase này: `platform/db/pgerr/constraints.go` (Phase 2 xuất `UqCoursesCode`, `UqCourseVersionsOneDraft`, `UqCvsVersionStage`, `UqCvsVersionPosition`), `cmd/lms/seed.go` (Phase 2 seed khóa học `BASIC` v1 published dùng 5 chặng theo seed.js). Fixture cho integration test dựng bằng `testdb` helpers (Phase 2), không qua seed. <!-- Red Team: RT-11 - constraints single owner --> <!-- Red Team: RT-14 - seed owned by Phase 2 -->

## Tasks & Steps

1. `entity.go` + unit test bảng: `SetStages` với ref draft → lỗi; hai ref cùng `StageID` → `ErrDuplicateStage`; `Publish` rỗng → `ErrNoStages`; `Publish` với lookup trả archived → lỗi; `CloneAsDraft` copy slice độc lập (sửa clone không đụng gốc); `ReplaceStageVersion` sai `StageID` → lỗi; giữ `Position`.
2. `repository_pg.go`: `Create` INSERT header `status='draft'` rồi stages; `SaveDraft` DELETE/INSERT stages rồi UPDATE header với `SET CONSTRAINTS uq_cvs_version_position DEFERRED`; `TransitionStatus` header-only `WHERE id=$1 AND status=$2`; `ByIDForUpdate`; map lỗi qua `pgerr.UqCoursesCode`, `pgerr.UqCourseVersionsOneDraft`, `pgerr.UqCvsVersionStage`; `UsedByClasses` kèm `memberCount`. <!-- Red Team: RT-01 --> <!-- Red Team: RT-11 -->
3. `service.go` use case 1–8, 10; `apply.go` use case 9 với `applyPreconditions(latest, draft, src)` tách hàm thuần để unit test không cần DB.
4. Adapter `StageVersionReader` ở `app/deps.go` gọi `stages.Service`/`StageVersionRepo`.
5. Handler + DTO; `apply` handler trả 200 + results.
6. Integration test FR-17 theo spec §7.3, chạy trên **schema thật** của Phase 2 với repository thật (`//go:build integration`, fixture qua `testdb` lấy tên từ seed.js): nền từ seed.js: chặng `DB` ("Cơ sở dữ liệu") v1 published, khóa học `BASIC` ("Lập trình cơ bản") published v1 dùng DB v1, lớp `basic01` active trỏ BASIC v1. Seed không có khóa học thứ hai hay DB v2, nên test tự tạo qua `testdb` helpers: DB v2 published; khóa học `HASDRAFT` published dùng DB v1 **và có bản nháp**; khóa học `WEBONLY` chỉ chứa `WEB`; khóa học `DBV2` đã dùng DB v2. Gọi apply với [BASIC, HASDRAFT, WEBONLY, DBV2]: BASIC → `newVersionNo=2`, BASIC v2 published chứa DB v2 cùng position, lớp basic01 vẫn trỏ BASIC v1; HASDRAFT → `DRAFT_EXISTS`; WEBONLY → "Khóa học không chứa chặng này."; DBV2 → "Khóa học đã dùng phiên bản này."; `course_versions` của HASDRAFT/WEBONLY/DBV2 không đổi (đếm trước/sau); audit có `course.stage_version_applied` chỉ cho BASIC. <!-- Red Team: RT-01 - apply on real schema --> <!-- Red Team: RT-03 - fixtures from seed.js -->
7. Integration test lỗi giữa tx: fake `audit.Recorder` trả lỗi ở BASIC → rollback, không tạo BASIC v2. Mọi lỗi pg trong repo đi qua `pgerr.Map(err)` (package `platform/db/pgerr`, Phase 2), không viết hàm map riêng.
8. Integration test trạng thái: (a) publish bản nháp có chặng → thành công, `published_at` set, `course_version_stages` không đổi; (b) **archive bản published thành công** không chạm con; (c) `TestImmutability_*`: gọi repository trực tiếp (bỏ qua service) lên bản published: `SaveDraft` (đổi stages), INSERT/DELETE `course_version_stages`, `Delete` header → mỗi câu `ErrVersionImmutable`, dữ liệu không đổi; `TransitionStatus(published→draft)` → `ErrInvalidTransition`; (d) delete bản nháp cuối → khóa học bị xóa; (e) `Create` với `stage_id` lệch với `stage_versions.stage_id` → 23503 `FkCvsStageVersion` qua `pgerr.Map`. Các test `TestImmutability_*` được Phase 14 H5 gọi lại qua `make hardening-immutability`. <!-- Red Team: RT-01 - write-order tests --> <!-- Red Team: RT-07 --> <!-- Updated: Session 2 - D1 -->
9. Ghi golden JSON `testdata/*.json` cho Phase 11. <!-- Red Team: RT-07 -->
10. Lint, test.

## Verification

```bash
cd apps/api && go test ./internal/features/courses/...
cd apps/api && go test -tags integration ./internal/features/courses/...
make dev && make seed   # cookie admin c.txt; H như Phase 5
curl -s -b c.txt $H -d '{"code":"DEMO","name":"Khóa học thử nghiệm"}' localhost:8080/api/v1/courses | jq '.versions[0] | {id, versionNo, status}'   # khóa học tạo tay để thử; seed chỉ có BASIC
CV=<draft id>
curl -s -b c.txt $H -X PUT -d '{"stageVersionIds":["<DB v1>","<DB v2>"]}' localhost:8080/api/v1/course-versions/$CV/stages | jq .error.message   # "Khóa học đã chứa một phiên bản của chặng này."
curl -s -b c.txt $H -X PUT -d '{"stageVersionIds":["<draft stage version>"]}' localhost:8080/api/v1/course-versions/$CV/stages | jq .error.message  # "Chỉ gắn được phiên bản chặng đã phát hành."
curl -s -b c.txt $H -X POST localhost:8080/api/v1/course-versions/$CV/publish | jq .error.message   # "Khóa học cần ít nhất một chặng."
curl -s -b c.txt $H -X PUT -d '{"stageVersionIds":["<DB v1>","<WEB v1>"]}' localhost:8080/api/v1/course-versions/$CV/stages -o /dev/null -w '%{http_code}\n'  # 200
curl -s -b c.txt $H -X POST localhost:8080/api/v1/course-versions/$CV/publish | jq .status   # published
curl -s -b c.txt $H -X PUT -d '{"stageVersionIds":["<DB v2>"]}' localhost:8080/api/v1/course-versions/$CV/stages | jq .error.code   # VERSION_IMMUTABLE
curl -s -b c.txt $H -X POST localhost:8080/api/v1/course-versions/$CV/archive | jq .status   # archived (header-only)
# FR-17: dùng BASIC từ seed (v1 published dùng DB v1); DB v2 tạo bằng clone+publish chặng DB trước
curl -s -b c.txt $H -d "{\"courseIds\":[\"$CID\",\"<course with draft>\",\"<course without DB>\"]}" localhost:8080/api/v1/stage-versions/<DB v2>/apply | jq .
# kỳ vọng: results[0] == {courseId, courseCode:"BASIC", newVersionNo:2}; results[1].error.code == "DRAFT_EXISTS"; results[2].error.message == "Khóa học không chứa chặng này."
curl -s -b c.txt localhost:8080/api/v1/courses/$CID | jq '.versions | length'   # 2
curl -s -b c.txt $H -X DELETE localhost:8080/api/v1/course-versions/$CV | jq '.error | {code, details}'   # IN_USE nếu đã có lớp (details.usedBy[].code = basic01), else INVALID_TRANSITION
```

## Security notes

- Chỉ admin; mọi thao tác ghi đều có audit với `actor_id`.
- FR-17 không bao giờ đụng bảng `classes`/`lesson_progress`; test khẳng định `classes.course_version_id` không đổi.
- DB không có trigger (D1), nên lớp bảo vệ thứ hai là chính câu SQL của repository: mọi INSERT/UPDATE/DELETE lên `course_version_stages` mang điều kiện header `draft`, nên bản published không thể bị sửa kể cả khi service có lỗi. FR-17 chèn header nháp trước rồi mới chuyển trạng thái; integration test gọi repository trực tiếp để chứng minh. SQL tay ngoài ứng dụng không bị chặn (quy tắc vận hành ở `docs/runbook.md`). <!-- Red Team: RT-01 --> <!-- Updated: Session 2 - D1 -->

## Risks & Rollback

| Rủi ro | Biện pháp |
|---|---|
| Spec nói "một transaction" nhưng plan.md chọn một tx **mỗi khóa học** | Giữ theo plan.md (từng khóa học độc lập, kết quả từng dòng như prototype); ghi rõ trong docs API |
| Chặng khác trong khóa học đã archived → apply thất bại | Message rõ "Phiên bản chặng <code> vN đã lưu trữ, không thể phát hành lại."; UI hiển thị theo dòng |
| Race apply + clone thủ công đồng thời | `LockForUpdate(course)` tuần tự hóa; partial unique index chặn tầng DB |
| Deferrable unique cho position khi `SaveDraft` DELETE/INSERT | `SET CONSTRAINTS uq_cvs_version_position DEFERRED` đầu tx của `SaveDraft`/`Create` (tên từ `pgerr.UqCvsVersionPosition`) | <!-- Red Team: RT-11 -->
| Một câu ghi lên `course_versions`/`course_version_stages` thiếu điều kiện trạng thái → sửa được bản published vì DB không còn trigger | Hợp đồng SQL có điều kiện + `db.ExecAffectOne`; `Create` luôn chèn header draft; test (8a)(8b)(8c) và FR-17 gọi repository thật; checklist review `docs/database.md` | <!-- Red Team: RT-01 --> <!-- Updated: Session 2 - D1 -->

Rollback: không mount `courses` trong router; dữ liệu giữ nguyên.

## Success Criteria

- [x] Unit + integration `courses` xanh.
- [x] Tạo khóa học → v1 draft; mã trùng → 409.
- [x] `PUT stages`: ref draft → 422; hai version cùng chặng → 409 `CONFLICT`; trên bản published → 409 `VERSION_IMMUTABLE`.
- [x] Publish rỗng → 422 "Khóa học cần ít nhất một chặng."; publish hợp lệ → `published_at`, audit, `course_version_stages` không đổi; archive bản published thành công trên schema thật, `archived_at` set. <!-- Red Team: RT-01 -->
- [x] Tạo khóa học ghi audit `course.created`; xóa bản nháp cuối cùng xóa luôn khóa học và trả `courseDeleted:true`. <!-- Red Team: RT-07 -->
- [x] Clone → v(n+1) draft với danh sách tham chiếu y hệt; clone lần hai → `DRAFT_EXISTS` + `draftVersionId`.
- [x] Delete published bị lớp dùng → 409 `IN_USE` liệt kê mã lớp (`basic01`); delete draft → 200 `{courseDeleted}`.
- [x] FR-17 integration (§7.3) trên repository và schema thật: BASIC thành công v2 published giữ thứ tự; HASDRAFT/WEBONLY/DBV2 (tạo trong test qua `testdb`) thất bại với đúng message; dữ liệu của ba khóa học đó không đổi; lớp `basic01` vẫn trỏ BASIC v1; audit `course.stage_version_applied` đúng một bản ghi; kết quả dùng `courseCode`. <!-- Red Team: RT-01 --> <!-- Red Team: RT-03 -->
- [x] FR-17 với nguồn chưa published → 422 toàn cục, không có khóa học nào đổi.
