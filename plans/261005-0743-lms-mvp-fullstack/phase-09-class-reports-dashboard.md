---
phase: 09
title: "Phase 9: Báo cáo lớp và dashboard"
status: pending
priority: P1
effort: "2.5 ngày"
dependencies: [8]
---

# Phase 9: Báo cáo lớp và dashboard

## Goal

Hoàn thành feature `reports`: báo cáo tiến độ lớp cho Admin/Giảng viên (một dòng mỗi học viên, % từng chặng, % tổng, hoạt động gần nhất, trạng thái lời mời) với bộ lọc và sắp xếp FR-41, drilldown theo học liệu, cờ "tự xác nhận" FR-42, và dashboard Admin (KPI, khóa học dùng chặng cũ, lời mời thất bại, audit gần nhất). Giảng viên chỉ thấy lớp mình phụ trách.

## Context & Requirements

FR phủ: FR-40, FR-41, FR-42; dashboard là màn hình trong prototype (`#/dashboard`) tổng hợp dữ liệu các feature khác. NFR §8: phân quyền giáo viên ở server, audit đọc được.

Quy tắc spec:

- FR-40: mỗi dòng gồm họ tên, email, trạng thái lời mời/tài khoản, % từng chặng, % tổng, hoạt động gần nhất (`last_active_at` hoặc max(`first_opened_at`, `completed_at`)), đăng nhập gần nhất. Drilldown: từng học liệu `not_opened|opened|completed` với thời điểm. Giảng viên chỉ lớp của mình.
<!-- Red Team: RT-14 - URL param prototype (below/notlogged/inactive) ↔ API param; STALE_DAYS=7 -->
- FR-41: lọc `notLoggedIn` (chưa đăng nhập lần nào: `last_login_at IS NULL`), `inactiveDays=N` (không hoạt động > N ngày: `last_activity_at IS NULL OR last_activity_at < now - N days`; mặc định gợi ý `STALE_DAYS=7`), `belowPercent=N` (% tổng < N). Sắp xếp: `name` (mặc định), `pct` (tăng), `pct-desc`, `activity` (gần nhất trước), `activity-asc`. Allowlist, giá trị lạ → 422. Phase 12 giữ URL search params theo prototype `below`, `notlogged`, `inactive`, `sort` và ánh xạ sang `belowPercent`, `notLoggedIn`, `inactiveDays`, `sort` khi gọi API.
- FR-42: response có `selfReported: true` và FE hiển thị "Tiến độ do học viên tự xác nhận.".
<!-- Updated: Validation Session 1 - V5 includeDropped mặc định false, checkbox "Hiện học viên đã rời lớp" -->
- Thành viên `dropped` giữ lịch sử (FR-23) nhưng **mặc định ẩn** (prototype `activeMembersOf`): query `includeDropped` mặc định `false`; thanh lọc có checkbox "Hiện học viên đã rời lớp" → `includeDropped=true`, khi đó dòng dropped mang badge "Đã rời lớp". `summary` (và KPI dashboard) chỉ đếm thành viên `active` trừ khi `includeDropped=true`.

Copy tiếng Việt từ `prototype/app.js`:

| Tình huống | Message |
|---|---|
| Teacher xem lớp khác | "Bạn chỉ xem được lớp mình phụ trách." |
| Ghi chú tự xác nhận | "Tiến độ do học viên tự xác nhận." |
| Nhãn lọc (app.js 715–717) | "Chưa đăng nhập" / "Không hoạt động quá (ngày)" placeholder `7` / "% toàn khóa dưới" placeholder `50` / "Hiện học viên đã rời lớp" |
| Nhãn sort (app.js 718) | "Tên A → Z" / "% toàn khóa thấp → cao" / "% toàn khóa cao → thấp" / "Hoạt động mới nhất" / "Lâu không hoạt động" |
| Dashboard KPI (app.js 566–569) | "Lớp đang chạy" hint "{n} lớp nháp"; "Học viên đang học" hint "{n} chưa đăng nhập lần đầu"; "Lời mời thất bại" hint "Cần gửi lại"; "Khóa học dùng chặng cũ" hint "Có thể áp dụng một thao tác" / "Mọi khóa học đã cập nhật" |
| Dashboard section (app.js 572–575) | "Lớp học" (link "Tất cả lớp"), "Nhật ký thao tác" |
| Sort/filter sai | "Tham số lọc không hợp lệ." (mới) |

