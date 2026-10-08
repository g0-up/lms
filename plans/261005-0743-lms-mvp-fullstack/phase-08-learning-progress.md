---
phase: 08
title: "Phase 8: Học tập và tiến độ"
status: pending
priority: P1
effort: "3 ngày"
dependencies: [7]
---

# Phase 8: Học tập và tiến độ

## Goal

Hoàn thành feature `learning` cho học viên: danh sách lớp của tôi với % tiến độ và học liệu kế tiếp, roadmap lớp (chặng → học liệu, trạng thái từng học liệu, % mỗi chặng và tổng), mở học liệu (ghi `first_opened_at` một lần, trả nội dung video URL ký hoặc HTML đã sanitize), tích/bỏ tích hoàn thành idempotent với đầy đủ kiểm tra server. Cung cấp `learning.ProgressReader` cho Phase 9 (báo cáo) và Phase 7 (`/teach/classes`: `avgPercent`, `notLoggedIn`, `inactiveOver7Days`).

<!-- Red Team: RT-06 - hợp đồng API học viên là plan.md §7; golden JSON cho Phase 13 -->
Hợp đồng API học viên (`/me/*`) ở phase này là bản canonical của `plan.md` §7; Phase 13 (web học viên) sao chép zod schema và MSW fixture từ golden JSON của integration test ở `apps/api/internal/features/learning/testdata/*.json` do phase này commit.

## Context & Requirements

FR phủ: FR-30, FR-31, FR-32, FR-33. NFR §8: kiểm quyền ở server, URL video ký.

Quy tắc spec:

<!-- Red Team: RT-06 - lớp draft vẫn thấy (chỉ đọc), dropped/không phải thành viên → 404 -->
- Học viên xem lớp mình là thành viên `class_members.status = active` ở mọi trạng thái lớp `draft | active | ended`; lớp `draft` và `ended` chỉ đọc (`readOnly=true`, `readOnlyReason='draft'|'ended'`); `dropped` hoặc không phải thành viên → `404 NOT_FOUND` (theo prototype `activeMembersOf`: không liệt kê lớp đã rời). Roadmap: chặng theo thứ tự trong `course_version_stages`, học liệu theo `position`; không khóa tuần tự.
- Trạng thái học liệu: `not_opened | opened | completed`. Mở lần đầu ghi `first_opened_at`, lần sau không đổi.
- Hoàn thành chỉ sau khi đã mở; tích → `completed_at = now`, bỏ tích → `NULL`; idempotent (tích lại khi đã tích không đổi `completed_at`). Server kiểm: học liệu thuộc `course_version` của lớp (không → 404); thành viên `active`; lớp `active`.
<!-- Red Team: RT-12 - Percent dùng shared kernel, làm tròn half away from zero khớp Postgres round() -->
- % = học liệu `required` đã hoàn thành / tổng `required`; không cộng học liệu tùy chọn; tính khi đọc, không lưu. 0/0 → 0% (prototype `pct = total ? round(done/total*100) : 0`). Dùng `domain.Percent(done, total)` của shared kernel Phase 3: `0` khi `total==0`, ngược lại `int(math.Round(float64(done)*100/float64(total)))` kẹp 0..100; làm tròn half away from zero trùng `round(numeric)` của Postgres nên SQL và Go cho cùng kết quả ((2,3)=67, (1,3)=33, (1,2)=50).
- Lớp `ended`: xem được roadmap và nội dung, không ghi `first_opened_at` mới, không tích (`readOnlyReason='ended'`). Lớp `draft`: học viên thấy thẻ lớp và roadmap chỉ đọc (prototype: "Lớp bắt đầu … Bạn sẽ vào học được khi lớp kích hoạt."), không ghi `first_opened_at`, không tích (`readOnlyReason='draft'`).
- Mỗi request học viên hợp lệ cập nhật `users.last_active_at` (throttle 1 phút như middleware session).

Copy tiếng Việt từ `prototype/app.js`:

