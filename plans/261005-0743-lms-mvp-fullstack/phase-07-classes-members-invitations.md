---
phase: 07
title: "Phase 7: Lớp, thành viên, lời mời"
status: pending
priority: P1
effort: "4 ngày"
dependencies: [4, 6]
---

# Phase 7: Lớp, thành viên, lời mời

## Goal

Hoàn thành feature `classes`: tạo lớp nháp gắn phiên bản khóa học đã phát hành, đổi phiên bản khi còn nháp, kích hoạt/kết thúc một chiều, mời học viên bằng email (tạo tài khoản + mật khẩu tạm hoặc chỉ thêm + thông báo), gửi lại lời mời, gỡ thành viên, danh sách thành viên kèm trạng thái gửi email và trạng thái tài khoản, và danh sách lớp của giảng viên. Toàn bộ FR-01, FR-02, FR-20…FR-23 ở backend.

## Context & Requirements

FR phủ: FR-01, FR-02, FR-20, FR-21, FR-22, FR-23, FR-50 (nội dung email). NFR §8: rate limit mời, không lộ mật khẩu tạm, audit `class.*`.

Quy tắc spec:

- Tạo lớp: mã unique, phiên bản khóa học `published`, ngày bắt đầu/kết thúc dự kiến (`end > start`, CHECK ở DB), giảng viên phụ trách (user `role=teacher`, `status=active`). Lớp bắt đầu ở `draft`.
- Đổi phiên bản khóa học: chỉ khi lớp `draft`, chỉ sang bản `published`. Audit `class.course_version_changed`.
- Trạng thái một chiều `draft → active → ended`; học/hoàn thành chỉ khi `active`; `ended` chỉ xem.
- Mời (FR-01): chuẩn hóa email; không mời vào lớp `ended`; email thuộc tài khoản `admin|teacher` → lỗi; tài khoản `disabled` → lỗi; đã là thành viên (chưa dropped) → "Học viên đã có trong lớp."; thành viên `dropped` → kích hoạt lại thành `active` (giữ tiến độ cũ) và gửi thông báo; email mới → tạo user `student` `invited` + mật khẩu tạm + `must_change_password` + hết hạn 72h + thêm vào lớp + email `invite`; user `active` → thêm + email `added`; user `invited` (chưa đổi mật khẩu) ở lớp khác: **mật khẩu tạm còn hạn → không xoay**, thêm + email `added` (nội dung "Bạn đã được thêm vào lớp …, dùng mật khẩu tạm đã gửi trước đó"); **mật khẩu tạm đã hết hạn → xoay** + email `invite`. Khi xoay, các hàng `email_outbox` `queued` cũ của cùng `to_email` với template `invite|resend` được đánh `failed`, `last_error='superseded'` trong cùng tx. Tất cả trong một transaction kèm ghi outbox. Đầu `Invite`/`Resend` khóa `pg_advisory_xact_lock(hashtext(lower(email)))` để hai request mời cùng email không tạo trùng. <!-- Red Team: RT-10 - không xoay khi temp còn hạn; supersede; advisory lock -->
- Gửi lại (FR-02): chỉ khi `must_change_password = true`; mật khẩu tạm mới làm mất hiệu lực mật khẩu cũ, reset hết hạn; ghi `invitations` kind `resend`; supersede hàng `queued` cũ như trên; tối đa 3 lần/giờ/thành viên, đếm trong `SELECT … FOR UPDATE` hàng `class_members`. <!-- Red Team: RT-10 -->
- Gỡ: `status=dropped`, `dropped_at`, tiến độ giữ nguyên; audit `class.member_dropped`.
- Danh sách thành viên (`GET /classes/{id}/members?includeDropped=` mặc định `false`): tên, email, trạng thái tài khoản, `tempPasswordExpiresAt` khi còn `must_change_password`, trạng thái gửi lời mời `inviteStatus` = `email_outbox.status` (`queued|sent|failed`) hoặc **NULL** khi thành viên không có lời mời nào (UI không hiện gì), kèm `attempts`, `lastError`, ngày tham gia, `lastLoginAt`, `lastActiveAt`. <!-- Red Team: RT-02 / RT-10 - inviteStatus nullable, includeDropped -->
- `GET /teach/classes` (phase này cung cấp, Phase 12 dùng): mỗi lớp kèm `memberCount`, `avgPercent`, `notLoggedIn` (thành viên `active` có tài khoản `invited`), `inactiveOver7Days` (thành viên `active` có `last_active_at IS NULL OR < now - STALE_DAYS`, chỉ tính khi lớp `active`, prototype dòng 736); `STALE_DAYS=7` từ `config.Config`. <!-- Red Team: RT-10 - hợp đồng /teach/classes -->

Copy tiếng Việt từ `prototype/app.js`:

| Tình huống | Message |
|---|---|
| Thiếu mã/tên lớp | "Nhập mã và tên lớp." |
| Mã lớp trùng | "Mã lớp đã tồn tại." (mới) |
| Thiếu ngày | "Nhập ngày bắt đầu và kết thúc dự kiến." |
| Ngày sai | "Ngày kết thúc phải sau ngày bắt đầu." |
| Thiếu giảng viên | "Chọn giảng viên phụ trách." |
| Giảng viên không hợp lệ (không phải teacher active) | "Giảng viên không hợp lệ hoặc đã bị vô hiệu hóa." (mới) |
| Phiên bản khóa học chưa published | "Chọn một phiên bản khóa học đã phát hành." |
| Đổi phiên bản khi không draft | "Chỉ đổi sang phiên bản đã phát hành." (prototype dùng chung) → tách: lớp không draft "Chỉ đổi phiên bản khi lớp còn nháp." (mới; Phase 14 E05 dùng đúng chuỗi này); bản không published "Chỉ đổi sang phiên bản đã phát hành." <!-- Red Team: RT-10 - giữ chuỗi tách --> |
| Chuyển trạng thái sai | "Chỉ chuyển trạng thái một chiều: nháp → đang chạy → đã kết thúc." |
| Mời vào lớp ended | "Không mời được vào lớp đã kết thúc." |
| Email sai | "Email không hợp lệ." |
| Thiếu tên (email mới) | "Nhập họ tên học viên." |
| Email nội bộ | "Email này thuộc tài khoản nội bộ, không mời làm học viên được." |
| Tài khoản disabled | "Tài khoản đã bị vô hiệu hóa. Kích hoạt lại trước khi mời." |
| Đã trong lớp | "Học viên đã có trong lớp." |
| Resend khi đã đổi mật khẩu | "Học viên đã đổi mật khẩu; không cần gửi lại lời mời." |
| Toast tạo lớp | "Đã tạo lớp ở trạng thái nháp." |
| Toast lưu | "Đã lưu cài đặt lớp." |
| Toast mời mới | `Đã tạo tài khoản cho ${name}, lời mời đang được gửi.` |
| Toast thêm user có sẵn | `${name} đã có tài khoản: đã thêm vào lớp và gửi thông báo.` |
| Toast resend | "Lời mời mới đang được gửi." |
| Toast gỡ | `Đã gỡ ${name} khỏi lớp.` |
| Teacher xem lớp khác | "Bạn chỉ xem được lớp mình phụ trách." |
| Resend quá 3 lần/giờ | "Đã gửi lại quá nhiều lần. Thử lại sau." (mới) |

Mã lỗi HTTP cho lời mời (Phase 12 ánh xạ đúng từng mã → message trên, không viết lại copy): <!-- Red Team: RT-10 - hợp đồng mã lỗi -->

| Tình huống | HTTP | `error.code` |
|---|---|---|
| Email sai / thiếu họ tên khi email mới | 422 | `VALIDATION_FAILED` |
| Lớp `ended` | 409 | `INVALID_TRANSITION` |
| Email thuộc `admin|teacher` | 403 | `FORBIDDEN` |
| Tài khoản `disabled` | 409 | `ACCOUNT_DISABLED` |
| Đã trong lớp (hoặc đua `uq_class_members_class_user`) | 409 | `CONFLICT` |
| Resend khi đã đổi mật khẩu | 409 | `CONFLICT` |
| Resend quá 3 lần/giờ | 429 | `RATE_LIMITED` |

Nhãn (`ClassStatus` chỉ có `draft|active|ended` <!-- Red Team: RT-03 -->): lớp `draft` "Nháp", `active` "Đang chạy", `ended` "Đã kết thúc"; thành viên `active`/`dropped` "Đã rời lớp" (`completed` có trong CHECK và VO nhưng MVP không có endpoint ghi, không cần nhãn <!-- Updated: Validation Session 1 - V3 -->); gửi email `queued` "Đang chờ gửi", `sent` "Đã gửi" (kind `added` + sent → "Đã gửi thông báo"), `failed` "Gửi thất bại"; tài khoản `invited` "Chưa đăng nhập", `disabled` "Vô hiệu hóa".

## Domain model

### Aggregate `Class`

```go
type Class struct {
    id              ids.ID
    code            domain.Code      // ^[a-z0-9][a-z0-9-]{1,29}$ (prototype `basic01`)  <!-- Red Team: RT-03 -->
    name            string
    courseVersionID ids.ID
    status          ClassStatus      // Draft | Active | Ended; chỉ Draft→Active, Active→Ended  <!-- Red Team: RT-03 -->
    startDate, endDate civil.Date    // lưu DATE; VO DateRange
    teacherID       ids.ID
    activatedAt, endedAt *time.Time
}
type DateRange struct{ Start, End civil.Date }; func NewDateRange(s, e civil.Date) (DateRange, error) // "Nhập ngày…" / "Ngày kết thúc phải sau ngày bắt đầu."
func NewClass(id ids.ID, code domain.Code, name string, cv PublishedCourseVersion, dates DateRange, teacher ActiveTeacher) (*Class, error)
func (c *Class) Rename(name string) error
func (c *Class) Reschedule(d DateRange) error                   // mọi trạng thái trừ ended
func (c *Class) AssignTeacher(t ActiveTeacher) error            // mọi trạng thái trừ ended
func (c *Class) ChangeCourseVersion(cv PublishedCourseVersion) error // chỉ draft
func (c *Class) Activate(now time.Time) error; func (c *Class) End(now time.Time) error // theo ClassStatus.CanTransitionTo
func (c *Class) IsActive() bool; func (c *Class) AcceptsInvitations() bool // status != ended
```