Trạng thái hiển thị dòng báo cáo: tài khoản `invited` "Chưa đăng nhập", `disabled` "Vô hiệu hóa"; thành viên `dropped` "Đã rời lớp" (chỉ khi `includeDropped=true`); lời mời `queued|sent|failed` theo `STATUS_VI`.

## Domain model

Feature `reports` là read-model thuần (CQRS nhẹ): không aggregate ghi, không audit ghi. Thành phần:

```go
<!-- Updated: Validation Session 1 - V7 bỏ Specification[T] generic, dùng struct Filter cụ thể + Apply(qb) -->
// Bộ lọc là một struct cụ thể; không có interface generic. Filter.Match dùng cho test thuần, Filter.Apply dịch sang SQL có tham số.
type Filter struct {
    NotLoggedIn    bool
    InactiveDays   *int     // > 0
    BelowPercent   *int     // 0..100
    IncludeDropped bool     // default false (V5)
    Sort           Sort
}
type Sort string // name | pct | pct-desc | activity | activity-asc
func ParseSort(s string) (Sort, error)              // "" → name; lạ → ErrInvalidFilter
func (f Filter) Validate() error                     // inactiveDays <= 0, belowPercent ngoài 0..100 → ErrInvalidFilter
func (f Filter) Match(r ReportRow, now time.Time) bool // thuần: dùng trong unit test và kiểm chéo với SQL
func (f Filter) Apply(qb *db.QueryBuilder, now time.Time) // thêm WHERE ... $n và ORDER BY từ allowlist map[Sort]string; không nối chuỗi từ input

type ReportRow struct {
    MemberID, UserID ids.ID; Name string; Email domain.Email
    AccountStatus string; MemberStatus string; MustChangePassword bool
    Invite *InviteStatus                          // kind, status, attempts, lastError
    StagePercents []StagePercent                  // theo thứ tự chặng: {StageID, Code, Name, Percent, RequiredDone, RequiredTotal}
    Percent domain.Percent; RequiredDone, RequiredTotal int
    LastLoginAt, LastActivityAt *time.Time
}
type ClassReport struct { Class ClassHeader; Stages []StageHeader; Rows []ReportRow; Summary Summary /* memberCount, activeCount, avgPercent, notLoggedIn, inactive, below */; Filter Filter; SelfReported bool }
type MemberReport struct { Class ClassHeader; Member ReportRow; Stages []learning.StageView /* drilldown từng học liệu */; SelfReported bool }

// Red Team: RT-07 - hình dạng dashboard theo plan.md §7 và prototype #/dashboard
type Dashboard struct {
    KPIs           KPIs           // Stages, Courses, Classes (đang chạy), Students (đang học: cm.status active, c.status active)
    Hints          Hints          // DraftClasses, NotLoggedIn, OutdatedCourses, FailedInvites
    Outdated       []OutdatedRow  // RT-07: khối cảnh báo FR-18 của prototype: courseId, courseCode, courseName, stageId, stageCode, stageName, currentVersionNo, latestPublishedNo (từ stages.OutdatedReader.AllOutdated)
    Classes        []ClassRow     // id, code, name, status, courseName, courseVersionNo, memberCount, avgPercent (ProgressReader.ClassAveragePercent)
    RecentActivity []ActivityRow  // 8 mới nhất: id, at, actorName, actionLabel (VI), target{type,id,label}, summary
}
```

Lỗi → apperr: `ErrInvalidFilter` → `422 VALIDATION_FAILED` "Tham số lọc không hợp lệ."; teacher sai lớp → `403 FORBIDDEN` "Bạn chỉ xem được lớp mình phụ trách."; lớp không tồn tại → 404; `mid` không thuộc lớp → 404.

<!-- Red Team: RT-07 - bảng nhãn audit sao chép nguyên văn từ prototype/app.js log('…') -->
Nhãn hành động audit (VI) cho `recentActivity.actionLabel` (map ở backend để FE không dịch), nguyên văn chuỗi `log('…')` trong `prototype/app.js` và `seed.js`:

