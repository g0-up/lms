---
phase: 05
title: "Phase 5: Chặng, phiên bản chặng, học liệu, media"
status: pending
priority: P1
effort: "4 ngày"
dependencies: [3]
---

# Phase 5: Chặng, phiên bản chặng, học liệu, media

## Goal

Hoàn thành feature `stages` (chặng, phiên bản chặng, học liệu) và feature `media` (upload video/ảnh lên object storage S3-compatible, URL ký). Sau phase này Admin tạo chặng với bản nháp v1, thêm/sửa/sắp học liệu trong bản nháp, phát hành, nhân bản sâu, lưu trữ, xóa theo đúng FR-10…FR-13, FR-19; màn hình chặng trả danh sách khóa học dùng phiên bản cũ (FR-18, phần đọc). Nút "Áp dụng" (FR-17) thuộc Phase 6. Xóa phiên bản nháp cuối cùng của một chặng thì xóa luôn chặng (theo `deleteStageVersion` prototype app.js:157-163). <!-- Red Team: RT-07 - delete last version deletes parent stage -->

## Context & Requirements

FR phủ: FR-10, FR-11, FR-12, FR-13, FR-19 (phần chặng), FR-18 (truy vấn đọc). NFR §8: lọc HTML markdown phía server, URL video ký có hạn, bucket không public, bất biến phiên bản ở tầng ứng dụng theo hợp đồng Phase 02 "Bất biến phiên bản ở tầng ứng dụng" (DB không có trigger, D1), audit `stage.created`, `stage_version.*`. <!-- Red Team: RT-07 - audit stage.created -->

**Bất biến ghi (RT-01):** mọi dòng con (`lessons`, `lesson_media`) chỉ được ghi khi header `stage_versions` còn `draft`; chuyển trạng thái chỉ chạm header. DB không còn trigger (D1), nên repository là lớp chặn cuối: mọi câu ghi lên `lessons`/`lesson_media` mang điều kiện header `draft` ngay trong SQL (`INSERT ... SELECT ... FROM stage_versions WHERE id=$1 AND status='draft'`; `UPDATE/DELETE lessons ... USING stage_versions sv WHERE sv.status='draft'`), header ghi `WHERE status='draft'`, transition `WHERE status=$from` (publish thêm `AND NOT EXISTS (markdown chưa render)`); tất cả qua `db.ExecAffectOne`, 0 dòng → `ErrVersionImmutable`/`ErrInvalidTransition`. Repository tách `SaveDraft` (con trước, header sau, không đổi `status`) và `TransitionStatus` (chỉ header); mọi use case ghi mở `ByIDForUpdate` trước. <!-- Red Team: RT-01 - write-order invariant --> <!-- Updated: Session 2 - D1 -->

Quy tắc spec giữ nguyên:

- Tạo chặng: mã (`domain.Code`, unique) + tên; tự tạo `stage_versions` v1 `draft`.
- Học liệu chỉ thêm/sửa/xóa/sắp xếp trong bản `draft`; thuộc tính: tiêu đề, loại `video|markdown`, thứ tự, `required`. Video bắt buộc có file; markdown bắt buộc có nội dung. Ảnh trong markdown phải upload qua media, không nhúng base64.
- Phát hành: ≥ 1 học liệu; sau đó bất biến.
- Nhân bản: chỉ từ `published`; `version_no` = max + 1; copy học liệu giữ `lesson_key`; chỉ tham chiếu `media_files`, không copy file; mỗi chặng tối đa một bản nháp, nếu đã có → lỗi và trả `draftVersionId` để UI link tới.
- Lưu trữ: chỉ `published`. Xóa: chỉ `draft`; bản `published` được khóa học tham chiếu → không xóa, trả danh sách nơi dùng.
- Video phát qua URL ký ngắn hạn, hỗ trợ tua (Range do storage xử lý).

Copy tiếng Việt từ `prototype/app.js`:

| Tình huống | Message |
|---|---|
| Thiếu mã/tên | "Nhập mã và tên chặng." |
| Mã trùng | "Mã chặng đã tồn tại." (mới) |
| Không tìm thấy | "Không tìm thấy chặng." |
| Thiếu tiêu đề học liệu | "Nhập tiêu đề học liệu." |
| Video không có file | "Học liệu video bắt buộc có file video." |
| Markdown trống | "Học liệu markdown bắt buộc có nội dung." |
| Publish không có học liệu | "Chặng cần ít nhất một học liệu trước khi phát hành." |
| Publish bản không nháp | "Chỉ phát hành được bản nháp." |
| Clone từ bản không published | "Chỉ nhân bản từ phiên bản đã phát hành." |
| Đã có bản nháp | "Chặng đã có bản nháp vN. Phát hành hoặc xóa bản nháp trước." + `details.draftVersionId` |
| Archive bản không published | "Chỉ lưu trữ phiên bản đã phát hành." |
| Sửa bản không nháp | "Phiên bản đã phát hành không thể chỉnh sửa." (mới, từ `ErrVersionImmutable` của domain hoặc 0 dòng của SQL có điều kiện) |
| Xóa bản published đang dùng | "Đang được dùng trong Lập trình cơ bản v1. Hãy lưu trữ thay vì xóa." (`details.usedBy`, app.js:155; fixture từ seed.js) | <!-- Red Team: RT-03 - fixtures from seed.js -->
| Toast tạo | "Đã tạo chặng với bản nháp v1." |
| Toast clone | "Đã tạo bản nháp mới. Học liệu được sao chép, giữ nguyên lesson_key." |
| Toast xóa/lưu trữ | "Đã xóa." / "Đã lưu trữ." |
| FR-18 heading | "Khóa học đang dùng phiên bản cũ của chặng này" |
| FR-18 ok | "Mọi khóa học đã phát hành đều dùng phiên bản mới nhất của chặng này." |
| FR-18 blocked | "Khóa học đang có bản nháp vN. Phát hành hoặc xóa bản nháp trước." |

Nhãn trạng thái: `draft` "Nháp", `published` "Đã phát hành", `archived` "Lưu trữ".

## Domain model

### Aggregate `Stage`

```go
type Stage struct { id ids.ID; code domain.Code; name string; description string; createdAt time.Time }
func NewStage(id ids.ID, code domain.Code, name, description string, now time.Time) (*Stage, error) // name trống → ErrValidation("Nhập mã và tên chặng.")
```