`PublishedCourseVersion{ID ids.ID; CourseName string; VersionNo}` và `ActiveTeacher{ID ids.ID; Name string}` là VO "đã kiểm" được service dựng từ `courses.VersionReader` / `identity.UserRepo`, đảm bảo entity không thể nhận dữ liệu chưa kiểm (Specification bọc bằng kiểu dữ liệu).

### Entity `ClassMember`

```go
type ClassMember struct { id, classID, userID ids.ID; status MemberStatus /* active|dropped|completed; MVP không có writer cho completed */; joinedAt time.Time; droppedAt *time.Time } // <!-- Updated: Validation Session 1 - V3 -->
func NewMember(id, classID, userID ids.ID, now time.Time) *ClassMember
func (m *ClassMember) Drop(now time.Time) error      // active → dropped
func (m *ClassMember) Rejoin(now time.Time) error    // dropped → active; joinedAt giữ nguyên (tiến độ cũ vẫn gắn member này)
```

### Entity `Invitation` (log gửi, append-only)

```go
type Invitation struct { id, classID, userID, sentBy ids.ID; kind InvitationKind /* invite|added|resend */; emailOutboxID ids.ID; createdAt time.Time }
```

### Domain service `InvitePolicy` (quy tắc FR-01 thuần, test không cần DB)

```go
type InviteDecision int // DecisionCreateUser | DecisionAddActive | DecisionAddInvitedKeep | DecisionAddInvitedRotate | DecisionRejoin | reject…
func DecideInvite(cls *Class, existing *identity.UserSnapshot, member *ClassMember, fullName string, now time.Time) (InviteDecision, error)
```

Thứ tự kiểm trong `DecideInvite` (khớp prototype dòng 288–309): (1) `!cls.AcceptsInvitations()` → `ErrClassEnded`; (2) `existing != nil && existing.Role != student` → `ErrInternalEmail`; (3) `existing.Status == disabled` → `ErrAccountDisabled`; (4) `member != nil && member.status == active` → `ErrAlreadyMember`; (5) `member != nil && dropped` → `DecisionRejoin`; (6) `existing == nil` → `fullName == ""` → `ErrNameRequired` else `DecisionCreateUser`; (7) `existing.MustChangePassword && existing.TempPasswordExpiresAt.After(now)` → `DecisionAddInvitedKeep` (thêm + email `added`, không xoay); (7b) `existing.MustChangePassword` và đã hết hạn → `DecisionAddInvitedRotate`; (8) `DecisionAddActive`. <!-- Red Team: RT-10 -->

Lỗi → apperr (khớp bảng mã lỗi ở trên): `ErrClassEnded` → `409 INVALID_TRANSITION`; `ErrInternalEmail` → `403 FORBIDDEN`; `ErrAccountDisabled` → `409 ACCOUNT_DISABLED`; `ErrAlreadyMember`, `ErrAlreadyActivated`, `ErrCodeTaken` → `409 CONFLICT`; `ErrResendLimit` → `429 RATE_LIMITED`; `ErrNameRequired`, `ErrInvalidDates`, `ErrTeacherInvalid`, `ErrVersionNotPublished` → `422 VALIDATION_FAILED`; `ErrInvalidTransition`, `ErrClassNotDraft` → `409 INVALID_TRANSITION`. <!-- Red Team: RT-10 -->

Mẫu: Repository, Unit of Work (mời = một `Transact` xuyên identity + classes + mailer), State machine (`ClassStatus`, `MemberStatus`), Specification (VO đã kiểm), Domain Service (`DecideInvite`), Outbox (qua `mailer.Enqueuer`), Facade (`identity.UserProvisioner`). SOLID: `classes` phụ thuộc interface `identity.UserProvisioner`, `identity.UserReader`, `courses.VersionReader`, `mailer.Enqueuer`, `mailer.OutboxStatusReader` (ISP, DIP) <!-- Red Team: RT-02 - tên interface -->; logic rẽ nhánh FR-01 ở một chỗ (SRP).

## Repository interfaces