| Action | Nhãn (prototype) |
|---|---|
| `stage.created` | "Tạo chặng" |
| `stage_version.cloned` | "Nhân bản chặng" |
| `stage_version.published` | "Phát hành chặng" |
| `stage_version.archived` | "Lưu trữ chặng" |
| `stage_version.deleted` | "Xóa phiên bản chặng" |
| `course.created` | "Tạo khóa học" |
| `course_version.cloned` | "Nhân bản khóa học" |
| `course_version.published` | "Phát hành khóa học" |
| `course_version.archived` | "Lưu trữ khóa học" |
| `course_version.deleted` | "Xóa phiên bản khóa học" |
| `course.stage_version_applied` | "Áp dụng chặng cho khóa học" |
| `class.created` | "Tạo lớp" |
| `class.course_version_changed` | "Đổi phiên bản khóa học của lớp" |
| `class.activated` | "Kích hoạt lớp" |
| `class.ended` | "Kết thúc lớp" |
| `class.member_invited` | "Mời học viên" |
| `class.invitation_resent` | "Gửi lại lời mời" |
| `class.member_dropped` | "Gỡ học viên khỏi lớp" |
| `user.disabled` | "Vô hiệu hóa tài khoản" |
| `user.enabled` | "Kích hoạt lại tài khoản" |

Action chưa có nhãn → fallback chính action string và log cảnh báo (test `model_test.go` bắt buộc map đủ 20 action trên).

Mẫu: Filter struct cụ thể (`Match` thuần + `Apply` SQL), Read model / CQRS, Composite query cho dashboard (từng truy vấn nhỏ chạy tuần tự trong một tx read-only `REPEATABLE READ`), Decorator middleware `ClassScope` (teacher own). SOLID: `reports` phụ thuộc `learning.ProgressReader`, `stages.OutdatedReader`, `audit.Reader` (interface khai báo ở nơi cung cấp); thêm filter mới = thêm trường vào `Filter` + nhánh trong `Match`/`Apply` (KISS, không trừu tượng hóa sớm).

## Repository interfaces

```go
package reports
type ReportRepo interface {
    ClassHeader(ctx context.Context, ex db.Executor, classID ids.ID) (ClassHeader, error)               // code, name, status, teacherId, courseName, versionNo
    StageHeaders(ctx context.Context, ex db.Executor, courseVersionID ids.ID) ([]StageHeader, error)
    Rows(ctx context.Context, ex db.Executor, classID ids.ID, f Filter, now time.Time) ([]ReportRow, error)
    // Rows: CTE percent (Phase 8) + JOIN users + LATERAL latest invitation/outbox; WHERE/ORDER BY từ f.Apply(qb, now) (tham số hóa + allowlist)
    MemberRow(ctx context.Context, ex db.Executor, classID, memberID ids.ID) (*ReportRow, error) // RT-15: WHERE cm.id=$2 AND cm.class_id=$1; nil → 404
}
type DashboardRepo interface {
    KPIs(ctx context.Context, ex db.Executor) (KPIs, error)                     // stages, courses, classes active, students active
    Hints(ctx context.Context, ex db.Executor) (Hints, error)                   // draftClasses, notLoggedIn, failedInvites (outdatedCourses từ AllOutdated)
    Classes(ctx context.Context, ex db.Executor) ([]ClassRow, error)            // mọi lớp + JOIN courses/course_versions (courseName, courseVersionNo) + memberCount (cm.status active); avgPercent gộp từ ProgressReader
}
// Interface sang feature khác
// learning.ProgressReader.MemberLessonProgress(ctx, ex, classID, memberID) (learning.Roadmap, error)   // drilldown
// learning.ProgressReader.ClassAveragePercent(ctx, ex, classIDs) (map[ids.ID]int, error)
// stages.OutdatedReader.AllOutdated(ctx) ([]stages.OutdatedCourse, error)   // RT-07: Phase 05 cung cấp cả OutdatedCourses(stageID) và AllOutdated(); phase này chỉ tiêu thụ
// audit.Reader.Latest(ctx, ex, limit int) ([]audit.Entry, error)            // platform/audit
// classes.ClassScope: TeacherOwns(ctx, ex, classID, userID) (bool, error)   // hoặc lấy từ ClassHeader.TeacherID
```