### Aggregate `StageVersion` (gốc của Lessons)

```go
type StageVersion struct {
    id, stageID   ids.ID
    versionNo     domain.VersionNo
    status        domain.VersionStatus      // draft | published | archived; state machine ở shared kernel
    clonedFromID  *ids.ID
    lessons       []*Lesson                 // đã sắp theo position
    publishedAt   *time.Time
}
func NewDraftVersion(id, stageID ids.ID, no domain.VersionNo, clonedFrom *ids.ID) *StageVersion
func (v *StageVersion) AddLesson(id ids.ID, key domain.LessonKey, title string, content LessonContent, required bool) (*Lesson, error) // chỉ draft; position = len+1
func (v *StageVersion) UpdateLesson(id ids.ID, title string, content LessonContent, required bool) error
func (v *StageVersion) RemoveLesson(id ids.ID) error                     // dồn position
func (v *StageVersion) Reorder(idsInOrder []ids.ID) error                 // phải là hoán vị đầy đủ → ErrValidation
func (v *StageVersion) RenderMarkdown(render MarkdownRenderer) error    // draft only; điền MarkdownContent.HTML cho từng lesson markdown (gọi trước Publish, trong cùng tx) <!-- Red Team: RT-01 -->
func (v *StageVersion) Publish(now time.Time) error                       // draft only; len(lessons)>0; mỗi content.Validate(); mọi markdown phải có HTML (ErrNotRendered); chỉ đổi status+publishedAt
func (v *StageVersion) Archive(now time.Time) error                       // published only; chỉ đổi status
func (v *StageVersion) CloneAsDraft(newID ids.ID, nextNo domain.VersionNo, lessonIDs func() ids.ID) (*StageVersion, error) // Factory; published only; deep copy lessons giữ lessonKey, media ref
func (v *StageVersion) CanDelete() error                                  // draft only
```

### Entity `Lesson` + Strategy `LessonContent`

```go
type Lesson struct { id ids.ID; key domain.LessonKey; title string; position int; required bool; content LessonContent }
type LessonContent interface {
    Type() LessonType                 // "video" | "markdown"
    Validate() error                  // video: mediaID != zero; markdown: strings.TrimSpace(source) != ""
}
type VideoContent struct { MediaID ids.ID; DurationSeconds *int }   // cột lessons.duration_seconds (Phase 2); UI "Thời lượng" mm:ss ↔ giây <!-- Red Team: RT-07 - durationSeconds -->
type MarkdownContent struct { Source string; HTML string }   // HTML chỉ được điền khi Publish (render + sanitize)
type MarkdownRenderer interface { Render(source string) (html string, err error) }  // goldmark + bluemonday, rewrite <img src>
```

`LessonKey` (shared kernel Phase 3): slug `^[a-z0-9][a-z0-9-]{0,39}$` (tối đa 40 ký tự, khớp CHECK Phase 2), sinh từ tiêu đề <!-- Red Team: RT-03 - lesson_key 40 chars --> nếu client không gửi, unique trong một `stage_version`; giữ nguyên khi clone để `lesson_progress` bản mới có thể đối chiếu (Phase 8 dùng `lesson_id` trực tiếp, key chỉ phục vụ báo cáo so sánh giữa phiên bản).

Lỗi domain → apperr: `ErrVersionImmutable` → `409 VERSION_IMMUTABLE`; `ErrDraftExists{DraftID, No}` → `409 DRAFT_EXISTS`; `ErrInvalidTransition` → `409 INVALID_TRANSITION`; `ErrNoLessons` → `422 VALIDATION_FAILED`; `ErrNotRendered` → `409 INVALID_TRANSITION` "Phiên bản còn học liệu chưa render." (domain kiểm trước; `TransitionStatus(draft→published)` còn có `AND NOT EXISTS (markdown chưa render)` trong SQL nên 0 dòng → `ErrInvalidTransition` cùng message, chỉ xảy ra khi có lỗi code); `ErrInUse{UsedBy}` → `409 IN_USE`; `ErrCodeTaken` → `409 CONFLICT`. <!-- Red Team: RT-01 -->

### Feature `media`

```go
type MediaFile struct { id ids.ID; storageKey string; kind MediaKind /* video|image */; contentType string; sizeBytes int64; status MediaStatus /* pending|ready */; uploadedBy ids.ID; createdAt time.Time }
func NewPendingUpload(id ids.ID, kind MediaKind, contentType string, size int64, by ids.ID, now time.Time) (*MediaFile, error) // kiểm content type + size theo config
func (m *MediaFile) MarkReady(stat ObjectStat) error   // size khớp, contentType khớp

package storage // internal/platform/storage
type Storage interface {
    PresignPut(ctx context.Context, key, contentType string, size int64, ttl time.Duration) (url string, headers map[string]string, err error)
    PresignGet(ctx context.Context, key string, ttl time.Duration, responseContentType string) (string, error)
    Stat(ctx context.Context, key string) (ObjectStat, error)
    Delete(ctx context.Context, key string) error
}
type ObjectStat struct { Size int64; ContentType string; ETag string }
```

Storage prod là Cloudflare R2, dev là MinIO community; code chỉ nói chuyện S3 API qua `Storage` (minio-go v7), không có nhánh riêng cho R2. <!-- Updated: Validation Session 1 - prod R2, dev MinIO -->

Mẫu: Repository, Unit of Work, State machine (`VersionStatus` shared kernel), Strategy (`LessonContent`, `Storage` với impl minio-go và `MemStorage` cho test), Factory (`CloneAsDraft`), Adapter (`MarkdownRenderer`). SOLID: `stages` không import `minio`; handler media chỉ biết `Storage`; thêm loại học liệu mới = thêm struct implement `LessonContent` + một case trong `repository_pg` scan (OCP, có ghi chú đây là điểm mở duy nhất).

## Repository interfaces