| Tình huống | Message |
|---|---|
| Dropped / không phải thành viên (API trả `404 NOT_FOUND`; FE Phase 13 hiển thị câu này) | "Bạn không còn là thành viên đang học của lớp." |
| Học liệu không thuộc phiên bản khóa học của lớp (API trả `404 NOT_FOUND`; FE Phase 13 hiển thị câu này) | "Học liệu không thuộc phiên bản khóa học của lớp." |
| Tích trước khi mở | "Mở học liệu trước khi tích hoàn thành." |
| Lớp chưa chạy / đã kết thúc khi tích | "Lớp chưa bắt đầu hoặc đã kết thúc; không ghi nhận tiến độ." (mới) |
| Toast tích | "Đã ghi nhận hoàn thành." |
| Toast bỏ tích | "Đã bỏ tích." |
| Ghi chú | "Tiến độ do học viên tự xác nhận." (FR-42, hiển thị cả ở FE học viên) |

## Domain model

### Aggregate `LessonProgress` (nhỏ, identity = (memberID, lessonID))

```go
type LessonProgress struct { memberID, lessonID ids.ID; firstOpenedAt time.Time; completedAt *time.Time }
func OpenLesson(memberID, lessonID ids.ID, now time.Time) *LessonProgress        // dùng với INSERT ... ON CONFLICT DO NOTHING
func (p *LessonProgress) SetCompleted(completed bool, now time.Time) (changed bool) // idempotent; completed && completedAt!=nil → không đổi
func (p *LessonProgress) State() LessonState                                       // opened | completed
```

### Read model / VO

```go
// domain.Percent(done, total int) int (shared kernel Phase 3, RT-12): 0..100; total==0 → 0; math.Round half away from zero
type LessonView struct { ID ids.ID; Key domain.LessonKey; Title string; Type string; Position int; Required bool; State LessonState; FirstOpenedAt, CompletedAt *time.Time; DurationSeconds *int }
type StageView struct { StageID, StageVersionID ids.ID; Code, Name string; VersionNo domain.VersionNo; Position int; Lessons []LessonView; Percent int; RequiredDone, RequiredTotal int }
type Roadmap struct { Class ClassSummary; Stages []StageView; Percent int; RequiredDone, RequiredTotal int; NextLesson *LessonRef /* lessonId, title, stageName */; LastActivityAt *time.Time; ReadOnly bool; ReadOnlyReason string /* "" | draft | ended */; SelfReported bool /* luôn true */ }
func BuildRoadmap(cls ClassSummary, stages []StageView, progress map[ids.ID]*LessonProgress, now time.Time) Roadmap // hàm thuần: tính Percent từng stage và tổng qua domain.Percent, NextLesson = học liệu đầu tiên chưa completed theo thứ tự stage→position, ReadOnly/ReadOnlyReason theo cls.Status
```

### Guard `LearningContext` (Specification cho mọi use case)

```go
type LearningContext struct { Member classes.MemberContext; User identity.UserSnapshot }
func (lc LearningContext) CanView() error    // member active ở lớp draft|active|ended → nil; không phải thành viên hoặc dropped → ErrNotFound
func (lc LearningContext) CanRecord() error  // CanView + class active → nil; class draft|ended → ErrClassNotActive
func (lc LearningContext) ReadOnlyReason() string // "" | "draft" | "ended"
```

<!-- Red Team: RT-06 - mã lỗi thống nhất với Phase 13: 404 NOT_FOUND, 409 CONFLICT, 409 INVALID_TRANSITION -->
Lỗi → apperr (giống hệt Phase 13): không phải thành viên / dropped / lớp không tồn tại / học liệu không thuộc `course_version` của lớp (`ErrNotFound`, `ErrLessonNotInCourse`) → `404 NOT_FOUND` (không lộ sự tồn tại lớp hay học liệu; FE hiển thị "Bạn không còn là thành viên đang học của lớp." hoặc "Học liệu không thuộc phiên bản khóa học của lớp." theo ngữ cảnh); `ErrNotOpened` → `409 CONFLICT` "Mở học liệu trước khi tích hoàn thành."; `ErrClassNotActive` → `409 INVALID_TRANSITION` "Lớp chưa bắt đầu hoặc đã kết thúc; không ghi nhận tiến độ.". Không dùng `403 FORBIDDEN` hay `422` cho các trường hợp này.