```go
package classes
type ClassRepo interface {
    Create(ctx context.Context, ex db.Executor, c *Class) error             // 23505 pgerr.UqClassesCode ("uq_classes_code") → ErrCodeTaken; 23514 pgerr.CkClassesDates → ErrInvalidDates  <!-- Red Team: RT-11 -->
    Update(ctx context.Context, ex db.Executor, c *Class) error
    ByID(ctx context.Context, ex db.Executor, id ids.ID) (*Class, error)
    ByIDForUpdate(ctx context.Context, ex db.Executor, id ids.ID) (*Class, error)
    List(ctx context.Context, ex db.Executor, q ListQuery) ([]ClassListRow, error) // status, teacherId filter; kèm courseName, versionNo, teacherName, memberCount, notLoggedIn, inactiveOver(staleDays)  <!-- Red Team: RT-10 -->
}
type MemberRepo interface {
    Create(ctx context.Context, ex db.Executor, m *ClassMember) error        // 23505 pgerr.UqClassMembersClassUser → ErrAlreadyMember (race)  <!-- Red Team: RT-11 -->
    Update(ctx context.Context, ex db.Executor, m *ClassMember) error
    ByID(ctx context.Context, ex db.Executor, classID, memberID ids.ID) (*ClassMember, error)
    ByIDForUpdate(ctx context.Context, ex db.Executor, classID, memberID ids.ID) (*ClassMember, error) // Resend đếm giới hạn dưới khóa hàng  <!-- Red Team: RT-10 -->
    ByClassAndUser(ctx context.Context, ex db.Executor, classID, userID ids.ID) (*ClassMember, error) // nil,nil khi không có
    ListRows(ctx context.Context, ex db.Executor, classID ids.ID, includeDropped bool) ([]MemberRow, error) // JOIN users + latest invitation + outbox status  <!-- Red Team: RT-10 -->
}
type InvitationRepo interface {
    Create(ctx context.Context, ex db.Executor, inv *Invitation) error
    LatestByMember(ctx context.Context, ex db.Executor, classID ids.ID) (map[ids.ID]InvitationRow, error) // userID → {kind, outboxID, createdAt}
    CountResendSince(ctx context.Context, ex db.Executor, classID, userID ids.ID, since time.Time) (int, error) // kind='resend'  <!-- Red Team: RT-10 -->
}
// Interface sang feature khác (khai báo tại nơi cung cấp, Phase 4/6)
// identity.UserProvisioner: ProvisionStudent(ctx, ex, email, name, now) (*User, TemporaryPassword, created bool, error); RotateTemporaryPassword(ctx, ex, u, now) (TemporaryPassword, error)
// identity.UserReader: SnapshotByEmail(ctx, ex, email) (*UserSnapshot, error); SnapshotByID(ctx, ex, id) (*UserSnapshot, error); ActiveTeacher(ctx, ex, id) (ActiveTeacher, error)
// courses.VersionReader: PublishedVersion(ctx, ex, id) (VersionSummary, error)
// mailer.Enqueuer; mailer.OutboxStatusReader (StatusByIDs); mailer.OutboxRepo.SupersedeQueued(ctx, ex, toEmail, []string{"invite","resend"}, now)  <!-- Red Team: RT-02 / RT-10 -->
```

`MemberRow` SQL (một truy vấn):

```sql
SELECT cm.id, cm.user_id, u.name, u.email, u.status account_status, u.must_change_password, u.last_login_at, u.last_active_at,
       cm.status member_status, cm.joined_at, cm.dropped_at,
       inv.kind invite_kind, inv.created_at invited_at,
       u.temp_password_expires_at,
       CASE WHEN eo.id IS NULL THEN NULL WHEN eo.status = 'sending' THEN 'queued' ELSE eo.status END invite_status, -- NULL khi chưa có lời mời  <!-- Red Team: RT-02 -->
       eo.attempts, eo.last_error, eo.sent_at
FROM class_members cm JOIN users u ON u.id = cm.user_id
LEFT JOIN LATERAL (SELECT i.* FROM invitations i WHERE i.class_id = cm.class_id AND i.user_id = cm.user_id ORDER BY i.created_at DESC LIMIT 1) inv ON true
LEFT JOIN email_outbox eo ON eo.id = inv.email_outbox_id
WHERE cm.class_id = $1 AND ($2 OR cm.status <> 'dropped') ORDER BY u.name;  -- $2 = includeDropped  <!-- Red Team: RT-10 -->
```

## Use cases / Service methods

`classes.Service` nhận `db.Tx`, `ClassRepo`, `MemberRepo`, `InvitationRepo`, `identity.UserProvisioner`, `identity.UserReader`, `courses.VersionReader`, `mailer.Enqueuer`, `mailer.OutboxRepo` (chỉ `SupersedeQueued`), `clock.Clock`, `audit.Recorder`, `ids.Generator`, và field `PublicBaseURL`, `StaleDays` của `config.Config` Phase 1; `AppName` là hằng "GoUp LMS" trong `mailer`. <!-- Red Team: RT-11 -->