```go
package stages
type StageRepo interface {
    Create(ctx context.Context, ex db.Executor, s *Stage) error              // 23505 pgerr.UqStagesCode → ErrCodeTaken <!-- Red Team: RT-11 - constraint consts -->
    ByID(ctx context.Context, ex db.Executor, id ids.ID) (*Stage, error)
    Delete(ctx context.Context, ex db.Executor, id ids.ID) error              // chỉ gọi khi stage không còn version nào (xem Delete use case)
    List(ctx context.Context, ex db.Executor, q ListQuery) ([]StageListRow, error) // kèm versions[], latestVersionNo, latestPublishedNo, draftVersionId, usedByCourseCount, outdatedCourseCount
}
type StageVersionRepo interface {
    // Bất biến RT-01: Create/SaveDraft chỉ chạy khi header là draft; TransitionStatus chỉ UPDATE header.
    Create(ctx context.Context, ex db.Executor, v *StageVersion) error        // INSERT header status='draft' rồi lessons + lesson_media; 23505 pgerr.UqStageVersionsOneDraft → ErrDraftExists
    SaveDraft(ctx context.Context, ex db.Executor, v *StageVersion) error     // diff lessons (delete/insert/update, kể cả markdown_html, duration_seconds), ghi lại lesson_media, rồi UPDATE header (không đổi status); SET CONSTRAINTS pgerr.UqLessonsVersionPosition DEFERRED <!-- Red Team: RT-01 - SaveDraft children first -->
    TransitionStatus(ctx context.Context, ex db.Executor, id ids.ID, from, to domain.VersionStatus, publishedAt *time.Time) error // db.ExecAffectOne: UPDATE stage_versions SET status=$3, published_at=COALESCE($4, published_at), archived_at=CASE WHEN $3='archived' THEN now() END, updated_at=now() WHERE id=$1 AND status=$2 [AND NOT EXISTS (SELECT 1 FROM lessons WHERE stage_version_id=$1 AND type='markdown' AND markdown_html IS NULL) khi to=published]; ErrNoRowsAffected → ErrInvalidTransition ("Phiên bản còn học liệu chưa render." khi to=published) <!-- Red Team: RT-01 - header-only transition --> <!-- Updated: Session 2 - D1 -->
    Delete(ctx context.Context, ex db.Executor, id ids.ID) error              // 23503 từ course_version_stages → ErrInUse (details điền bởi UsedBy); CASCADE lessons, lesson_media
    ByID(ctx context.Context, ex db.Executor, id ids.ID) (*StageVersion, error) // load lessons ORDER BY position
    ByIDForUpdate(ctx context.Context, ex db.Executor, id ids.ID) (*StageVersion, error) // như ByID nhưng SELECT ... FOR UPDATE header; dùng cho mọi use case ghi <!-- Red Team: RT-01 - FOR UPDATE -->
    ListByStage(ctx context.Context, ex db.Executor, stageID ids.ID) ([]*StageVersion, error)
    CountByStage(ctx context.Context, ex db.Executor, stageID ids.ID) (int, error)
    NextVersionNo(ctx context.Context, ex db.Executor, stageID ids.ID) (domain.VersionNo, error) // SELECT coalesce(max)+1 ... FOR UPDATE trên stages row để tránh race
    DraftOf(ctx context.Context, ex db.Executor, stageID ids.ID) (*StageVersion, error)
    UsedBy(ctx context.Context, ex db.Executor, versionID ids.ID) ([]UsedByRow, error) // courseId, courseCode, courseName, courseVersionId, versionNo, status, classCodes[] <!-- Red Team: RT-07 - usedBy DTO -->
    OutdatedCourses(ctx context.Context, ex db.Executor, stageID ids.ID) ([]OutdatedCourseRow, error) // FR-18
    AllOutdated(ctx context.Context, ex db.Executor) ([]OutdatedCourseRow, error)                    // FR-18 toàn hệ thống (dashboard Phase 9) <!-- Red Team: RT-07 - AllOutdated -->
}
// Interface stages cung cấp cho dashboard (Phase 9 chỉ tiêu thụ, không viết SQL riêng)
type OutdatedReader interface {
    OutdatedCourses(ctx context.Context, stageID ids.ID) ([]OutdatedCourse, error)
    AllOutdated(ctx context.Context) ([]OutdatedCourse, error)
}
type OutdatedCourse struct { CourseID ids.ID; CourseCode, CourseName string; CourseVersionID ids.ID; CourseVersionNo domain.VersionNo; StageID ids.ID; StageCode, StageName string; UsingVersionNo domain.VersionNo; LatestVersionID ids.ID; LatestVersionNo domain.VersionNo; DraftVersionID *ids.ID; DraftVersionNo *domain.VersionNo }

package media
type MediaRepo interface {
    Create(ctx context.Context, ex db.Executor, m *MediaFile) error
    Update(ctx context.Context, ex db.Executor, m *MediaFile) error
    ByID(ctx context.Context, ex db.Executor, id ids.ID) (*MediaFile, error)
    CanUserAccess(ctx context.Context, ex db.Executor, mediaID, userID ids.ID) (bool, error) // JOIN lesson_media; xem Security <!-- Red Team: RT-15 -->
}
```

`CanUserAccess` SQL (admin được kiểm ở service, không vào đây): <!-- Red Team: RT-15 - lesson_media join replaces position() scan -->

```sql
SELECT EXISTS (
  SELECT 1 FROM lesson_media lm
  JOIN lessons l ON l.id = lm.lesson_id
  JOIN course_version_stages cvs ON cvs.stage_version_id = l.stage_version_id
  JOIN classes c ON c.course_version_id = cvs.course_version_id
  WHERE lm.media_id = $1
    AND ( c.teacher_id = $2
       OR EXISTS (SELECT 1 FROM class_members cm WHERE cm.class_id = c.id AND cm.user_id = $2
                  AND cm.status = 'active' AND c.status IN ('active','ended')) )
);
```

`lesson_media(lesson_id, media_id)` (Phase 2) được `SaveDraft` ghi lại toàn bộ cho từng lesson thay đổi: `video_media_id` + mọi UUID trích từ `<img src="/api/v1/media/{id}/content">` trong `markdown_html` đã render (regex cùng pattern với bluemonday). FK `media_id` RESTRICT nên không xóa được media đang được học liệu tham chiếu.

`OutdatedCourses` SQL (theo logic `outdatedCourses` trong prototype dòng 116–128; `AllOutdated` là cùng CTE bỏ điều kiện `stage_id=$1`, thêm `s.code, s.name` và `latest.id`): <!-- Red Team: RT-07 - AllOutdated shares SQL -->