Mẫu: Repository, Unit of Work, Specification (`LearningContext`), Read model thuần (`BuildRoadmap` test không DB), Strategy tái dùng `LessonContent` để dựng nội dung trả về (video → `media.SignedURL`; markdown → `html`). SOLID: `learning` phụ thuộc `classes.MembershipReader`, `media.URLSigner`, `stages.LessonReader` (interface khai báo ở các feature cung cấp); `ProgressReader` cung cấp cho `reports`/`classes` là interface khai báo ở `learning`.

## Repository interfaces

```go
package learning
type ProgressRepo interface {
    Open(ctx context.Context, ex db.Executor, p *LessonProgress) (inserted bool, err error)
    // INSERT INTO lesson_progress(class_member_id, lesson_id, first_opened_at) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING
    Get(ctx context.Context, ex db.Executor, memberID, lessonID ids.ID) (*LessonProgress, error) // nil,nil khi chưa mở
    GetForUpdate(ctx context.Context, ex db.Executor, memberID, lessonID ids.ID) (*LessonProgress, error)
    SetCompleted(ctx context.Context, ex db.Executor, p *LessonProgress) error  // UPDATE completed_at
    ByMember(ctx context.Context, ex db.Executor, memberID ids.ID) (map[ids.ID]*LessonProgress, error)
}
type CourseStructureRepo interface {   // read-only, SQL trực tiếp qua stage_versions/lessons/course_version_stages
    StagesOfCourseVersion(ctx context.Context, ex db.Executor, courseVersionID ids.ID) ([]StageView, error) // lessons chưa gắn state
    LessonInCourseVersion(ctx context.Context, ex db.Executor, courseVersionID, lessonID ids.ID) (*LessonRow, error) // nil → ErrLessonNotInCourse (→ 404)
}
type MyClassesRepo interface {
    // RT-06: lớp draft cũng được liệt kê; dropped bị loại ở WHERE
    ListForStudent(ctx context.Context, ex db.Executor, userID ids.ID) ([]MyClassRow, error) // JOIN class_members(cm.status='active') + classes(c.status IN ('draft','active','ended')) + course + teacher; kèm requiredTotal, requiredDone (SQL dưới)
}
// Interface sang feature khác
// classes.MembershipReader.MemberContext(ctx, ex, classID, userID) (MemberContext, error)
// media.URLSigner.SignedURLFor(ctx, ex, user, mediaID) (URLWithExpiry, error)   // Phase 5 use case 13
// learning.ProgressReader (cho reports/classes):
type ProgressReader interface {
    ClassProgress(ctx context.Context, ex db.Executor, classID ids.ID) ([]MemberProgress, error)      // mỗi member: percent tổng, percent từng stage, lastActivityAt
    MemberLessonProgress(ctx context.Context, ex db.Executor, classID, memberID ids.ID) (Roadmap, error) // drilldown; WHERE cm.id=$2 AND cm.class_id=$1, không khớp → ErrNotFound
    ClassAveragePercent(ctx context.Context, ex db.Executor, classIDs []ids.ID) (map[ids.ID]int, error)
    ClassActivityCounts(ctx context.Context, ex db.Executor, classIDs []ids.ID, staleDays int) (map[ids.ID]ActivityCounts, error) // notLoggedIn, inactiveOverStaleDays cho Phase 7 /teach/classes (STALE_DAYS=7)
}
```

SQL tính % cho `ListForStudent` / `ClassProgress` (tính khi đọc, không cột lưu):