1. `CreateClass(ctx, actor, cmd{Code, Name, CourseVersionID, StartDate, EndDate, TeacherID}) (ClassDetailDTO, error)`: Transact: `VersionReader.PublishedVersion` (lỗi → "Chọn một phiên bản khóa học đã phát hành."); `UserReader.ActiveTeacher` (lỗi → "Giảng viên không hợp lệ hoặc đã bị vô hiệu hóa."); `NewDateRange`; `NewClass`; `Create`. Không audit (spec không yêu cầu).
2. `UpdateClass(ctx, actor, id, patch{Name?, StartDate?, EndDate?, TeacherID?, CourseVersionID?})`: Transact: `ByIDForUpdate`; áp từng field; `CourseVersionID` ≠ hiện tại → `ChangeCourseVersion` (lớp không draft → `ErrClassNotDraft`; bản không published → "Chỉ đổi sang phiên bản đã phát hành.") → audit `class.course_version_changed` `{from, to}`; `Update`.
3. `Activate(ctx, actor, id)` / `End(ctx, actor, id)`: Transact: `ByIDForUpdate` → `Activate/End` → `Update` → audit `class.activated` / `class.ended`.
4. `GetClass(ctx, actor, id) (ClassDetailDTO, error)`: admin mọi lớp; teacher chỉ `teacher_id == actor` (sai → `403 FORBIDDEN` "Bạn chỉ xem được lớp mình phụ trách."). Trả class + course version summary (stages/lessons count) + teacher + memberCount.
5. `ListClasses(ctx, q)` (admin); `ListTeachingClasses(ctx, actor)` (teacher: `teacher_id = actor`, kèm `memberCount`, `notLoggedIn`, `inactiveOver7Days` tính trong `ClassRepo.List` với `StaleDays`, và `avgPercent` qua interface `classes.ProgressReader{AvgPercentByClass(ctx, ex, classIDs) (map[ids.ID]int, error)}` do Phase 8 (`learning`) hiện thực; cho tới khi Phase 8 nối vào `deps.go`, `deps.go` cắm bản `zeroProgressReader` trả `null`). <!-- Red Team: RT-10 -->
6. `ListMembers(ctx, actor, classID, includeDropped bool) ([]MemberDTO, error)`: scope như 4; `ListRows(classID, includeDropped)`.
7. `Invite(ctx, actor, classID, cmd{Email, FullName}) (InviteResult{Kind InviteKind /* invited|added */, Member MemberDTO}, error)` — một `Transact`: <!-- Red Team: RT-10 - hợp đồng response {kind, member} -->
   1. `domain.NewEmail` → "Email không hợp lệ.".
   2. `pg_advisory_xact_lock(hashtext(lower(email)))` (qua `identity.UserRepo.LockEmail`); `ByIDForUpdate(class)`; `SnapshotByEmail`; `ByClassAndUser`.
   3. `DecideInvite(cls, existing, member, fullName, now)` → lỗi hoặc decision.
   4. Theo decision (mật khẩu tạm đi vào `Message.Secret`, **không** vào `Payload`):
      - `CreateUser`: `ProvisionStudent` (tạo user invited + temp pw); `NewMember` → `Create`; `Enqueue(Message{Template: invite, Payload{Name, ClassName, ClassCode, LoginURL, Email, ExpiresAt}, Secret: tmp.Reveal()})`; `Invitation{kind: invite}`; `Kind: invited`.
      - `AddInvitedKeep`: member mới; `Enqueue(Message{Template: added, Payload{Name, ClassName, ClassCode, LoginURL, UsePreviousTempPassword: true}})`; kind `added`; `Kind: added`. Không xoay, không supersede.
      - `AddInvitedRotate`: `RotateTemporaryPassword` (hash mới, expiry 72h); `SupersedeQueued(email, [invite,resend])`; member mới; `Enqueue(invite + Secret)`; kind `invite`; `Kind: invited`.
      - `AddActive`: member mới; `Enqueue(Message{Template: added, Payload{Name, ClassName, ClassCode, LoginURL}})`; kind `added`; `Kind: added`.
      - `Rejoin`: `member.Rejoin(now)` → `Update`; email `added` (hoặc như `AddInvitedKeep`/`AddInvitedRotate` nếu user vẫn `must_change_password`); kind `added`; `Kind: added`.
   5. Audit `class.member_invited` `{classId, userId, kind}` — không chứa mật khẩu.
   6. Trả `{kind, member}`; `member.inviteStatus = queued`. Toast FE theo `kind` (copy prototype) do Phase 12 chọn; response không mang message toast.
8. `Resend(ctx, actor, classID, memberID)`: Transact: `MemberRepo.ByIDForUpdate` (member `active`) → `CountResendSince(now-1h) >= 3` → `ErrResendLimit` (429) → `SnapshotByID` → `!MustChangePassword` → `ErrAlreadyActivated` (409); `LockEmail`; `RotateTemporaryPassword`; `SupersedeQueued(email, [invite,resend])`; `Enqueue(Message{Template: resend, Payload{…}, Secret})`; `Invitation{kind: resend}`; audit `class.invitation_resent`. Trả `200 {member}`. <!-- Red Team: RT-10 - đếm dưới FOR UPDATE, 200 -->
9. `RemoveMember(ctx, actor, classID, memberID)`: Transact: `member.Drop(now)` → `Update` → audit `class.member_dropped`. Không xóa `lesson_progress`. Toast FE `Đã gỡ ${name} khỏi lớp.`.
10. `MemberContext(ctx, ex, classID, userID) (MemberContext, error)` — interface `classes.MembershipReader` cho Phase 8: trả `{MemberID, MemberStatus, ClassStatus, CourseVersionID, TeacherID}` một truy vấn JOIN.

## HTTP API