```sql
WITH latest AS (SELECT id, version_no FROM stage_versions WHERE stage_id=$1 AND status='published' ORDER BY version_no DESC LIMIT 1),
lp AS (SELECT DISTINCT ON (cv.course_id) cv.id cv_id, cv.course_id, cv.version_no
       FROM course_versions cv WHERE cv.status='published' ORDER BY cv.course_id, cv.version_no DESC)
SELECT c.id course_id, c.code, c.name, lp.version_no course_version_no, sv.version_no using_version_no,
       d.id draft_version_id, d.version_no draft_version_no
FROM lp JOIN course_version_stages cvs ON cvs.course_version_id=lp.cv_id
JOIN stage_versions sv ON sv.id=cvs.stage_version_id AND sv.stage_id=$1
JOIN courses c ON c.id=lp.course_id
LEFT JOIN course_versions d ON d.course_id=c.id AND d.status='draft'
WHERE sv.version_no < (SELECT version_no FROM latest)
ORDER BY c.name;
```

## Use cases / Service methods

`stages.Service` nhận `db.Tx`, `StageRepo`, `StageVersionRepo`, `media.Reader` (interface `ByID`, kiểm `status=ready` và `kind`), `MarkdownRenderer`, `clock.Clock`, `audit.Recorder`, `ids.Generator`.

1. `CreateStage(ctx, actor, cmd{Code, Name, Description}) (StageDetailDTO, error)`: Transact: `NewStage` → `Create`; `NewDraftVersion(no=1)` → `Create` → audit `stage.created` `{stageId, code, name}` (prototype log "Tạo chặng"). <!-- Red Team: RT-07 - audit stage.created -->
2. `GetStage(ctx, id) (StageDetailDTO, error)`: stage + versions (DESC, mỗi version `lessonCount`) + `usedBy` (gộp từ `UsedBy` của mọi version, thêm `stageVersionNo`, `outdated`) + `outdatedCourses` (FR-18; mỗi row `canApply = draftVersionId == nil`, `blockedReason` = message FR-18 blocked). <!-- Red Team: RT-07 - usedBy on stage detail -->
3. `ListStages(ctx, q)`.
4. `GetVersion(ctx, versionID) (StageVersionDTO, error)`: lessons theo `LessonDTO` (video: `videoMediaId`, `videoFileName`, `durationSeconds`; markdown: `markdownSource`, `markdownHtml` chỉ khi published) + `usedBy`. <!-- Red Team: RT-07 - LessonDTO -->
5. `AddLesson(ctx, actor, versionID, cmd{Title, Type, LessonKey?, Required, DurationSeconds?, VideoMediaID?, MarkdownSource?})`: Transact: `ByIDForUpdate` → build content qua `contentFactory(cmd)` → với video kiểm `media.Reader.ByID` status `ready` kind `video` (sai → "Học liệu video bắt buộc có file video.") → `v.AddLesson` → `SaveDraft`. Tiêu đề trống → "Nhập tiêu đề học liệu.". `FOR UPDATE` tuần tự hóa với `Publish`; nếu header đã published khi lock xong → `ErrVersionImmutable` từ domain; lớp chặn thứ hai là SQL có điều kiện của `SaveDraft` (0 dòng → `ErrVersionImmutable`), cũng map `VERSION_IMMUTABLE`. <!-- Red Team: RT-01 - ByIDForUpdate + SaveDraft --> <!-- Updated: Session 2 - D1 -->
6. `UpdateLesson`, `RemoveLesson`, `ReorderLessons(ctx, actor, versionID, []ids.ID)`: tương tự; reorder chạy `SET CONSTRAINTS uq_lessons_version_position DEFERRED` (tên từ `pgerr.UqLessonsVersionPosition`) trong tx. <!-- Red Team: RT-11 -->
7. `Publish(ctx, actor, versionID)`: Transact: `ByIDForUpdate` → `v.RenderMarkdown(renderer)` → `SaveDraft` (ghi `markdown_html` + `lesson_media` khi header còn draft) → `v.Publish(now)` (kiểm ≥1 học liệu, mọi content hợp lệ, mọi markdown đã có HTML) → `TransitionStatus(draft→published, now)` → audit `stage_version.published` `{stageId, versionNo, lessonCount}`. Thứ tự này là bắt buộc: ghi con trước khi header đổi trạng thái, vì câu ghi con chỉ khớp khi header còn `draft` (0 dòng → `ErrVersionImmutable`). <!-- Red Team: RT-01 - publish order --> <!-- Updated: Session 2 - D1 -->
8. `Clone(ctx, actor, versionID) (StageVersionDTO, error)`: Transact: lock stage row → `DraftOf` ≠ nil → `ErrDraftExists{DraftID, No}` → `NextVersionNo` → `src.CloneAsDraft` → `Create` (partial unique index là lớp chặn thứ hai) → audit `stage_version.cloned` `{fromVersionNo, toVersionNo}`.
9. `Archive(ctx, actor, versionID)`: Transact: `ByIDForUpdate` → `v.Archive(now)` → `TransitionStatus(published→archived, nil)`; không bao giờ chạm `lessons` → audit `stage_version.archived`. <!-- Red Team: RT-01 - archive header-only -->
10. `Delete(ctx, actor, versionID)`: Transact: `ByIDForUpdate` → `v.CanDelete()`; nếu `published` → `UsedBy` → `ErrInUse{UsedBy}` message "Đang được dùng trong …" nếu có, ngược lại vẫn `INVALID_TRANSITION` "Chỉ xóa được bản nháp." (spec: bản published chỉ lưu trữ); `Delete` (CASCADE lessons, lesson_media) → `CountByStage == 0` → `StageRepo.Delete` (chặng mất theo prototype app.js:157-163) → audit `stage_version.deleted` `{stageId, versionNo, stageDeleted}`. <!-- Red Team: RT-07 - delete last version deletes stage -->

`media.Service` nhận `db.Tx`, `MediaRepo`, `storage.Storage`, `clock`, và các field của `config.Config` Phase 1: `MaxVideoBytes` (`MAX_VIDEO_BYTES`, 2 GiB), `MaxImageBytes` (`MAX_IMAGE_BYTES`, 10 MiB), `MediaURLTTL` (`MEDIA_URL_TTL`, 2h — giữ theo quyết định validation), `UploadTTL` cố định 15m. <!-- Updated: Validation Session 1 - MEDIA_URL_TTL 2h, config fields owned by Phase 1 -->