```sql
WITH req AS (
  SELECT cvs.course_version_id, cvs.stage_id, l.id lesson_id
  FROM course_version_stages cvs JOIN lessons l ON l.stage_version_id = cvs.stage_version_id AND l.required
),
tot AS (SELECT course_version_id, count(*) required_total FROM req GROUP BY 1),
done AS (
  SELECT cm.id class_member_id, count(*) FILTER (WHERE lp.completed_at IS NOT NULL) required_done,
         max(greatest(lp.first_opened_at, lp.completed_at)) last_activity_at
  FROM class_members cm JOIN classes c ON c.id = cm.class_id
  JOIN req ON req.course_version_id = c.course_version_id
  LEFT JOIN lesson_progress lp ON lp.class_member_id = cm.id AND lp.lesson_id = req.lesson_id
  WHERE cm.class_id = $1 GROUP BY cm.id
)
SELECT cm.id, cm.user_id, coalesce(done.required_done,0) required_done, coalesce(tot.required_total,0) required_total,
       CASE WHEN coalesce(tot.required_total,0)=0 THEN 0 ELSE round(100.0*done.required_done/tot.required_total) END percent,
       done.last_activity_at
FROM class_members cm JOIN classes c ON c.id=cm.class_id
LEFT JOIN tot ON tot.course_version_id=c.course_version_id LEFT JOIN done ON done.class_member_id=cm.id
WHERE cm.class_id=$1;
```

% từng chặng: cùng CTE nhóm theo `(class_member_id, stage_id)`; trả `jsonb_object_agg(stage_id, percent)` hoặc rows riêng, service gộp. `round(100.0*…)` của Postgres và `domain.Percent` cho cùng kết quả (RT-12); test (h) ở Tasks kiểm điều này.

## Use cases / Service methods

`learning.Service` nhận `db.Tx`, `ProgressRepo`, `CourseStructureRepo`, `MyClassesRepo`, `classes.MembershipReader`, `media.URLSigner`, `identity.ActivityRecorder` (`Touch(userID, now)`), `clock.Clock`, `ids` không cần (PK hợp).

<!-- Red Team: RT-06 - DTO khớp từng trường với plan.md §7 và Phase 13 -->
1. `MyClasses(ctx, student) ([]MyClassItemDTO, error)`: `ListForStudent` → mỗi item `{id, code, name, status:'draft'|'active'|'ended', startDate, endDate, teacherName, courseName, courseVersionNo, percent, requiredDone, requiredTotal, nextLesson?:{lessonId, title, stageName}, readOnlyReason?:'draft'|'ended'}`. `nextLesson` tính bằng một truy vấn phụ `FirstIncompleteLesson(memberID)` (ORDER BY cvs.position, l.position LIMIT 1 WHERE lp.completed_at IS NULL). Lớp `draft` và `ended` vẫn trả (kèm `readOnlyReason`); membership `dropped` không trả.
2. `Roadmap(ctx, student, classID) (RoadmapDTO, error)`: `MemberContext` → `CanView()` (không phải thành viên / dropped / lớp không tồn tại → 404 `NOT_FOUND`); `StagesOfCourseVersion` + `ByMember` → `BuildRoadmap`; `ReadOnly = class.status != active`, `ReadOnlyReason = 'draft'|'ended'`; `SelfReported = true`; `Touch` user. Lớp `draft` → 200 chỉ đọc, không phải 404.
3. `OpenLesson(ctx, student, classID, lessonID) (LessonContentDTO, error)`:
   1. `MemberContext` → `CanView()` (→ 404).
   2. `LessonInCourseVersion` → nil → 404 `NOT_FOUND`.
   3. Nếu `CanRecord() == nil` (lớp active, member active): Transact `Open(OpenLesson(memberID, lessonID, now))` (ON CONFLICT DO NOTHING — lần sau không đổi). Lớp `draft`/`ended`: bỏ qua ghi.
   4. Dựng nội dung: video → `URLSigner.SignedURLFor(user, mediaID)` → `{type:"video", mediaId, url, expiresAt}`; markdown → `{type:"markdown", html}` (lấy `markdown_html` đã sanitize; nếu NULL vì phiên bản chưa publish — không thể xảy ra với lớp hợp lệ — trả 500 có log). FE ký lại URL video hết hạn qua `GET /media/{mediaId}/url` (Phase 5).
   5. `Touch` user; trả `{lesson:{id,title,type,required,position,durationSeconds?,stage:{id,name}}, content, progress:{firstOpenedAt, completedAt?}, readOnly, prev?:{lessonId,title}, next?:{lessonId,title}}`.