| Endpoint | Vai trò | Request | Response | Lỗi |
|---|---|---|---|---|
| `GET /classes?status=&teacherId=&q=` | admin | – | `200 {items:[{id,code,name,status,courseName,courseVersionNo,teacherName,startDate,endDate,memberCount}]}` | – |
| `POST /classes` | admin | `{code,name,courseVersionId,startDate,endDate,teacherId}` | `201 ClassDetailDTO` | 422 (5 message); 409 `CONFLICT` "Mã lớp đã tồn tại." |
| `GET /classes/{id}` | admin, teacher (own) | – | `200 {id,code,name,status,startDate,endDate,teacher:{id,name},courseVersion:{id,courseId,courseName,versionNo,stageCount,lessonCount},memberCount,activatedAt,endedAt}` | 403; 404 "Không tìm thấy lớp." |
| `PATCH /classes/{id}` | admin | partial `{name?,startDate?,endDate?,teacherId?,courseVersionId?}` | `200 ClassDetailDTO` | 422; 409 `INVALID_TRANSITION` "Chỉ đổi phiên bản khi lớp còn nháp."; 422 "Chỉ đổi sang phiên bản đã phát hành." |
| `POST /classes/{id}/activate` · `/end` | admin | – | `200 ClassDetailDTO` | 409 `INVALID_TRANSITION` "Chỉ chuyển trạng thái một chiều: nháp → đang chạy → đã kết thúc." |
| `GET /classes/{id}/members?includeDropped=` | admin, teacher (own) | `includeDropped` mặc định `false` | `200 {items:[MemberDTO]}` | 403; 404 |
| `POST /classes/{id}/invitations` | admin; rate limit 30/phút/IP | `{email, fullName}` (`fullName` bắt buộc khi email chưa có tài khoản) | `201 {kind:'invited'\|'added', member: MemberDTO}` | 422 `VALIDATION_FAILED`; 409 `INVALID_TRANSITION` (ended); 403 `FORBIDDEN` (nội bộ); 409 `ACCOUNT_DISABLED`; 409 `CONFLICT` (đã trong lớp) — message theo bảng copy |
| `POST /classes/{id}/members/{mid}/resend` | admin | – | `200 {member: MemberDTO}` | 409 `CONFLICT` "Học viên đã đổi mật khẩu; không cần gửi lại lời mời."; 429 `RATE_LIMITED` sau 3 lần/giờ |
| `DELETE /classes/{id}/members/{mid}` | admin | – | `200 MemberDTO` (status dropped) | 404; 409 nếu đã dropped |
| `GET /teach/classes` | teacher | – | `200 {items:[{id, code, name, status, startDate, endDate, courseName, courseVersionNo, memberCount, avgPercent, notLoggedIn, inactiveOver7Days}]}` | 403 cho admin/student (plan.md: admin không vào `/teach/*`) |

`MemberDTO{id (class_members.id), userId, email, fullName, accountStatus: 'invited'|'active'|'disabled', memberStatus: 'active'|'dropped'|'completed', tempPasswordExpiresAt?, inviteStatus?: 'queued'|'sent'|'failed' (vắng khi chưa có lời mời), inviteKind?: 'invite'|'added'|'resend', inviteAttempts?, inviteLastError?, invitedAt?, lastLoginAt?, lastActiveAt?, joinedAt, droppedAt?}`. Các field `invite*` phụ là dữ liệu prototype dòng 674 ("Gửi thất bại · {lastError} · {attempts} lần thử"); không có field nào chứa mật khẩu. <!-- Red Team: RT-10 - MemberDTO -->

## Files to Create / Modify

```text
apps/api/internal/features/classes/
  entity.go            # Class, ClassStatus, DateRange, ClassMember, MemberStatus, Invitation, VO đã kiểm, lỗi
  invite_policy.go     # DecideInvite
  entity_test.go invite_policy_test.go   # bảng 8 nhánh FR-01 + chuyển trạng thái
  repository.go repository_pg.go repository_pg_test.go   # //go:build integration: CHECK end>start, UNIQUE(class_id,user_id), MemberRow SQL
  service.go service_test.go service_integration_test.go  # mời đủ nhánh với outbox thật (LogSender), resend, remove, activate/end
  handler.go dto.go
apps/api/internal/features/identity/provisioner.go   # impl UserProvisioner + UserReader (Phase 4 khai báo interface; file này hoàn thiện nếu Phase 4 chưa)
apps/api/internal/features/mailer/templates/*        # dùng lại Phase 4: invite, added, resend  <!-- Red Team: RT-02 -->
apps/api/internal/app/{router.go, deps.go}
apps/api/internal/platform/middleware/ratelimit.go  # thêm limiter cho invitations
# Không sửa cmd/lms/seed.go ở phase này (Phase 2 port prototype/seed.js: lớp basic01 active, basic02 active, basic03 draft trên Lập trình cơ bản v1; thành viên + invitations + outbox mẫu gồm một bản failed last_error "Mailbox không tồn tại (550 5.1.1)", attempts 3). Integration test dùng testdb helper chèn hàng tối thiểu đặt tên theo seed.js.  <!-- Red Team: RT-14 / RT-03 - fixture từ seed.js -->
```

## Tasks & Steps