SQL `Rows` (phác thảo; CTE `pct` từ Phase 8 thêm nhóm theo stage):

```sql
WITH pct AS (/* Phase 8 CTE: class_member_id, required_done, required_total, percent, last_activity_at */),
stage_pct AS (
  SELECT cm.id class_member_id, cvs.stage_id, cvs.position,
         count(*) FILTER (WHERE l.required) required_total,
         count(*) FILTER (WHERE l.required AND lp.completed_at IS NOT NULL) required_done
  FROM class_members cm JOIN classes c ON c.id=cm.class_id
  JOIN course_version_stages cvs ON cvs.course_version_id=c.course_version_id
  JOIN lessons l ON l.stage_version_id=cvs.stage_version_id
  LEFT JOIN lesson_progress lp ON lp.class_member_id=cm.id AND lp.lesson_id=l.id
  WHERE cm.class_id=$1 GROUP BY cm.id, cvs.stage_id, cvs.position
)
SELECT cm.id member_id, u.id user_id, u.name, u.email, u.status account_status, u.must_change_password, cm.status member_status,
       u.last_login_at, coalesce(pct.last_activity_at, u.last_active_at) last_activity_at,
       pct.percent, pct.required_done, pct.required_total,
       (SELECT jsonb_agg(jsonb_build_object('stageId', sp.stage_id, 'position', sp.position, 'requiredDone', sp.required_done, 'requiredTotal', sp.required_total,
               'percent', CASE WHEN sp.required_total=0 THEN 0 ELSE round(100.0*sp.required_done/sp.required_total) END) ORDER BY sp.position)
        FROM stage_pct sp WHERE sp.class_member_id=cm.id) stage_percents,
       inv.kind invite_kind, CASE eo.status WHEN 'sent' THEN 'sent' WHEN 'failed' THEN 'failed' ELSE 'queued' END invite_status, eo.attempts, eo.last_error
FROM class_members cm JOIN users u ON u.id=cm.user_id JOIN pct ON pct.class_member_id=cm.id
LEFT JOIN LATERAL (SELECT * FROM invitations i WHERE i.class_id=cm.class_id AND i.user_id=cm.user_id ORDER BY created_at DESC LIMIT 1) inv ON true
LEFT JOIN email_outbox eo ON eo.id=inv.email_outbox_id
WHERE cm.class_id=$1
  AND ($2::bool IS FALSE OR u.last_login_at IS NULL)                                                    -- notLoggedIn
  AND ($3::int IS NULL OR coalesce(pct.last_activity_at, u.last_active_at) IS NULL OR coalesce(pct.last_activity_at, u.last_active_at) < $4 - make_interval(days => $3))  -- inactiveDays
  AND ($5::int IS NULL OR pct.percent < $5)                                                             -- belowPercent
  AND ($6::bool OR cm.status='active')                                                                  -- includeDropped (mặc định false → chỉ active)
ORDER BY <allowlist: name → u.name; pct → pct.percent ASC, u.name; pct-desc → pct.percent DESC, u.name; activity → last_activity_at DESC NULLS LAST, u.name; activity-asc → last_activity_at ASC NULLS FIRST, u.name>;
```