4. `SetCompletion(ctx, student, classID, lessonID, completed bool) (CompletionDTO, error)`:
   1. `MemberContext` → `CanRecord()` (không phải thành viên / dropped → 404; class `draft`/`ended` → 409 `INVALID_TRANSITION`).
   2. `LessonInCourseVersion` → nil → 404.
   3. Transact: `GetForUpdate` nil → 409 `CONFLICT` "Mở học liệu trước khi tích hoàn thành."; `p.SetCompleted(completed, now)` → nếu `changed` → `SetCompleted`; `Touch`.
   4. Trả `{completedAt: string|null, percent, requiredDone, requiredTotal}` (percent tổng toàn khóa sau thay đổi). Toast "Đã ghi nhận hoàn thành." / "Đã bỏ tích." do FE hiển thị (Phase 13). Idempotent: 200 cả khi không đổi.
5. `ProgressReader` impl (dùng SQL ở trên) — không qua HTTP; test integration với fixture.

Tất cả use case học viên yêu cầu `RequireRole(student)`; admin/teacher không dùng `/me/*` (plan.md: một vai trò mỗi user).

## HTTP API

| Endpoint | Vai trò | Request | Response | Lỗi |
|---|---|---|---|---|
<!-- Red Team: RT-06 - bảng route là bản canonical plan.md §7; Phase 13 zod schema mirror -->
| `GET /me/classes` | student | – | `200 {items:[{id, code, name, status:'draft'\|'active'\|'ended', startDate, endDate, teacherName, courseName, courseVersionNo, percent, requiredDone, requiredTotal, nextLesson?:{lessonId,title,stageName}, readOnlyReason?:'draft'\|'ended'}]}` (gồm lớp `draft`; không gồm membership `dropped`) | – |
| `GET /me/classes/{id}` | student | – | `200 {class:{id,code,name,status,startDate,endDate,teacherName,courseName,courseVersionNo}, percent, requiredDone, requiredTotal, readOnly, readOnlyReason?, nextLesson?:{lessonId,title,stageName}, stages:[{id,name,position,lessons:[{id,title,type:'markdown'\|'video',required,position,durationSeconds?,firstOpenedAt?,completedAt?}]}], selfReported:true}` (lớp `draft` → 200 `readOnly=true`, `readOnlyReason='draft'`) | 404 `NOT_FOUND` |
| `GET /me/classes/{id}/lessons/{lid}` | student | – | `200 {lesson:{id,title,type,required,position,durationSeconds?,stage:{id,name}}, content:{type:'markdown',html}\|{type:'video',mediaId,url,expiresAt}, progress:{firstOpenedAt,completedAt?}, readOnly, prev?:{lessonId,title}, next?:{lessonId,title}}` (ghi `first_opened_at` nếu lớp active) | 404 `NOT_FOUND` (không thành viên / học liệu không thuộc course version) |
| `PUT /me/classes/{id}/lessons/{lid}/completion` | student | `{completed:bool}` | `200 {completedAt?:string\|null, percent, requiredDone, requiredTotal}` | 404 `NOT_FOUND`; 409 `CONFLICT` "Mở học liệu trước khi tích hoàn thành."; 409 `INVALID_TRANSITION` "Lớp chưa bắt đầu hoặc đã kết thúc; không ghi nhận tiến độ." |