1. `entity.go` + test: `NewDateRange` sai → lỗi; `ChangeCourseVersion` khi active → `ErrClassNotDraft`; `Activate` từ ended → `ErrInvalidTransition`; `End` từ draft → `ErrInvalidTransition` (một chiều, không nhảy cóc); `Drop/Rejoin`.
2. `invite_policy.go` + bảng test 9 nhánh (ended, internal, disabled, already, rejoin, new-without-name, new, invited-keep, invited-rotate, active).
3. `repository_pg.go` + `MemberRow` SQL (`includeDropped`, `invite_status` NULL); map lỗi qua `pgerr.Map(err)` + const `pgerr.UqClassesCode`, `pgerr.CkClassesDates`, `pgerr.UqClassMembersClassUser`, `pgerr.CkClassesStatus` (không tự parse mã SQLSTATE); integration test CHECK/UNIQUE. <!-- Red Team: RT-11 -->
4. Hoàn thiện `identity.UserProvisioner`/`UserReader` (nếu Phase 4 chỉ khai báo interface) và `courses.VersionReader` adapter trong `app/deps.go`.
5. `service.go`: use case 1–10; `Invite` một `Transact` với advisory lock; response `{kind, member}`.
6. Handler + DTO + rate limit invitations.
7. Integration test mời: (a) email mới → `users` invited, `must_change_password=true`, `temp_password_expires_at ≈ now+72h`, `class_members` active, `invitations` kind invite, `email_outbox` `queued` với `secret_enc` không NULL và `payload` không chứa mật khẩu tạm; chạy worker với `LogSender`/Mailpit để lấy mật khẩu rồi login → 200 + `mustChangePassword=true` <!-- Red Team: RT-02 - test (a) -->; (b) user active → kind added, template `added`; (b2) user invited còn hạn mật khẩu tạm → template `added`, hash không đổi, `payload.UsePreviousTempPassword=true`; (b3) user invited hết hạn → xoay, template `invite`, hàng `queued` cũ → `failed` `last_error='superseded'` <!-- Red Team: RT-10 -->; (c) đã trong lớp → 409 `CONFLICT`; (d) ended → 409 `INVALID_TRANSITION` (seed không có lớp `ended`; test tạo bằng testdb helper, ví dụ `basic01` sau khi gọi `End`); (e) teacher email → 403 `FORBIDDEN`; (f) disabled → 409 `ACCOUNT_DISABLED`; (g) dropped → rejoin, `joined_at` giữ, `lesson_progress` cũ còn; (h) resend: hash đổi, mật khẩu cũ login thất bại, mới thành công, 200; lần 4 trong 1 giờ → 429; resend sau khi đã đổi mật khẩu → 409; (i) invite trong tx lỗi enqueue (fake trả lỗi) → không có user/member nào được tạo; (j) 10 request mời cùng email song song → đúng 1 user, 1 member, 9 lần `CONFLICT` (advisory lock + `uq_class_members_class_user`); (k) `GET members` mặc định không có dropped, `includeDropped=true` có; thành viên không có lời mời → `inviteStatus` vắng.
8. Kiểm `make seed` (Phase 2) cho đủ dữ liệu lớp/thành viên để chạy Verification; không thêm seed riêng. <!-- Red Team: RT-14 -->
9. Lint, test, docs.

## Verification

```bash
cd apps/api && go test ./internal/features/classes/...
cd apps/api && go test -tags integration ./internal/features/classes/...
make dev && make seed   # cookie admin c.txt
# fixture: khóa học "Lập trình cơ bản" (cv-basic-1 published), giảng viên huong.le@goup.vn; lớp mới basic04 (seed đã có basic01/basic02 active, basic03 draft)  <!-- Red Team: RT-03 / RT-14 -->
curl -s -b c.txt $H -d '{"code":"basic04","name":"Lập trình cơ bản 04","courseVersionId":"<cv-basic-1>","startDate":"2026-11-01","endDate":"2026-10-01","teacherId":"<huong.le>"}' localhost:8080/api/v1/classes | jq .error.message   # "Ngày kết thúc phải sau ngày bắt đầu."
curl -s -b c.txt $H -d '{"code":"BASIC04","name":"x","courseVersionId":"<cv-basic-1>","startDate":"2026-11-01","endDate":"2027-03-01","teacherId":"<huong.le>"}' localhost:8080/api/v1/classes | jq .error.code   # VALIDATION_FAILED (mã lớp chữ thường)
curl -s -b c.txt $H -d '{..."courseVersionId":"<cv-basic-2 draft>"...}' localhost:8080/api/v1/classes | jq .error.message   # "Chọn một phiên bản khóa học đã phát hành."
curl -s -b c.txt $H -d '{... hợp lệ ...}' localhost:8080/api/v1/classes | jq '{id, status}'   # draft
CL=<id>
curl -s -b c.txt $H -X POST localhost:8080/api/v1/classes/$CL/end | jq .error.message   # "Chỉ chuyển trạng thái một chiều: …"
curl -s -b c.txt $H -d '{"email":"  Moi.Hoc.Vien@Gmail.com ","fullName":"Học Viên Mới"}' localhost:8080/api/v1/classes/$CL/invitations | jq '{kind, inviteStatus: .member.inviteStatus}'  # invited, queued
curl -s -b c.txt $H -d '{"email":"moi.hoc.vien@gmail.com"}' localhost:8080/api/v1/classes/$CL/invitations | jq '{code: .error.code, m: .error.message}'   # CONFLICT "Học viên đã có trong lớp."
curl -s -b c.txt $H -d '{"email":"huong.le@goup.vn"}' localhost:8080/api/v1/classes/$CL/invitations | jq '{code: .error.code, m: .error.message}'   # FORBIDDEN "Email này thuộc tài khoản nội bộ, không mời làm học viên được."
curl -s -b c.txt $H -d '{"email":"thao.vo@gmail.com"}' localhost:8080/api/v1/classes/$CL/invitations | jq .error.code   # ACCOUNT_DISABLED
curl -s -b c.txt $H -d '{"email":"an.nguyen@gmail.com"}' localhost:8080/api/v1/classes/$CL/invitations | jq .kind   # added (tài khoản active)
curl -s -b c.txt $H -d '{"email":"minh.bui@gmail.com"}' localhost:8080/api/v1/classes/$CL/invitations | jq .kind   # added (invited, mật khẩu tạm còn hạn → không xoay)
sleep 6; curl -s 'http://localhost:8025/api/v1/search?query=to:moi.hoc.vien@gmail.com' | jq '.messages[0].Subject'   # "[GoUp LMS] Lời mời vào lớp basic04"
curl -s -b c.txt localhost:8080/api/v1/classes/$CL/members | jq '.items[] | select(.email=="moi.hoc.vien@gmail.com") | {inviteStatus, inviteAttempts}'   # sent, 1
curl -s -b c.txt $H -X POST localhost:8080/api/v1/classes/$CL/members/<mid>/resend -w '%{http_code}'   # 200; Mailpit có mail thứ hai, mật khẩu khác
docker compose -f infra/docker-compose.yml exec -T postgres psql -U lms -d lms -c "select status, last_error from email_outbox where to_email='moi.hoc.vien@gmail.com' order by created_at"   # hàng cũ nếu còn queued → failed/superseded
curl -s -b c.txt $H -X POST localhost:8080/api/v1/classes/$CL/activate | jq .status   # active
curl -s -b c.txt $H -X PATCH -d '{"courseVersionId":"<other published>"}' localhost:8080/api/v1/classes/$CL | jq .error.message   # "Chỉ đổi phiên bản khi lớp còn nháp."
# teacher khác xem lớp → 403
curl -s -b teacher2.txt localhost:8080/api/v1/classes/$CL | jq .error.message   # "Bạn chỉ xem được lớp mình phụ trách."
```