11. `InitUpload(ctx, actor, cmd{Kind, FileName, ContentType, SizeBytes}) ({MediaID, UploadURL, ExpiresAt}, error)`: allowlist `video/mp4` cho video; `image/png|jpeg|webp|gif` cho image; size ≤ max; key `media/{kind}/{yyyy}/{mm}/{id}{ext}`; `PresignPut` (ký kèm `Content-Type`; client đặt header `Content-Type` = file type khi PUT); insert `pending`, lưu `file_name`. Chỉ admin. <!-- Red Team: RT-07 - upload body/response contract -->
12. `CompleteUpload(ctx, actor, mediaID)`: `Stat(key)` (không có → `422` "Chưa nhận được file. Tải lên lại."); size/contentType khớp → `MarkReady` → `Update`. Trả `MediaDTO`.
13. `SignedURL(ctx, user, mediaID) ({URL, ExpiresAt}, error)`: `status=ready`; authz: admin hoặc `CanUserAccess`; `PresignGet(key, MediaURLTTL, contentType)`; `expiresAt = now + MediaURLTTL` để player Phase 13 ký lại trước khi hết hạn. Route nhóm `/media/*` có limiter theo user 120 req/phút (`platform/httpx/ratelimit`, `x/time/rate`), vượt → 429 `RATE_LIMITED`. <!-- Red Team: RT-15 - per-user limiter on /media/* -->
14. `RedirectContent(ctx, user, mediaID)`: như 13 nhưng handler trả `302 Location` + `Cache-Control: private, max-age=300` (ảnh markdown).

`MarkdownRenderer` impl (`stages/markdown.go`): goldmark `WithExtensions(extension.GFM)`, không `WithUnsafe`; sau render chạy policy dựng từ `bluemonday.NewPolicy()` (KHÔNG dùng `UGCPolicy()`/`AllowImages()` vì `AllowImages` cho phép mọi `img src` và `.Matching()` thêm vào không thu hẹp được — bluemonday sanitize.go:513-523): `AllowElements("p","br","h1","h2","h3","h4","ul","ol","li","strong","em","del","code","pre","blockquote","hr","table","thead","tbody","tr","th","td")`; `AllowAttrs("href").OnElements("a")` + `AllowURLSchemes("http","https","mailto")` + `RequireNoFollowOnLinks(true)` + `AddTargetBlankToFullyQualifiedLinks(true)`; `AllowAttrs("class").Matching(regexp.MustCompile(`^language-[a-z0-9]+$`)).OnElements("code")`; quy tắc duy nhất cho ảnh: `AllowAttrs("src").Matching(regexp.MustCompile(`^/api/v1/media/[0-9a-f-]{36}/content$`)).OnElements("img")` + `AllowAttrs("alt").OnElements("img")`. Ảnh nguồn ngoài (`https://x/a.png`) hoặc `data:` bị gỡ cả thẻ. CSP của API/web đặt `img-src 'self' blob:`. Client gửi `src` dưới dạng `/api/v1/media/{id}/content` ngay khi upload (Phase 11). <!-- Red Team: RT-15 - bluemonday NewPolicy, single img src rule -->

## HTTP API

Tất cả yêu cầu đăng nhập + `RequireRole(admin)` trừ `GET /media/{id}/url`, `GET /media/{id}/content` (mọi vai trò, authz trong service) và `GET /stage-versions/{vid}` (teacher/student đọc qua Phase 8 dùng DTO riêng, ở đây chỉ admin). Route phiên bản là route phẳng `/stage-versions/{vid}/...` (không lồng dưới `/stages/{id}`), khớp plan.md §7 và Phase 11. <!-- Red Team: RT-07 - flat version routes -->

DTO dùng chung (plan.md §7 là bản chuẩn; golden JSON của integration test commit tại `apps/api/internal/features/stages/testdata/*.json` và `.../media/testdata/*.json` để Phase 11 nạp vào MSW): <!-- Red Team: RT-07 - golden JSON shared with web -->

- `StageListItem = {id, code, name, description?, latestVersionNo, latestPublishedNo?, draftVersionId?, versions:[{id, versionNo, status, publishedAt?, lessonCount}], usedByCourseCount, outdatedCourseCount}`.
- `UsedByRow = {courseId, courseCode, courseName, courseVersionId, versionNo, status, classCodes:[string]}`; trên `GET /stages/{id}` thêm `stageVersionNo`, `outdated`.
- `OutdatedCourseRow = {courseId, courseCode, courseName, courseVersionId, courseVersionNo, usingVersionNo, latestVersionId, latestVersionNo, canApply, blockedReason?, draftVersionId?, draftVersionNo?}`.
- `StageVersionDTO = {id, stageId, stageCode, stageName, versionNo, status, publishedAt?, clonedFromVersionNo?, lessons:[LessonDTO], usedBy:[UsedByRow]}`.
- `LessonDTO = {id, lessonKey, title, type, required, position, durationSeconds?, markdownSource?, markdownHtml?, videoMediaId?, videoFileName?}`.
- `MediaDTO = {id, kind, fileName, contentType, sizeBytes, status}`.

| Endpoint | Request | Response | Lỗi |
|---|---|---|---|
| `GET /stages?q=` | – | `200 {items:[StageListItem]}` | – |
| `POST /stages` | `{code,name,description?}` | `201 StageDetailDTO` (kèm `versions[0]` là v1 draft) | 422 "Nhập mã và tên chặng."; 409 `CONFLICT` "Mã chặng đã tồn tại." |
| `GET /stages/{id}` | – | `200 {id,code,name,description?, versions:[{id,versionNo,status,lessonCount,publishedAt?,clonedFromVersionNo?}], usedBy:[UsedByRow+stageVersionNo,outdated], outdatedCourses:[OutdatedCourseRow]}` | 404 "Không tìm thấy chặng." |
| `GET /stage-versions/{vid}` | – | `200 StageVersionDTO` | 404 |
| `POST /stage-versions/{vid}/lessons` | `{lessonKey?,title,type,required,durationSeconds?,markdownSource?,videoMediaId?}` | `201 LessonDTO` | 422 (3 message); 409 `VERSION_IMMUTABLE` |
| `PATCH /stage-versions/{vid}/lessons/{lid}` | như trên, partial | `200 LessonDTO` | 404; 422; 409 |
| `DELETE /stage-versions/{vid}/lessons/{lid}` | – | 204 | 404; 409 |
| `PUT /stage-versions/{vid}/lessons/order` | `{lessonIds:[...]}` | `200 StageVersionDTO` | 422 "Danh sách sắp xếp không khớp."; 409 |
| `POST /stage-versions/{vid}/publish` | – | `200 StageVersionDTO` | 422 "Chặng cần ít nhất một học liệu trước khi phát hành."; 409 `INVALID_TRANSITION` "Chỉ phát hành được bản nháp." |
| `POST /stage-versions/{vid}/clone` | – | `201 StageVersionDTO` (bản nháp mới) | 409 `INVALID_TRANSITION` "Chỉ nhân bản từ phiên bản đã phát hành."; 409 `DRAFT_EXISTS` + `details:{draftVersionId,draftVersionNo}` |
| `POST /stage-versions/{vid}/archive` | – | `200 StageVersionDTO` | 409 "Chỉ lưu trữ phiên bản đã phát hành." |
| `DELETE /stage-versions/{vid}` | – | `200 {stageDeleted: bool}` (true khi đó là phiên bản cuối → chặng cũng bị xóa) | 409 `IN_USE` + `details:{usedBy:[UsedByRow]}`; 409 `INVALID_TRANSITION` |
| `POST /stage-versions/{vid}/apply` | – | – | thuộc Phase 6 (handler `courses`) |
| `POST /media/uploads` | `{kind,fileName,contentType,sizeBytes}` | `201 {mediaId,uploadUrl,expiresAt}` | 422 "Định dạng không hỗ trợ." / "File vượt giới hạn 2 GB." |
| `POST /media/uploads/{id}/complete` | – | `200 MediaDTO` | 404; 422 "Chưa nhận được file. Tải lên lại." |
| `GET /media/{id}/url` | – | `200 {url,expiresAt}` (`expiresAt = now + MEDIA_URL_TTL`) | 403; 404; 429 |
| `GET /media/{id}/content` | – | `302` → signed URL | 403; 404; 429 |

## Files to Create / Modify

```text
apps/api/internal/features/stages/
  entity.go            # Stage, StageVersion, Lesson, LessonContent, VideoContent, MarkdownContent, lỗi
  entity_test.go       # AddLesson/Publish/Clone/Reorder/Archive bảng trạng thái
  markdown.go markdown_test.go   # goldmark + bluemonday, bảng test XSS (script, onerror, data:, javascript:, ảnh ngoài)
  repository.go repository_pg.go # lessons scan: content_type switch; SaveDraft diff + lesson_media; TransitionStatus; ByIDForUpdate; NextVersionNo lock; AllOutdated
  repository_pg_test.go          # //go:build integration: one_draft index, guarded writes trên bản published (TestImmutability_*), publish guard trong SQL, deferrable reorder, CASCADE delete, RESTRICT delete, concurrent AddLesson vs Publish
  service.go service_test.go handler.go dto.go
  testdata/*.json                # golden JSON của StageListItem/StageDetail/StageVersionDTO/LessonDTO cho Phase 11 MSW <!-- Red Team: RT-07 -->
apps/api/internal/features/media/
  entity.go repository.go repository_pg.go service.go handler.go dto.go service_test.go testdata/*.json
apps/api/internal/platform/storage/{storage.go, minio.go, minio_test.go, mem.go}   # minio_test: presign offline với Region cố định <!-- Red Team: RT-08 -->
apps/api/internal/app/{router.go, deps.go}    # mount stages/media; limiter /media/*; CSP img-src 'self' blob:
```

Không sửa trong phase này: `platform/config` và `.env.example` (Phase 1 đã khai báo `S3_*`, `S3_PUBLIC_ENDPOINT`, `MEDIA_URL_TTL`, `MAX_VIDEO_BYTES`, `MAX_IMAGE_BYTES`), `platform/db/pgerr/constraints.go` (Phase 2 xuất `UqStagesCode`, `UqStageVersionsOneDraft`, `UqLessonsVersionPosition`, `UqLessonsVersionKey`), `infra/docker-compose.yml` (Phase 1 đã có `minio` với `MINIO_API_CORS_ALLOW_ORIGIN` và `minio-init` tạo bucket private), `cmd/lms/seed.go` (Phase 2 seed chặng/học liệu theo seed.js). <!-- Red Team: RT-11 - config and constraints single owner --> <!-- Red Team: RT-14 - seed owned by Phase 2 -->

## Tasks & Steps

1. Shared kernel đã có `VersionStatus`, `VersionNo`, `Code`, `LessonKey` (Phase 3); viết `entity.go` stages + test bảng: `AddLesson` trên published → `ErrVersionImmutable`; `Publish` rỗng → `ErrNoLessons`; `Publish` markdown rỗng → `Validate` lỗi; `Publish` khi markdown chưa `RenderMarkdown` → `ErrNotRendered`; `CloneAsDraft` giữ `lessonKey`, `MediaID`, `DurationSeconds`, `required`, position; `Reorder` thiếu id → lỗi. <!-- Red Team: RT-01 -->
2. `markdown.go` + test XSS; test ảnh `src=/api/v1/media/<uuid>/content` giữ, `https://x/a.png` và `data:image/png;base64,...` bị gỡ cả thẻ `img`; test policy không chấp nhận `<img>` không có `src` hợp lệ. <!-- Red Team: RT-15 -->
3. `platform/storage`: interface + minio-go impl; **cả hai** client (`S3_ENDPOINT` nội bộ và `S3_PUBLIC_ENDPOINT` cho presign vì chữ ký gắn host) đều `minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(...), Secure: cfg.S3UseSSL, Region: cfg.S3Region})` để presign không gọi `getBucketLocation` qua mạng (dev `us-east-1`, R2 `auto`); `minio_test.go` presign với endpoint giả `127.0.0.1:1` không mở kết nối và trả URL có `X-Amz-Signature`; `MemStorage` cho unit test. <!-- Red Team: RT-08 - Region on both clients, offline presign test -->
4. `media` feature: entity, repo, service, handler; `CanUserAccess` SQL ở mục Repository (JOIN `lesson_media`, học viên cần `cm.status='active' AND c.status IN ('active','ended')`, giáo viên theo `c.teacher_id`); limiter theo user 120/phút cho nhóm `/media/*`. <!-- Red Team: RT-15 -->
5. `stages` repo pg: `SaveDraft` diff lessons theo id rồi ghi lại `lesson_media` (video_media_id + UUID trích từ `markdown_html`) rồi UPDATE header; `TransitionStatus` header-only với `WHERE id=$1 AND status=$2`; `ByIDForUpdate`; `NextVersionNo` sau `SELECT ... FROM stages WHERE id=$1 FOR UPDATE`; `UsedBy` kèm `classCodes`; `OutdatedCourses` + `AllOutdated` dùng chung CTE; map lỗi qua `pgerr.Map(err)` (package `platform/db/pgerr`, Phase 2) rồi so constraint với `pgerr.UqStagesCode`, `pgerr.UqStageVersionsOneDraft`, `pgerr.UqLessonsVersionPosition`, `pgerr.UqLessonsVersionKey`; không có hàm map riêng trong feature. <!-- Red Team: RT-01 --> <!-- Red Team: RT-11 -->
6. `stages` service + handler + DTO; `contentFactory` map DTO → `LessonContent`; `OutdatedReader` adapter cho Phase 9.
7. Integration test (`//go:build integration`, schema thật của Phase 2, không trigger): (a) `TestImmutability_*`: gọi repository **trực tiếp, bỏ qua service** lên bản published: `SaveDraft` (sửa title), UPDATE/DELETE lesson, INSERT lesson, ghi `lesson_media`, `Delete` header → mỗi câu `ErrVersionImmutable`, dữ liệu không đổi (so snapshot trước/sau); (b) **publish chặng có học liệu markdown thành công** và `markdown_html` không NULL sau publish; (c) `TransitionStatus(draft→published)` khi còn markdown NULL (gọi repo trực tiếp, bỏ qua `v.Publish`) → 0 dòng → `ErrInvalidTransition` message "Phiên bản còn học liệu chưa render."; (d) archive published không chạm `lessons` và thành công; (e) **AddLesson song song với Publish** (hai tx, `ByIDForUpdate` tuần tự hóa): đúng một bên thành công, bên kia nhận `VERSION_IMMUTABLE`, không có lesson "mồ côi" trên bản published; (f) insert bản nháp thứ hai → `ErrDraftExists`; (g) reorder đổi chỗ hai lesson không lỗi unique; (h) delete published được dùng → `ErrInUse` với `UsedBy` đúng tên "Lập trình cơ bản v1"; (i) delete bản nháp cuối → stage bị xóa; (j) `OutdatedCourses`/`AllOutdated`: nền là seed.js (chặng `DB` v1 published, khóa học `BASIC` v1 published dùng DB v1); test tự tạo qua `testdb` helpers (seed không có): DB v2 published (làm BASIC thành outdated), một khóa học thứ hai `UPTODATE` dùng DB v2 (không outdated) và một khóa học thứ ba `HASDRAFT` dùng DB v1 kèm bản nháp (blockedReason); (k) `lesson_media` có đúng các media_id của video + ảnh markdown sau `SaveDraft`; (l) `TransitionStatus(published→draft)` và `(archived→published)` → `ErrInvalidTransition`, chỉ `published→archived` thành công và set `archived_at`. Các test `TestImmutability_*` được Phase 14 H5 gọi lại qua `make hardening-immutability`. <!-- Red Team: RT-01 - write-order tests --> <!-- Red Team: RT-03 - fixtures from seed.js --> <!-- Updated: Session 2 - D1 -->
8. Ghi golden JSON `testdata/*.json` từ integration test (dữ liệu seed.js) để Phase 11 dùng cho MSW. <!-- Red Team: RT-07 -->
9. Kiểm tra end-to-end dev: `docker compose --profile full up`, trình duyệt tại `http://localhost:5173` PUT thẳng lên `uploadUrl` thành công nhờ `MINIO_API_CORS_ALLOW_ORIGIN` của Phase 1 (không có `mc cors set`). <!-- Red Team: RT-08 -->
10. Lint, test; bổ sung mục R2 (`S3_REGION=auto`, `S3_USE_SSL=true`, CORS bucket qua Cloudflare dashboard) vào `docs/runbook.md` khi Phase 14 tổng hợp — phase này chỉ ghi chú trong `docs/architecture.md` phần storage. <!-- Updated: Validation Session 1 - R2 prod -->

## Verification

```bash
cd apps/api && go test ./internal/features/stages/... ./internal/features/media/... ./internal/platform/storage/...
cd apps/api && go test -tags integration ./internal/features/stages/... ./internal/features/media/...
make dev && make seed   # đăng nhập admin, lưu cookie c.txt, H='-H X-Requested-With:fetch -H Content-Type:application/json'
curl -s -b c.txt $H -d '{"code":"DOCKER","name":"Docker cơ bản"}' localhost:8080/api/v1/stages | jq '{id, draft: .versions[0].versionNo}'   # 201, v1 draft (placeholder của prototype)
VID=<draft id>
curl -s -b c.txt $H -X POST localhost:8080/api/v1/stage-versions/$VID/publish | jq .error.message        # "Chặng cần ít nhất một học liệu trước khi phát hành."
curl -s -b c.txt $H -d '{"title":"Giới thiệu","type":"markdown","required":true,"markdownSource":"# Hi <script>alert(1)</script> ![x](https://x/a.png)"}' localhost:8080/api/v1/stage-versions/$VID/lessons -w '%{http_code}'  # 201
curl -s -b c.txt $H -X POST localhost:8080/api/v1/stage-versions/$VID/publish | jq '.lessons[0].markdownHtml'   # không chứa <script>, không chứa <img>
curl -s -b c.txt $H -d '{"title":"x","type":"markdown","required":true,"markdownSource":"y"}' localhost:8080/api/v1/stage-versions/$VID/lessons | jq .error.code  # VERSION_IMMUTABLE
curl -s -b c.txt $H -X POST localhost:8080/api/v1/stage-versions/$VID/clone | jq '.versionNo'            # 2
curl -s -b c.txt $H -X POST localhost:8080/api/v1/stage-versions/$VID/clone | jq '.error | {code, details}'  # DRAFT_EXISTS, draftVersionId
curl -s -b c.txt $H -X POST localhost:8080/api/v1/stage-versions/$VID/archive | jq .status                # archived (header-only)
# upload video
curl -s -b c.txt $H -d '{"kind":"video","fileName":"a.mp4","contentType":"video/mp4","sizeBytes":1048576}' localhost:8080/api/v1/media/uploads | tee up.json   # {mediaId, uploadUrl, expiresAt}
curl -s -X PUT -H 'Content-Type: video/mp4' --data-binary @sample.mp4 "$(jq -r .uploadUrl up.json)" -w '%{http_code}'   # 200
curl -s -b c.txt $H -X POST localhost:8080/api/v1/media/uploads/$(jq -r .mediaId up.json)/complete | jq .status      # ready
curl -s -b c.txt localhost:8080/api/v1/media/$(jq -r .mediaId up.json)/url | jq .expiresAt   # ≈ now + 2h
# bucket không public
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:9000/lms-media/media/video/...   # 403
# presign offline + CORS từ web dev
cd apps/api && go test ./internal/platform/storage/ -run TestPresignOffline   # không mở socket
# mở http://localhost:5173, thêm học liệu video → PUT lên http://localhost:9000 không lỗi CORS (MINIO_API_CORS_ALLOW_ORIGIN từ Phase 1)
```

## Security notes

- Markdown render + sanitize ở server khi publish; nguồn `markdown_source` giữ nguyên để sửa trong bản nháp sau khi clone; client không bao giờ render source thô của người khác.
- Bucket private; presigned PUT giới hạn `Content-Type` + `Content-Length` qua header ký; presigned GET TTL `MEDIA_URL_TTL`; không log URL ký (chứa chữ ký) ở mức info.
- `CanUserAccess` kiểm qua `lesson_media` (không tìm chuỗi trong HTML) theo lớp giáo viên phụ trách hoặc lớp học viên đang `active`; admin xem tất cả. Lớp đã kết thúc: học viên vẫn xem được (view-only, FR-22) → điều kiện `c.status IN ('active','ended')` cho học viên; lớp `draft` không mở media. <!-- Red Team: RT-15 -->
- CSP `img-src 'self' blob:`; policy bluemonday chỉ cho `img src` nội bộ, nên không có ảnh ngoại tuyến rò IP học viên. <!-- Red Team: RT-15 -->
- `/media/*` có limiter theo user 120/phút để chặn quét media_id. <!-- Red Team: RT-15 -->
- Upload chỉ admin; `complete` kiểm `Stat` để tránh tạo record `ready` cho key không tồn tại.

## Risks & Rollback

| Rủi ro | Biện pháp |
|---|---|
| Dev dùng MinIO community, prod dùng Cloudflare R2 | Interface `Storage` chuẩn S3 (minio-go v7); không spike, không ADR; prod đặt `S3_ENDPOINT=https://<account>.r2.cloudflarestorage.com`, `S3_REGION=auto`, `S3_USE_SSL=true`; CORS bucket R2 cấu hình trên dashboard và ghi trong `docs/runbook.md` (Phase 14) | <!-- Updated: Validation Session 1 - R2 -->
| Presign host khác host trình duyệt thấy / presign gọi mạng | Client minio thứ hai với `S3_PUBLIC_ENDPOINT`; cả hai client đặt `Region` nên presign thuần cục bộ | <!-- Red Team: RT-08 -->
| Một câu ghi lên `stage_versions`/`lessons`/`lesson_media` thiếu điều kiện trạng thái → sửa được bản published vì DB không còn trigger | Hợp đồng SQL có điều kiện + `db.ExecAffectOne`; review theo checklist `docs/database.md`; test (a)(c)(e)(l) gọi repository trực tiếp | <!-- Red Team: RT-01 --> <!-- Updated: Session 2 - D1 -->
| Race clone song song tạo hai draft | Lock stage row + partial unique index; `pgerr.Map` 23505 `uq_stage_versions_one_draft` → `DRAFT_EXISTS` |
| Video lớn upload qua trình duyệt timeout | Presigned PUT trực tiếp lên storage, TTL 15m; multipart là backlog |

Rollback: không mount `stages`/`media` trong router; dữ liệu đã tạo giữ nguyên (schema Phase 2).

## Success Criteria

- [x] `go test` unit + integration xanh cho `stages`, `media`, `platform/storage`.
- [x] Tạo chặng → có đúng một `stage_versions` v1 `draft`; tạo mã trùng → 409.
- [x] Thêm/sửa/xóa/sắp học liệu trên bản published → 409 `VERSION_IMMUTABLE` (kể cả khi gọi repo trực tiếp bỏ qua service, nhờ SQL có điều kiện; `TestImmutability_*` xanh). <!-- Updated: Session 2 - D1 -->
- [x] Publish không học liệu → 422 đúng message; publish có học liệu markdown → thành công trên schema thật, `markdown_html` đã sanitize và không NULL, `published_at` set, audit `stage_version.published`; archive bản published thành công mà không chạm `lessons`. <!-- Red Team: RT-01 -->
- [x] AddLesson song song với Publish: đúng một bên thành công, không có học liệu mồ côi trên bản published. <!-- Red Team: RT-01 -->
- [x] Tạo chặng ghi audit `stage.created`; xóa bản nháp cuối cùng xóa luôn chặng và trả `stageDeleted:true`. <!-- Red Team: RT-07 -->
- [x] `OutdatedReader.AllOutdated` trả đúng tập hợp cho dashboard Phase 9. <!-- Red Team: RT-07 -->
- [x] Clone → v(n+1) draft, lessons cùng `lesson_key`/`video_media_id`/`required`/`position`; clone lần hai → 409 `DRAFT_EXISTS` kèm `draftVersionId`.
- [x] Delete published được khóa học dùng → 409 `IN_USE` với `usedBy` tên khóa học + version; delete draft → 204 và lessons CASCADE. (Thực tế: 200 `{stageDeleted}` theo bảng endpoint.)
- [x] `GET /stages/{id}` trả `outdatedCourses` khớp fixture 3 khóa học (một `canApply=false` với blockedReason).
- [x] Upload handshake: body `{kind,fileName,contentType,sizeBytes}`, sai content type → 422; PUT thành công → `complete` → `ready`; `GET /media/{id}/url` trả `expiresAt` ≈ now + 2h; học viên không thuộc lớp hoặc lớp còn nháp → 403; media ảnh markdown được cấp quyền qua `lesson_media`. <!-- Red Team: RT-15 --> <!-- Updated: Validation Session 1 - TTL 2h -->
- [x] Presign không mở kết nối mạng (unit test); PUT từ `http://localhost:5173` không lỗi CORS. <!-- Red Team: RT-08 -->
- [x] Bộ test XSS markdown: `<script>`, `onerror`, `javascript:`, `data:` URI, ảnh ngoài đều bị gỡ; policy dựng từ `NewPolicy()` không gọi `AllowImages()`. <!-- Red Team: RT-15 -->