`Cache-Control: no-store` cho mọi response `/me/*` (chứa URL ký). Mọi mutation nhận qua `http.ts` của Phase 10 kèm `X-Requested-With: fetch` (RT-05, middleware CSRF Phase 04 kiểm).

## Files to Create / Modify

```text
apps/api/internal/features/learning/
  entity.go            # LessonProgress, LessonState, LearningContext, lỗi
  roadmap.go           # StageView/LessonView/Roadmap + BuildRoadmap (thuần)
  entity_test.go roadmap_test.go   # SetCompleted idempotent; BuildRoadmap: percent theo required, 0/0 → 0, nextLesson, ended → ReadOnly
  repository.go repository_pg.go   # ProgressRepo, CourseStructureRepo, MyClassesRepo, ProgressReader impl (SQL CTE)
  repository_pg_test.go            # //go:build integration: ON CONFLICT DO NOTHING, percent SQL khớp BuildRoadmap trên cùng fixture
  service.go service_test.go service_integration_test.go
  handler.go dto.go
  testdata/my-classes.json testdata/my-class-basic01.json testdata/lesson-video.json testdata/lesson-markdown.json testdata/completion.json   # golden JSON từ integration test (RT-06); Phase 13 import làm MSW fixture
apps/api/internal/features/classes/membership_reader.go   # impl classes.MembershipReader (MemberContext một JOIN)
apps/api/internal/features/media/url_signer.go            # expose SignedURLFor (bọc use case 13 Phase 5)
apps/api/internal/features/identity/activity.go           # ActivityRecorder.Touch (UPDATE users SET last_active_at WHERE last_active_at < now - 1m)
apps/api/internal/app/{router.go, deps.go}                # wire learning; nối learning.ProgressReader vào classes (avgPercent, notLoggedIn, inactiveOver7Days) và reports
```

<!-- Red Team: RT-14 - không sửa seed.go; fixture test qua testdb helper đặt tên theo seed.js -->
Phase này **không** sửa `apps/api/cmd/lms/seed.go` (Phase 02 sở hữu port đầy đủ của `prototype/seed.js`, gồm `lesson_progress`: An ~79%, Dũng chưa đăng nhập, Khang ngưng hoạt động 21 ngày, Thảo đã rời lớp). Integration test dùng `testdb` helper chèn tối thiểu theo tên thực thể seed.js: lớp `basic01` (active, giảng viên `huong.le`), `basic03` (draft), một lớp `ended` chèn riêng trong test (seed.js không có lớp `ended`), học viên `an.nguyen@gmail.com`, chặng `Database` với học liệu "Giới thiệu SQL" (video 18:24), "Thiết kế bảng và khóa", "Đọc thêm: chỉ mục" (không bắt buộc).

## Tasks & Steps