<!-- Red Team: RT-07 - KPI/hint theo tile prototype -->
Dashboard: `kpis.stages = count(stages)`; `kpis.courses = count(courses)`; `kpis.classes = count(classes status active)` (tile "Lớp đang chạy"); `kpis.students = count(DISTINCT cm.user_id) WHERE cm.status active AND c.status active` (tile "Học viên đang học"); `hints.draftClasses = count(classes status draft)`; `hints.notLoggedIn = count(DISTINCT users) học viên active chưa `last_login_at`; `hints.failedInvites = count(invitations i JOIN email_outbox eo status failed)` chỉ lấy lời mời mới nhất mỗi member chưa được resend thành công (tile "Lời mời thất bại"); `hints.outdatedCourses = count(DISTINCT courseId)` từ `AllOutdated` (tile "Khóa học dùng chặng cũ"). `classes[]` liệt kê mọi lớp (section "Lớp học") với `courseName`, `courseVersionNo`, `memberCount` (cm.status active) và `avgPercent` từ `ProgressReader.ClassAveragePercent`. `outdated[]` là từng cặp (khóa học, chặng) đang dùng phiên bản chặng cũ, map 1:1 từ `AllOutdated` (`currentVersionNo` = bản đang dùng, `latestPublishedNo` = bản published mới nhất) cho khối cảnh báo FR-18 trên dashboard; `hints.outdatedCourses` = số `courseId` distinct trong `outdated[]`.

## Use cases / Service methods

`reports.Service` nhận `db.Tx` (tx read-only), `ReportRepo`, `DashboardRepo`, `learning.ProgressReader`, `stages.OutdatedReader`, `audit.Reader`, `identity.UserReader` (tên actor), `clock.Clock`.

1. `ClassReport(ctx, actor, classID, f Filter) (ClassReport, error)`
   1. `f.Validate()` → 422 "Tham số lọc không hợp lệ.".
   2. `ClassHeader` → 404; `actor.Role == teacher && header.TeacherID != actor.ID` → 403 "Bạn chỉ xem được lớp mình phụ trách.". Admin: mọi lớp. Student: 403 (route bị `RequireRole(admin, teacher)` chặn trước).
   3. `StageHeaders`, `Rows(f, now)`; `Summary` tính trên toàn lớp bằng truy vấn aggregate riêng `Summary(classID, includeDropped)` (tránh double scan), `Rows` là danh sách đã lọc. `Summary` chỉ đếm thành viên `active` trừ khi `includeDropped=true` (V5).
   4. `SelfReported = true`.
<!-- Red Team: RT-15 - mid phải thuộc lớp trong URL; mọi truy vấn WHERE cm.id=$2 AND cm.class_id=$1 -->
2. `MemberReport(ctx, actor, classID, memberID) (MemberReport, error)`: scope như 1; `MemberRow(classID, memberID)` (`WHERE cm.id=$2 AND cm.class_id=$1`) nil → 404 (không lộ `mid` của lớp khác); `ProgressReader.MemberLessonProgress(classID, memberID)` cùng điều kiện → drilldown theo học liệu với `state`, `firstOpenedAt`, `completedAt`.
<!-- Red Team: RT-07 - dashboard theo hình dạng plan.md §7; ví dụ summary dùng dữ liệu seed.js -->
3. `Dashboard(ctx, actor) (Dashboard, error)` (admin): một tx read-only: `KPIs`, `Hints` + `AllOutdated` (→ `outdated[]`, `hints.outdatedCourses` đếm distinct course), `Classes` (kèm `courseName`, `courseVersionNo`) + `ClassAveragePercent`, `audit.Latest(8)` → `recentActivity[]` với `actionLabel` (bảng trên), `actorName` (batch `SnapshotByIDs`), `target {type, id, label}` (ví dụ `{type:'class', id, label:'basic01'}`), `summary` dựng từ `after`/`before` jsonb (ví dụ "Lập trình cơ bản v1 → v2", "basic01: mời an.nguyen@gmail.com", "Database v2") qua `summarize(action, before, after)` với bảng test.
4. `Export` CSV: **không** trong scope (spec không yêu cầu); ghi non-goal.

Middleware `ClassScope` dùng chung với Phase 7 (`classes.Service.GetClass` đã kiểm); `reports` tái dùng `ClassHeader.TeacherID` thay vì gọi `classes` để tránh phụ thuộc vòng.

## HTTP API

| Endpoint | Vai trò | Request | Response | Lỗi |
|---|---|---|---|---|
| `GET /classes/{id}/report?notLoggedIn=true&inactiveDays=7&belowPercent=30&includeDropped=false&sort=pct-desc` | admin, teacher (own) | query (`includeDropped` mặc định `false`) | `200 {class:{id,code,name,status,courseName,courseVersionNo,teacher:{id,name}}, stages:[{stageId,code,name,versionNo,position,requiredTotal}], rows:[{memberId,userId,name,email,accountStatus,memberStatus,mustChangePassword,invite:{kind,status,attempts,lastError}?,stagePercents:[{stageId,percent,requiredDone,requiredTotal}],percent,requiredDone,requiredTotal,lastLoginAt,lastActivityAt}], summary:{memberCount,activeCount,avgPercent,notLoggedInCount,inactiveCount,belowCount}, filter:{...echo}, selfReported:true}` | 403; 404; 422 |
| `GET /classes/{id}/report/members/{mid}` | admin, teacher (own) | – (`mid` = `class_members.id`, phải thuộc `{id}`) | `200 {class, member: ReportRow, stages:[{stageId,code,name,versionNo,percent,lessons:[{id,title,type,required,position,state,firstOpenedAt,completedAt}]}], selfReported:true}` | 403; 404 (lớp hoặc `mid` không thuộc lớp) |
| `GET /dashboard` | admin | – | `200 {kpis:{stages,courses,classes,students}, hints:{draftClasses,notLoggedIn,outdatedCourses,failedInvites}, outdated:[{courseId,courseCode,courseName,stageId,stageCode,stageName,currentVersionNo,latestPublishedNo}], classes:[{id,code,name,status,courseName,courseVersionNo,memberCount,avgPercent}], recentActivity:[{id,at,actorName,actionLabel,target:{type,id,label},summary}]}` <!-- Red Team: RT-07 - thêm outdated[] và courseName/courseVersionNo cho classes[] (Phase 11 tiêu thụ) --> | 403 |

`Cache-Control: private, no-store`. Giá trị `inactiveDays`/`belowPercent` không phải số → 422; `sort` lạ → 422. Không có phân trang (`httpx.Paginate` đã bỏ, V7): báo cáo trả toàn bộ dòng của lớp (MVP ≤ vài trăm học viên).

## Files to Create / Modify

```text
apps/api/internal/features/reports/
  filter.go            # Filter, Sort, ParseSort, Validate, Match, Apply, orderBy allowlist (V7: không generic)
  filter_test.go       # Match bảng; ParseSort; Validate biên (0, 101, -1)
  model.go             # ReportRow, ClassReport, MemberReport, Dashboard/KPIs/Hints/OutdatedRow/ClassRow/ActivityRow, actionLabel map, summarize()
  testdata/dashboard.json  # golden JSON GET /dashboard trên fixture seed.js (Phase 11 MSW import); cập nhật bằng cờ -update
  model_test.go        # actionLabel đủ 20 action theo bảng prototype; summarize mẫu
  repository.go repository_pg.go repository_pg_test.go   # //go:build integration: Rows với fixture testdb theo seed.js; filter SQL == Match trên cùng rows; sort ổn định; MemberRow scope
  service.go service_test.go handler.go dto.go