## Security notes

- Mật khẩu tạm chỉ đi vào `Message.Secret` → `secret_enc` trong cùng tx; không có trong `payload`, response, audit, log. `InviteResult` không có field password. <!-- Updated: Validation Session 1 - V1 -->
- Mời cần quyền admin; rate limit 30/phút/IP chống spam email; resend 3 lần/giờ/member.
- Teacher scope kiểm ở service (`teacher_id == actor`), không chỉ ở FE.
- Email chuẩn hóa trước khi tra cứu để tránh tạo tài khoản trùng khác hoa/thường (unique `email_normalized` là lớp chặn cuối).

## Risks & Rollback

| Rủi ro | Biện pháp |
|---|---|
| Transaction mời chéo ba feature dài | Chỉ gồm vài INSERT + một `Seal` AES-GCM; không render template, không I/O mạng trong tx <!-- Red Team: RT-02 --> |
| Hai request mời cùng email đồng thời | Advisory lock theo `lower(email)` + `uq_class_members_class_user`; test (j) <!-- Red Team: RT-10 --> |
| Email nội bộ (teacher) muốn học thử | Ngoài phạm vi; message rõ; mỗi user một vai trò (plan.md quyết định) |
| Resend liên tục gây spam | Rate limit theo member + outbox |
| `avgPercent` ở `/teach/classes` phụ thuộc Phase 8 | Interface `classes.ProgressReader` khai báo ở đây; `zeroProgressReader` trả `null` cho tới khi Phase 8 nối trong `deps.go`; `notLoggedIn`/`inactiveOver7Days` không phụ thuộc Phase 8 |

Rollback: không mount `classes`; user đã tạo qua mời vẫn đăng nhập được nhưng không có lớp.

## Success Criteria

- [ ] Unit + integration `classes` xanh; `invite_policy_test` phủ đủ 10 nhánh (gồm invited-keep).
- [ ] Tạo lớp: bản draft/archived → 422; teacher disabled → 422; `end <= start` → 422; hợp lệ → `status=draft`.
- [ ] `PATCH courseVersionId` chỉ khi draft và chỉ sang published; audit `class.course_version_changed`.
- [ ] `activate`/`end` một chiều; sai → 409 đúng message; audit `class.activated`/`class.ended`.
- [ ] Mời đủ nhánh FR-01 theo bảng mã lỗi (422/409 `INVALID_TRANSITION`/403/409 `ACCOUNT_DISABLED`/409 `CONFLICT`); email mới → user invited + mật khẩu tạm 72h + outbox `invite` (`secret_enc` không NULL, `payload` sạch); user active → outbox `added`; invited còn hạn → `added` không xoay; email chuẩn hóa. <!-- Red Team: RT-10 -->
- [ ] Mailpit nhận mail đúng subject; `GET members` phản ánh `queued → sent`; thành viên chưa có lời mời → `inviteStatus` vắng; `includeDropped` mặc định ẩn dropped; bản `failed` seed hiển thị `inviteAttempts=3`, `inviteLastError`.
- [ ] Resend 200, đổi hash + reset expiry, mật khẩu cũ vô hiệu, hàng `queued` cũ `superseded`; lần 4/giờ → 429; sau khi đổi mật khẩu → 409.
- [ ] Gỡ → `dropped`, `lesson_progress` còn; mời lại → `Rejoin` giữ `joined_at`.
- [ ] Teacher xem lớp người khác → 403 "Bạn chỉ xem được lớp mình phụ trách."; `GET /teach/classes` chỉ trả lớp của mình, có `notLoggedIn`, `inactiveOver7Days` (seed: khang.vu 21 ngày không hoạt động được tính).
- [ ] Không có mật khẩu tạm trong response/log/audit (grep suite integration).