1. `entity.go` + `roadmap.go` + unit test: `SetCompleted(true)` hai lần → `changed=false` lần hai, `completedAt` giữ; `SetCompleted(false)` → nil; `BuildRoadmap` với fixture 2 chặng (3 required + 1 optional; 2 required) → percent chặng 67/0, tổng 40 (2/5); optional completed không đổi %; `nextLesson` là học liệu required/optional đầu tiên chưa completed theo thứ tự.
2. `repository_pg.go`: `Open` với `ON CONFLICT DO NOTHING` trả `inserted` qua `RowsAffected`; `StagesOfCourseVersion` một truy vấn JOIN ORDER BY `cvs.position, l.position`; `LessonInCourseVersion`; `ListForStudent` (gồm `draft`, loại `dropped`); `ProgressReader` CTE; `FirstIncompleteLesson`; `ClassActivityCounts(staleDays=7)`. Lỗi Postgres ánh xạ qua `pgerr.Map(err)` và các hằng constraint export của gói `platform/db/pgerr` (`constraints.go` Phase 02, `map.go` Phase 03), không có `db.MapErr`.
3. `classes.MembershipReader`, `media.URLSigner`, `identity.ActivityRecorder` adapters.
4. `service.go` use case 1–4; handler; DTO; `no-store`.
<!-- Red Team: RT-06 - test theo mã lỗi 404/409; lớp draft 200 chỉ đọc; ghi golden JSON -->
5. Integration test (fixture `testdb` theo seed.js, học viên An ở `basic01`): (a) mở học liệu lần 1 → `first_opened_at` set, lần 2 sau 1 phút → không đổi; (b) tích trước khi mở → 409 `CONFLICT`; (c) tích → `completed_at`, tích lại → không đổi, bỏ tích → NULL, bỏ tích lại → 200 không đổi; (d) học liệu của khóa học khác → 404 `NOT_FOUND`; (e) member dropped → 404 `NOT_FOUND` ở cả 3 route và không xuất hiện trong `GET /me/classes`; (f) lớp ended (chèn trong test): GET roadmap 200 `readOnly=true, readOnlyReason='ended'`, GET lesson 200 không ghi `first_opened_at` mới, PUT → 409 `INVALID_TRANSITION`; (g) lớp `basic03` draft: có trong `GET /me/classes` với `readOnlyReason='draft'`, GET roadmap 200 `readOnly=true`, PUT → 409 `INVALID_TRANSITION`; (h) % SQL == `domain.Percent`/`BuildRoadmap` trên cùng dữ liệu, kể cả (2,3)=67 và (1,2)=50; (i) `last_active_at` cập nhật; (j) teacher gọi `/me/classes` → 403; (k) response của 4 route được ghi golden vào `testdata/*.json` (so khớp byte khi chạy lại; cập nhật có chủ đích bằng flag `-update`).
6. Cung cấp `ProgressReader` cho Phase 9 (lọc "chưa đăng nhập", "không hoạt động > 7 ngày", "dưới N%") và Phase 7 (`/teach/classes`); dữ liệu demo đã nằm trong seed Phase 02.
7. Lint, test.

## Verification

```bash
cd apps/api && go test ./internal/features/learning/...
cd apps/api && go test -tags integration ./internal/features/learning/...
make dev && make seed   # login học viên active an.nguyen@gmail.com → s.txt; H='-H Content-Type:application/json -H X-Requested-With:fetch'
curl -s -b s.txt localhost:8080/api/v1/me/classes | jq '.items[] | {code, status, percent, readOnlyReason, next: .nextLesson.title}'   # basic01 active; basic03 draft readOnlyReason=draft
CL=<id lớp basic01>; LID=<id học liệu "Thiết kế bảng và khóa" chưa mở>
curl -s -b s.txt $H -X PUT -d '{"completed":true}' localhost:8080/api/v1/me/classes/$CL/lessons/$LID/completion | jq '{code: .error.code, msg: .error.message}'   # CONFLICT "Mở học liệu trước khi tích hoàn thành."
curl -s -b s.txt localhost:8080/api/v1/me/classes/$CL/lessons/<id "Giới thiệu SQL"> | jq '{type: .content.type, mediaId: .content.mediaId, exp: .content.expiresAt, opened: .progress.firstOpenedAt, dur: .lesson.durationSeconds}'   # video → url+expiresAt, 1104
curl -s -b s.txt localhost:8080/api/v1/me/classes/$CL/lessons/$LID | jq '{type: .content.type, opened: .progress.firstOpenedAt}'   # markdown, firstOpenedAt ≈ now
curl -s -b s.txt $H -X PUT -d '{"completed":true}' localhost:8080/api/v1/me/classes/$CL/lessons/$LID/completion | jq '{completedAt, percent, requiredDone, requiredTotal}'
curl -s -b s.txt $H -X PUT -d '{"completed":true}' localhost:8080/api/v1/me/classes/$CL/lessons/$LID/completion | jq .completedAt   # giống lần trước
curl -s -b s.txt $H -X PUT -d '{"completed":false}' localhost:8080/api/v1/me/classes/$CL/lessons/$LID/completion | jq .completedAt   # null
curl -s -b s.txt localhost:8080/api/v1/me/classes/$CL/lessons/<id học liệu chặng không thuộc BASIC v1> -o /dev/null -w '%{http_code}\n'   # 404
curl -s -b thao.txt localhost:8080/api/v1/me/classes/$CL | jq .error.code   # NOT_FOUND (Thảo đã rời basic01)
curl -s -b s.txt localhost:8080/api/v1/me/classes/<id basic03> | jq '{readOnly, readOnlyReason, selfReported}'   # true, "draft", true
curl -s -b s.txt $H -X PUT -d '{"completed":true}' localhost:8080/api/v1/me/classes/<id basic03>/lessons/<lid>/completion | jq .error.code   # INVALID_TRANSITION
psql "$DATABASE_URL" -c "select last_active_at from users where email_normalized='an.nguyen@gmail.com'"   # ≈ now
```