apps/api/internal/platform/audit/reader.go               # Latest(limit)
apps/api/internal/features/identity/reader.go            # SnapshotByIDs (batch)
apps/api/internal/app/{router.go, deps.go}
```

## Tasks & Steps

1. `filter.go` + unit test: `Filter.Match` trên `ReportRow` thuần (inactive với `LastActivityAt=nil` → thỏa; đúng N ngày → không thỏa, N+1 → thỏa; `IncludeDropped=false` loại `MemberStatus=dropped`); `ParseSort` allowlist; `Validate` biên.
2. `model.go`: `actionLabel` map (20 action, nguyên văn prototype) + `summarize` + test; `Summary` struct.
3. `repository_pg.go`: `Rows` SQL với tham số lọc nullable qua `Filter.Apply` (không nối chuỗi), `ORDER BY` qua map allowlist → chuỗi hằng; `MemberRow(classID, memberID)`; `Summary(classID, includeDropped)` aggregate; `KPIs`; `Hints`; `Classes`.
4. `audit.Reader.Latest`; `identity.SnapshotByIDs`; tiêu thụ `stages.OutdatedReader.AllOutdated` (Phase 05 đã cung cấp, RT-07).
5. `service.go` 1–3; handler parse query (`strconv`, lỗi → 422; `includeDropped` mặc định false); DTO.
<!-- Red Team: RT-03 / RT-14 - fixture theo seed.js basic01; ngưỡng 7 ngày; RT-15 test scope mid -->
6. Integration test với fixture `testdb` theo seed.js (lớp `basic01`, giảng viên Hương `huong.le`; thành viên: An ~79%, Bích, Cường, Dũng chưa đăng nhập, Hà, Khang không hoạt động 21 ngày, Phong, Thảo `dropped`): (a) không lọc → 7 dòng active sort tên, không có Thảo; `includeDropped=true` → 8 dòng, Thảo có `memberStatus=dropped`; (b) `notLoggedIn=true` → chỉ Dũng; (c) `inactiveDays=7` → Dũng (null) + Khang; (d) `belowPercent=50` → mọi người < 50 theo seed; (e) `sort=pct-desc` → An đầu; `sort=activity` → gần nhất đầu, null cuối; (f) kết hợp 3 filter → giao; (g) `sort=evil` → 422; `inactiveDays=0` → 422; (h) teacher Hương xem lớp của Bảo (`basic02`) → 403; admin → 200; student → 403; (i) drilldown trả đủ học liệu với state; `mid` của học viên lớp `basic02` gọi trên `basic01` (teacher Hương) → 404; `mid` không tồn tại → 404; (j) dashboard: `kpis`/`hints` khớp đếm SQL thủ công; `hints.outdatedCourses` khớp fixture Phase 5/6 (Database v2 phát hành, Lập trình cơ bản v1 dùng v1); `hints.failedInvites` có bản seed `attempts=3` "Mailbox không tồn tại (550 5.1.1)"; `outdated[]` có đúng một dòng `{courseCode:"BASIC", courseName:"Lập trình cơ bản", stageCode:"DB", stageName:"Database", currentVersionNo:1, latestPublishedNo:2}` và rỗng sau khi áp dụng FR-17; `classes[]` có `basic01` với `courseName="Lập trình cơ bản"`, `courseVersionNo=1`, `memberCount=7`; `recentActivity` 8 bản mới nhất có `actionLabel` thuộc bảng; response khớp golden `testdata/dashboard.json`.
7. Lint, test, docs API.

## Verification

```bash
cd apps/api && go test ./internal/features/reports/...
cd apps/api && go test -tags integration ./internal/features/reports/...
make dev && make seed   # cookie admin c.txt, teacher t.txt (huong.le), teacher2 t2.txt (bao.pham), student s.txt
CL=<id lớp basic01>
curl -s -b c.txt "localhost:8080/api/v1/classes/$CL/report" | jq '{n: (.rows|length), self: .selfReported, avg: .summary.avgPercent, first: .rows[0].name}'   # 7 dòng (không có Thảo)
curl -s -b c.txt "localhost:8080/api/v1/classes/$CL/report?includeDropped=true" | jq '[.rows[] | select(.memberStatus=="dropped") | .name]'   # ["Thảo …"]
curl -s -b c.txt "localhost:8080/api/v1/classes/$CL/report?notLoggedIn=true" | jq '[.rows[] | {name, lastLoginAt}]'           # chỉ Dũng (lastLoginAt null)
curl -s -b c.txt "localhost:8080/api/v1/classes/$CL/report?inactiveDays=7" | jq '[.rows[] | {name, lastActivityAt}]'          # Dũng + Khang
curl -s -b c.txt "localhost:8080/api/v1/classes/$CL/report?belowPercent=30&sort=pct-desc" | jq '[.rows[] | {name, percent}]'   # tất cả < 30, giảm dần
curl -s -b c.txt "localhost:8080/api/v1/classes/$CL/report?sort=evil" | jq .error.message                # "Tham số lọc không hợp lệ."
curl -s -b c.txt "localhost:8080/api/v1/classes/$CL/report?inactiveDays=abc" -o /dev/null -w '%{http_code}\n'   # 422
curl -s -b t.txt "localhost:8080/api/v1/classes/$CL/report" -o /dev/null -w '%{http_code}\n'            # 200 (lớp của Hương)
curl -s -b t2.txt "localhost:8080/api/v1/classes/$CL/report" | jq .error.message                        # "Bạn chỉ xem được lớp mình phụ trách."
curl -s -b s.txt "localhost:8080/api/v1/classes/$CL/report" | jq .error.code                            # FORBIDDEN
curl -s -b c.txt "localhost:8080/api/v1/classes/$CL/report/members/<mid của An>" | jq '.stages[0].lessons[] | {title, state, completedAt}'
curl -s -b c.txt "localhost:8080/api/v1/classes/$CL/report/members/<mid của học viên basic02>" -o /dev/null -w '%{http_code}\n'   # 404
curl -s -b c.txt localhost:8080/api/v1/dashboard | jq '{kpis, hints, outdated: [.outdated[] | "\(.courseCode) v\(.currentVersionNo) → \(.stageCode) v\(.latestPublishedNo)"], classes: [.classes[] | {code, courseName, courseVersionNo, memberCount, avgPercent}], activity: [.recentActivity[] | .actionLabel]}'
# đối chiếu KPI với SQL
psql "$DATABASE_URL" -c "select count(*) from classes where status='active'"
psql "$DATABASE_URL" -c "select count(distinct cm.user_id) from class_members cm join classes c on c.id=cm.class_id where cm.status='active' and c.status='active'"
```

## Security notes

- Teacher scope kiểm trong service bằng `ClassHeader.TeacherID`; test 403 bắt buộc.
- `mid` luôn được ràng với `class_id` của URL ở tầng SQL (`WHERE cm.id=$2 AND cm.class_id=$1`), không khớp → 404; test cross-class bắt buộc (RT-15).
- `ORDER BY` và filter chỉ qua tham số/allowlist; không nối chuỗi từ query string.
- Response không có `last_error` chi tiết SMTP nào chứa thông tin nhạy cảm ngoài mã lỗi + mô tả ngắn (mailer đã cắt `last_error` ≤ 500 ký tự khi ghi).
- Dashboard chỉ admin; audit `before/after` không trả raw, chỉ `summary` đã dựng.

## Risks & Rollback

| Rủi ro | Biện pháp |
|---|---|
| Truy vấn báo cáo nặng với nhiều học viên × học liệu | Một truy vấn/lớp với CTE; index `lesson_progress(class_member_id)`, `invitations(class_id,user_id,created_at desc)`; cân nhắc materialized view sau MVP |
| `Summary` và `Rows` lệch do hai truy vấn | Cùng tx `REPEATABLE READ` |
| `hints.failedInvites` đếm cả lời mời đã resend thành công | Chỉ lấy invitation mới nhất mỗi (class, user); test với seed có resend |
| Teacher scope lọt do kiểm chỉ ở FE | Test integration 403 là tiêu chí bắt buộc |

Rollback: không mount `reports`; không có dữ liệu ghi.

## Success Criteria

- [x] Unit + integration `reports` xanh; `Filter.Match` và SQL (`Filter.Apply`) cho cùng tập dòng trên fixture.
- [x] Báo cáo trả đủ cột FR-40: tên, email, trạng thái tài khoản/thành viên/lời mời, % từng chặng theo thứ tự, % tổng, đăng nhập và hoạt động gần nhất.
- [x] Ba filter hoạt động riêng và kết hợp; năm sort đúng thứ tự, null xử lý nhất quán; tham số sai → 422 "Tham số lọc không hợp lệ.".
- [x] `includeDropped` mặc định `false`: dòng dropped chỉ xuất hiện (badge "Đã rời lớp") khi `includeDropped=true`; `summary` đếm theo cùng quy tắc.
- [x] `selfReported: true` ở cả báo cáo lớp và drilldown.
- [x] Teacher lớp khác → 403 "Bạn chỉ xem được lớp mình phụ trách."; student → 403; admin → 200; `mid` không thuộc lớp → 404.
- [x] Drilldown liệt kê mọi học liệu của course version với `state`, `firstOpenedAt`, `completedAt`.
- [x] Dashboard trả đúng hình dạng `{kpis, hints, outdated, classes, recentActivity}` (plan.md §7); `kpis`/`hints` khớp SQL thủ công; `outdated[]` và `hints.outdatedCourses` khớp fixture FR-18; `classes[]` có `courseName`, `courseVersionNo`; golden `testdata/dashboard.json` khớp; `recentActivity` 8 dòng với `actionLabel` nguyên văn prototype, `actorName`, `target`.
- [x] Không có chuỗi query nào được nối vào SQL (review `repository_pg.go`); không có `Specification[T]` hay `httpx.Paginate`.