## Security notes

- Mọi kiểm tra quyền trong service qua `MemberContext`; FE không được tin.
- Không phải thành viên, dropped, học liệu ngoài course version → cùng 404 `NOT_FOUND` để không lộ lớp/học liệu; FE hiển thị message spec.
- URL video ký TTL `MEDIA_URL_TTL`; response `no-store`; không log URL.
- `lesson_id` kiểm thuộc `course_version` của lớp (không phải chỉ tồn tại) để tránh ghi tiến độ chéo lớp.

## Risks & Rollback

| Rủi ro | Biện pháp |
|---|---|
| % tính khi đọc chậm với lớp lớn | CTE một truy vấn/lớp; index `lesson_progress(class_member_id)`, `lessons(stage_version_id, required)`; MVP ≤ vài trăm học viên/lớp |
| Lớp đổi version không xảy ra sau active, nhưng clone chặng tạo lesson id mới | Tiến độ gắn `lesson_id`; lớp trỏ version cố định nên không mất; `lesson_key` dành cho so sánh báo cáo tương lai |
| URL ký hết hạn giữa video dài | TTL `MEDIA_URL_TTL=2h`; FE ký lại qua `GET /media/{mediaId}/url` khi `<video>` lỗi, tối đa 2 lần (Phase 13) |
| Học viên tích hàng loạt (self-report) | Theo spec, hiển thị "Tiến độ do học viên tự xác nhận." |

Rollback: không mount `learning`; `lesson_progress` giữ nguyên.

## Success Criteria

- [x] Unit + integration `learning` xanh; `BuildRoadmap` và SQL % cho cùng kết quả trên fixture.
- [x] `first_opened_at` ghi đúng một lần; lớp ended không ghi.
- [x] Tích trước mở → 409 `CONFLICT`; tích/bỏ tích idempotent; `completed_at` đúng.
- [x] Học liệu ngoài course version, dropped, không member → 404 `NOT_FOUND`; lớp draft → 200 `readOnly=true, readOnlyReason='draft'`; lớp ended `readOnlyReason='ended'`; PUT trên draft/ended → 409 `INVALID_TRANSITION`.
- [x] % theo required: optional không ảnh hưởng; 0 required → 0%; tổng = required done/required total toàn khóa; `domain.Percent` và SQL `round()` trùng nhau.
- [x] `/me/classes` gồm lớp draft, không gồm dropped; `nextLesson {lessonId,title,stageName}` đúng học liệu đầu chưa hoàn thành.
- [x] `users.last_active_at` cập nhật khi học viên gọi API học tập.
- [x] `selfReported: true` trong roadmap; `Cache-Control: no-store`.
- [x] DTO 4 route khớp từng trường bảng HTTP API (plan.md §7); golden JSON `testdata/*.json` được commit cho Phase 13.
- [x] `learning.ProgressReader` được nối vào `classes` (`avgPercent`, `notLoggedIn`, `inactiveOver7Days`) và sẵn cho Phase 9. (`avgPercent` qua `ClassAveragePercent`; `notLoggedIn`/`inactiveOver7Days` giữ SQL của classes cùng quy tắc, `ClassActivityCounts` sẵn cho Phase 9.)
