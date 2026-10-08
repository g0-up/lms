---
phase: 12
title: "Phase 12: Web admin và giảng viên: lớp, học viên, báo cáo"
status: pending
priority: P1
effort: "2.5 ngày"
dependencies: [10, 7, 9]
---

# Phase 12: Web admin và giảng viên: lớp, học viên, báo cáo

## Goal

Hoàn thành vòng đời lớp học trên web cho admin (`/admin/classes`, `/admin/classes/:id` với ba tab Học viên / Tiến độ /
Cài đặt) và khu vực giảng viên (`/teach`, `/teach/classes/:id`), gồm mời học viên, gửi lại lời mời, gỡ khỏi lớp, vô
hiệu hóa/kích hoạt tài khoản, kích hoạt/kết thúc lớp, và báo cáo tiến độ FR-40 với bộ lọc, sắp xếp, drawer chi tiết
từng học viên. Mọi lời thoại bám `prototype/app.js` (`pageClasses`, `pageClassDetail`, `renderStudentsTab`,
`renderSettingsTab`, `renderReport`, `studentDetailDrawer`, `teacherClassCards`, `pageTeachClasses`,
`pageTeachClass`).

## Context & Requirements

- Phụ thuộc: phase 10 (shell, `shared/ui`, router, hợp đồng `routes.tsx`), phase 7 (API
  classes/members/invitations/users **và `GET /teach/classes`**), phase 9 (API reports, course versions cho select).
  <!-- Red Team: RT-10 - /teach/classes thuộc phase 7 -->
- **Quy tắc sở hữu file:** chỉ thêm/sửa dưới `src/features/classes/**` và `src/features/reports/**`.
  Không sửa `src/app`, `src/shared`, `src/styles`, cũng không sửa `features/courses`
  (danh sách phiên bản khóa học cho Select lấy qua `GET /courses` trong `features/classes/api`).
  `features/classes/routes.tsx` export `routes` (tương đối dưới `/admin`: `'classes'`, `'classes/:classId'`).
  `features/reports/routes.tsx` export `teachRoutes` (dưới `/teach`: `''`, `'classes/:classId'`) và `index.ts`
  export `ReportPanel`, `StudentDetailDrawer` để `features/classes` import qua `@/features/reports`.
- API (base `/api/v1`):
  - `GET/POST /classes`, `GET/PATCH /classes/{id}`, `POST /classes/{id}/activate|end`.
  - `GET /classes/{id}/members?includeDropped=` (tab Học viên gọi `includeDropped=true` vì prototype hiện cả hàng đã
    rời lớp), `POST /classes/{id}/invitations {email, fullName}` → `201 {kind, member}`,
    `POST /classes/{id}/members/{mid}/resend` → `200 {member}`, `DELETE /classes/{id}/members/{mid}` → `200 MemberDTO`.
    `MemberDTO` phẳng theo phase 7: `{id (class_members.id), userId, email, fullName, accountStatus, memberStatus,
    tempPasswordExpiresAt?, inviteStatus?, inviteKind?, inviteAttempts?, inviteLastError?, invitedAt?, lastLoginAt?,
    lastActiveAt?, joinedAt, droppedAt?}`; `inviteStatus` vắng/null khi chưa có lời mời. <!-- Red Team: RT-10 - MemberDTO -->
  - `GET /users?role=teacher`, `POST /users/{id}/disable|enable`.
  - `GET /classes/{id}/report?includeDropped=&notLoggedIn=&inactiveDays=&belowPercent=&sort=` (tên tham số API;
    URL trang dùng tên ngắn `below`, `notlogged`, `inactive` theo prototype và `parseFilter` ánh xạ sang tên API
    <!-- Red Team: RT-14 - tên tham số URL ↔ API -->),
    `GET /classes/{id}/report/members/{mid}` với `mid` = `class_members.id` (không phải `userId`); 404 khi `mid`
    không thuộc lớp. <!-- Red Team: RT-15 -->
  - `GET /teach/classes`; `GET /courses` (để lấy phiên bản published).
- Mã lỗi: `VALIDATION_FAILED`, `CONFLICT` (mã lớp trùng, học viên đã trong lớp), `INVALID_TRANSITION`,
  `FORBIDDEN` (giảng viên xem lớp không phụ trách), `NOT_FOUND`, `ACCOUNT_DISABLED`.
- Quy ước: toast thành công theo bảng; lỗi trong modal → `Alert danger role=alert` trong modal;
  ngoài modal → `toast.error(message)`. Một nút `gradient` mỗi trang.

## Design system / Architecture

- `features/classes/`
  - `api/classes-api.ts`, `api/msw-handlers.ts`
  - `model/{schemas.ts, member-view.ts, class-form.ts, invite-form.ts}`
  - `hooks/{use-classes, use-class, use-members, use-teachers, use-published-course-versions,
    use-class-mutations, use-member-mutations, use-user-mutations}.ts`
  - `components/{class-table, class-tabs, lifecycle-action, students-tab, member-row, invite-dialog,
    settings-tab, new-class-dialog}.tsx`
  - `pages/{classes-page, class-detail-page}.tsx`, `routes.tsx`, `index.ts`
- `features/reports/`
  - `api/reports-api.ts`, `api/msw-handlers.ts`
  - `model/{schemas.ts, report-filter.ts, sort.ts}` (Specification-like: `ReportFilter` ↔ query string)
  - `hooks/{use-report, use-member-report, use-teach-classes}.ts`
  - `components/{report-panel, filter-bar, report-table, stage-header, student-detail-drawer,
    teacher-class-card}.tsx`
  - `pages/{teach-classes-page, teach-class-page}.tsx`, `routes.tsx`, `index.ts`
- `model/member-view.ts` (thuần TS): `accountState(member)` → `disabled | invited-expired | invited | active`
  với nhãn "Vô hiệu hóa" / "Mật khẩu tạm hết hạn" / "Chưa đăng nhập" / "Đã kích hoạt";
  `inviteCell(member)` → `{dot, label, secondary}`; `sortMembers` (dropped cuối, rồi tên theo `localeCompare('vi')`).
- `model/report-filter.ts`: `parseFilter(searchParams)` đọc URL `notlogged`, `inactive`, `below`, `dropped`, `sort` →
  `{notLoggedIn, inactiveDays, belowPercent, includeDropped, sort}` (`sort ∈ name|pct|pct-desc|activity|activity-asc`,
  mặc định `name`; `includeDropped` mặc định `false`); `toQuery(filter)` xuất tên ngắn cho URL, `toApiParams(filter)`
  xuất `notLoggedIn`, `inactiveDays`, `belowPercent`, `includeDropped` cho API <!-- Red Team: RT-14 -->; `hasAnyFilter`;
  `toggleSort(current, column)` (pct ↔ pct-desc, activity ↔ activity-asc).
- Query keys: `['classes']`, `['class', id]`, `['members', id]`, `['teachers']`, `['course-versions-published']`,
  `['report', id, filter]`, `['member-report', id, mid]` (`mid` = `class_members.id`), `['teach-classes']`.
- `ReportPanel` nhận `classId`, `basePath` (`/admin/classes/{id}?tab=report` hoặc `/teach/classes/{id}`) và
  đồng bộ filter với URL qua `useSearchParams` (giữ `tab=report` khi ở admin).

## Pages & components

### `/admin/classes` — ClassesPage (title "Lớp học")

- `PageHead` h1 "Lớp học", lede "Mỗi lớp chạy trọn đời trên một phiên bản khóa học. Đổi phiên bản chỉ khi lớp còn ở
  trạng thái nháp.", `Button gradient` icon `Plus` "Tạo lớp".
- Bảng `TableWrap`: Lớp (`cell-2` mã link → detail + tên), Khóa học (tên + `VersionPill size=sm`),
  Trạng thái (`Badge` theo `STATUS_VI`), Giảng viên, Học viên (`num`), Tiến độ TB (`ProgressBar` hoặc "—" khi nháp),
  Bắt đầu (`num`, `fmtDate`). Rỗng: EmptyState "Chưa có lớp" / "Tạo lớp để mời học viên." + nút "Tạo lớp".
- NewClassDialog: trước khi mở, nếu không có phiên bản khóa học published → `toast.error("Chưa có phiên bản khóa học
  nào được phát hành.")`.
  Dialog "Tạo lớp" confirm "Tạo lớp": `form-grid` "Mã lớp" (`required`, "vd: basic04"), "Tên lớp" (`required`, "vd:
  Lập trình cơ bản – khóa 4"),
  `span-2` Select "Phiên bản khóa học" (chỉ published, mặc định bản mới nhất, help "Chỉ phiên bản đã phát hành. Đổi
  được khi lớp còn ở trạng thái nháp."),
  "Ngày bắt đầu dự kiến" (mặc định hôm nay +14), "Ngày kết thúc dự kiến" (+120), `span-2` Select "Giảng viên phụ
  trách" (`GET /users?role=teacher`).
  → `POST /classes` → toast "Đã tạo lớp ở trạng thái nháp." → `/admin/classes/{id}`.
  Lỗi client: "Nhập ngày bắt đầu và kết thúc dự kiến.", "Ngày kết thúc phải sau ngày bắt đầu.", "Chọn giảng viên phụ
  trách.";
  `CONFLICT` → lỗi field "Mã lớp đã tồn tại."

### `/admin/classes/:classId` — ClassDetailPage (title = mã lớp)

- `Crumbs` [Lớp học → `/admin/classes`, mã]; h1 mã lớp + `Badge` trạng thái;
  lede "{course} v{n} · {fmtDate(start)} → {fmtDate(end)} · Giảng viên {teacherName}".
- LifecycleAction (góc phải PageHead): draft → `Button gradient` icon `Play` "Kích hoạt lớp"; active → `Button
  outline` "Kết thúc lớp"; ended → không nút.
  - Kích hoạt: ConfirmDialog "Kích hoạt lớp {code}?" text "{n} học viên sẽ bắt đầu học và tích hoàn thành được. Sau
    khi kích hoạt, lớp không đổi được phiên bản khóa học."
    confirm "Kích hoạt" → `POST .../activate` → toast "Lớp {code} đang chạy." `INVALID_TRANSITION` → toast message
    server.
  - Kết thúc: dialog danger "Kết thúc lớp {code}?" text "Học viên chỉ còn xem học liệu, không tích hoàn thành được
    nữa. Không mời thêm học viên được. Không hoàn tác được."
    confirm "Kết thúc lớp" → `POST .../end` → toast "Lớp {code} đã kết thúc."
- ClassTabs `nav.tabs aria-label="Mục của lớp"`: link `?tab=students` "Học viên" + `.count` (số thành viên không
  `dropped`), `?tab=report` "Tiến độ", `?tab=settings` "Cài đặt";
  `aria-current="page"` cho tab đang mở; mặc định `students`; tab lạ → students.

#### Tab Học viên — StudentsTab

- Card head h2 "Học viên trong lớp" + `Button default sm` icon `Mail` "Mời học viên"
  (ended → `aria-disabled` title "Không mời được vào lớp đã kết thúc", click → toast lỗi cùng text).
- Bảng cột: Học viên (`cell-2` `fullName` + email), Tài khoản (`StatusDot` theo `accountStatus`: `disabled` → "Vô hiệu
  hóa"; `invited` → "Mật khẩu tạm hết hạn" nếu `tempPasswordExpiresAt < now` else "Chưa đăng nhập"; `active` → ok
  "Đã kích hoạt"),
  Lời mời (`cell-2`: dot theo `inviteStatus` + nhãn `STATUS_VI` (queued "Đang chờ gửi" / sent "Đã gửi" / failed "Gửi
  thất bại"); `inviteKind=added` và `sent` → "Đã gửi thông báo"; `.secondary`: failed → "{inviteLastError} ·
  {inviteAttempts} lần thử", khác → `rel(invitedAt)`; `inviteStatus` null/vắng → ô chỉ hiện "—", không dot)
  <!-- Red Team: RT-02 / RT-10 - inviteStatus nullable, field phẳng -->,
  Tiến độ (`ProgressBar` hoặc "—" khi dropped), Thao tác.
- Thao tác: `memberStatus=dropped` → `muted` "Đã rời lớp"; khác → `Button ghost sm` "Gửi lại" (chỉ khi
  `accountStatus === 'invited'`),
  "Kích hoạt lại" (khi disabled) hoặc "Vô hiệu hóa", "Gỡ khỏi lớp". Hàng dropped `opacity-60`; sắp xếp `sortMembers`.
- Rỗng: "Chưa có học viên" / "Mời học viên bằng email; họ sẽ nhận mật khẩu tạm có hiệu lực 72 giờ." + nút "Mời học
  viên" (ẩn khi ended).
- InviteDialog "Mời học viên vào {code}" confirm "Gửi lời mời": "Email" (`type=email required`, placeholder
  "hocvien@example.com", help "Email được chuẩn hóa (cắt khoảng trắng, chữ thường) trước khi so khớp."),
  "Họ tên" (`fullName`, placeholder "Bỏ trống nếu email đã có tài khoản", help "Bắt buộc khi email chưa có tài khoản."),
  `Alert info` "Email mới: tạo tài khoản, gửi mật khẩu
  tạm hiệu lực 72 giờ. Email đã có tài khoản: chỉ thêm vào lớp và gửi thông báo. Mật khẩu tạm không bao giờ hiển thị
  trên giao diện này."
  → `POST .../invitations {email, fullName}` → `201 {kind:'invited'|'added', member}`: `added` → toast
  "{member.fullName} đã có tài khoản: đã thêm vào lớp và gửi thông báo."; `invited` → "Đã tạo tài khoản cho
  {member.fullName}, lời mời đang được gửi." (copy toast do FE chọn theo `kind`; response không mang message);
  invalidate `['members']`, `['class']`, `['dashboard']`.
  Lỗi client "Email không hợp lệ."; server ánh xạ **đúng từng mã** của phase 7 (status → `error.code` → copy):
  `422 VALIDATION_FAILED` → "Nhập họ tên học viên." (khi `details.field === 'fullName'`) hoặc "Email không hợp lệ.";
  `409 INVALID_TRANSITION` → "Không mời được vào lớp đã kết thúc."; `403 FORBIDDEN` → "Email này thuộc tài khoản nội
  bộ, không mời làm học viên được."; `409 ACCOUNT_DISABLED` → "Tài khoản đã bị vô hiệu hóa. Kích hoạt lại trước khi
  mời."; `409 CONFLICT` → "Học viên đã có trong lớp."; mã khác → `error.message` server. <!-- Red Team: RT-10 - ánh xạ mã lỗi -->
- Gửi lại: dialog "Gửi lại lời mời cho {name}?" text "Mật khẩu tạm cũ mất hiệu lực ngay; mật khẩu tạm mới có hiệu lực
  72 giờ kể từ bây giờ." confirm "Gửi lại" → `POST .../resend` → `200 {member}` → toast "Lời mời mới đang được gửi.";
  `409 CONFLICT` → toast lỗi "Học viên đã đổi mật khẩu; không cần gửi lại lời mời."; `429 RATE_LIMITED` → "Đã gửi lại
  quá nhiều lần. Thử lại sau." <!-- Red Team: RT-10 -->
- Gỡ khỏi lớp: dialog danger "Gỡ {name} khỏi lớp?" text "Thành viên chuyển sang trạng thái đã rời lớp; dữ liệu tiến độ
  được giữ nguyên." confirm "Gỡ khỏi lớp" → `DELETE` → toast "Đã gỡ {name} khỏi lớp."
- Vô hiệu hóa: dialog danger "Vô hiệu hóa tài khoản {email}?" text "Tài khoản không đăng nhập được, phiên hiện tại bị
  hủy. Tiến độ học giữ nguyên." confirm "Vô hiệu hóa" → `POST /users/{id}/disable` → toast "Đã vô hiệu hóa tài khoản."
  Kích hoạt lại: không dialog, `POST /users/{id}/enable` → toast "Đã kích hoạt lại tài khoản."
- Trạng thái lời mời `queued` → poll `refetchInterval` 5s cho `['members', id]` khi còn phần tử queued (tối đa 2
  phút), để dot chuyển sent/failed không cần tải lại.

#### Tab Tiến độ — ReportPanel (từ `@/features/reports`)

Xem mục "ReportPanel" dưới; `basePath=/admin/classes/{id}`, giữ `tab=report` trong URL.

#### Tab Cài đặt — SettingsTab

- Card form "Cài đặt lớp". Khi `status !== 'draft'` → `Alert info` "Lớp {STATUS_VI lower} không đổi được phiên bản
  khóa học. Lớp chạy trọn đời trên {course} v{n}."
- `form-grid`: "Mã lớp" (Input disabled), "Tên lớp" (`required`), Select "Phiên bản khóa học" (published + bản hiện
  tại; disabled khi không nháp; help "Chỉ liệt kê phiên bản đã phát hành."),
  Select "Giảng viên phụ trách", "Ngày bắt đầu dự kiến", "Ngày kết thúc dự kiến" (`type=date`); `Alert danger` lỗi;
  `form-actions` `Button default` "Lưu thay đổi" (disabled khi form không đổi).
- Submit `PATCH /classes/{id}` → toast "Đã lưu cài đặt lớp."; lỗi client: "Ngày kết thúc phải sau ngày bắt đầu.",
  "Chọn giảng viên phụ trách.", "Nhập ngày bắt đầu và kết thúc dự kiến.";
  server `INVALID_TRANSITION`/`VALIDATION_FAILED` đổi phiên bản khi không nháp → "Chỉ đổi sang phiên bản đã phát
  hành." hoặc message server.

### ReportPanel (FR-40) — dùng chung admin và giảng viên

- FilterBar `form.filter-bar` (GET, submit → cập nhật search params): Checkbox "Chưa đăng nhập" (URL `notlogged=1`),
  Field số "Không hoạt động quá (ngày)" (URL `inactive`, `min=1`, placeholder 7), Field số "% toàn khóa dưới" (URL
  `below`, 1–100, placeholder 50), Checkbox "Hiện học viên đã rời lớp" (URL `dropped=1` → API `includeDropped=true`;
  mặc định tắt, báo cáo chỉ gồm thành viên `active`) <!-- Updated: Validation Session 1 - V5 includeDropped -->,
  Select "Sắp xếp" (Tên A → Z / % toàn khóa thấp → cao / % toàn khóa cao → thấp / Hoạt động mới nhất / Lâu không hoạt
  động), `Button default sm` "Lọc", `Button ghost sm` "Xóa bộ lọc" (link về `basePath`, chỉ hiện khi `hasAnyFilter`).
- Card head: `small muted` "{rows}/{total} học viên · Bấm vào một dòng để xem từng học liệu" (`total` = số thành viên
  `active`; hàng đã rời lớp không tính vào `total` lẫn KPI/trung bình, kể cả khi đang hiện) + `Badge warn` icon
  `Info` "Tiến độ do học viên tự xác nhận". <!-- Updated: Validation Session 1 - V5 KPI chỉ active -->
- Bảng: Học viên (`cell-2`), Lời mời (dot/“—”), mỗi chặng `th.num title="{stage} v{n}"` hiện mã chặng, "Toàn khóa"
  (`num`, sortable), "Hoạt động cuối" (sortable).
  `th` sortable render `button.sort` với `aria-sort="ascending|descending"` khi đang sort theo cột; click →
  `toggleSort`.
  Hàng `tr.is-clickable tabIndex=0 role=button aria-label="Xem chi tiết {name}"`, Enter/Space hoặc click → mở drawer.
  Ô: từng chặng "{pct}%" (màu ok khi 100) hoặc "—"; Toàn khóa `ProgressBar`; Hoạt động cuối `cell-2`
  `rel(lastActivityAt)` + `fmtDateTime` hoặc `StatusDot invited` "Chưa đăng nhập". Hàng `memberStatus=dropped` (chỉ
  khi bật checkbox) thêm `Badge muted` "Đã rời lớp" cạnh tên và `opacity-60`. <!-- Updated: Validation Session 1 - V5 -->
- Rỗng: có filter → "Không có học viên khớp bộ lọc" / "Nới lỏng điều kiện hoặc xóa bộ lọc." + `Button outline sm` "Xóa
  bộ lọc"; không filter → "Chưa có học viên" / "Mời học viên vào lớp để theo dõi tiến độ."
- Lỗi query → `Alert danger` "Không tải được báo cáo." + "Thử lại". Loading → Skeleton 6 hàng, giữ filter bar tương
  tác.

### StudentDetailDrawer (`Sheet` phải 520px, `aria-label="Chi tiết học viên"`)

- Head h2 tên, `small muted` "{email} · {code} · hoạt động cuối {rel}", IconButton "Đóng".
- `card-pad`: `ProgressBar size=lg` + "{done}/{total} học liệu bắt buộc · tiến độ do học viên tự xác nhận".
- `panel-body` mỗi chặng: head (bg surface-50) h2 "{i}. {stage} <span muted>v{n}</span>" + `ProgressBar` 140px; list
  học liệu: `TypeIcon`, title + `Badge subtle` "Không bắt buộc", meta "Mở lần đầu {fmtDateTime}" / "Chưa mở",
  `StatusDot ok` "Tích {fmtDateTime}" khi hoàn thành + icon `CircleCheck` màu ok.
- Dữ liệu `GET /classes/{id}/report/members/{mid}` với `mid` = `row.id` (`class_members.id`); `404 NOT_FOUND` →
  `toast.error("Không tìm thấy học viên trong lớp.")` và đóng drawer <!-- Red Team: RT-15 -->; Escape/scrim đóng, focus
  trả về hàng vừa mở; body `overflow hidden` khi mở; ≤960 full width.

### `/teach` — TeachClassesPage (title "Lớp của tôi")

- `PageHead` h1 "Lớp của tôi", lede "Các lớp bạn phụ trách. Vào lớp để xem tiến độ từng học viên theo chặng."
- `grid-3` TeacherClassCard `a.card.card-pad` → `/teach/classes/{id}` (dữ liệu `GET /teach/classes` phase 7: `{id,
  code, name, status, startDate, endDate, courseName, courseVersionNo, memberCount, avgPercent, notLoggedIn,
  inactiveOver7Days}`): `row-between` h2 mã + `Badge` trạng thái; `muted small` "{name} · {courseName}
  v{courseVersionNo}";
  draft → `small` "Bắt đầu {fmtDate(start)}" else `ProgressBar(avgPct)`; `row small muted` "{n} học viên" + `StatusDot
  invited` "{stale} lâu không hoạt động" (chỉ lớp active, `inactiveOver7Days > 0`).
- Rỗng: "Chưa có lớp" / "Bạn chưa được phân công lớp nào."

### `/teach/classes/:classId` — TeachClassPage (title = mã lớp)

- `GET /classes/{id}` trả `FORBIDDEN`/`NOT_FOUND` → `toast.error("Bạn chỉ xem được lớp mình phụ trách.")` +
  `navigate('/teach', {replace:true})`.
- `Crumbs` [Lớp của tôi → `/teach`, mã]; h1 mã + `Badge`; lede "{course} v{n} · {start} → {end}"; không tab, không nút
  lifecycle.
- `ReportPanel basePath=/teach/classes/{id}` (filter trong query string không có `tab`).

## Files to Create / Modify

- `src/features/classes/**` theo cây Architecture; ghi đè `routes.tsx` stub.
- `src/features/reports/**` theo cây Architecture; ghi đè `routes.tsx` stub (export `teachRoutes`; `routes` giữ `[]`).
- Test: `model/*.test.ts` (member-view, report-filter, sort, class-form dates), `pages/*.test.tsx`,
  `components/report-panel.test.tsx`.

## Tasks & Steps

1. Schemas zod (enum `status` lớp `draft|active|ended`, `memberStatus` `active|dropped|completed`, `accountStatus`
   `invited|active|disabled`, `inviteStatus` nullable `queued|sent|failed` <!-- Red Team: RT-03 / Validation Session 1 - V3 -->)
   + api + MSW handlers cho classes, members, invitations, users, reports, teach-classes; fixture MSW chép từ JSON vàng
   của phase 7 `apps/api/internal/features/classes/testdata/*.json` (sinh từ seed `prototype/seed.js`: lớp `basic01`,
   `basic02` active, `basic03` draft; thành viên `an.nguyen`, `dung.pham` (temp hết hạn), `minh.bui` (invited),
   `khang.vu` (21 ngày không hoạt động), `thao.vo` (disabled), một lời mời `failed` "Mailbox không tồn tại (550
   5.1.1)" 3 lần thử) để hai bên không lệch shape. <!-- Red Team: RT-10 - fixture từ golden JSON -->
2. `model/`: `accountState`, `inviteCell` (null `inviteStatus` → "—"), `sortMembers`, `parseFilter/toQuery/toApiParams/
   toggleSort` (test ánh xạ `below`→`belowPercent`, `notlogged`→`notLoggedIn`, `inactive`→`inactiveDays`,
   `dropped`→`includeDropped`), `classFormSchema` (date so sánh), `inviteFormSchema` (`fullName` string, rỗng bỏ); unit test.
3. ClassesPage + NewClassDialog (chặn khi không có bản published, default dates).
4. ClassDetailPage: head, LifecycleAction, ClassTabs theo `?tab=`.
5. StudentsTab + MemberRow + InviteDialog + resend/remove/disable/enable + polling queued.
6. SettingsTab (RHF, `isDirty`, khóa select phiên bản khi không nháp).
7. `features/reports`: FilterBar, ReportTable (sortable `aria-sort`, hàng clickable có keyboard), StudentDetailDrawer,
   ReportPanel đồng bộ URL.
8. TeachClassesPage, TeachClassPage, `teachRoutes`; chặn `FORBIDDEN` → `/teach`.
9. Test: StudentsTab (ba trạng thái tài khoản, ô Lời mời rỗng khi `inviteStatus` null, nút theo điều kiện, ended
   disable mời), InviteDialog (hai toast theo `kind`, 5 lỗi server đúng mã: 422/409 `INVALID_TRANSITION`/403/409
   `ACCOUNT_DISABLED`/409 `CONFLICT`), ReportPanel (filter ↔ URL tên ngắn, request MSW nhận tên API, checkbox đã rời
   lớp → `includeDropped=true` và KPI không đổi, sort toggle, drawer mở bằng Enter, drawer 404 → toast + đóng),
   TeachClassPage (FORBIDDEN redirect).
10. Lint/typecheck/`lint:design`/build; kiểm thủ công với API phase 7/9 thật; đối chiếu prototype `#/admin/classes/*`
    và `#/teach/*`.

## Verification

- Vitest xanh; `pnpm lint typecheck build` xanh; `git diff --stat` chỉ gồm `src/features/{classes,reports}`.
- Kịch bản thủ công (API thật + mail dev, seed `make seed`): tạo lớp nháp `basic04` → mời email mới → dot "Đang chờ
  gửi" → poll sang "Đã gửi" → học viên login (phase 10) → tài khoản "Đã kích hoạt" → kích hoạt lớp → học viên tích
  (phase 13) → tab Tiến độ thấy % → lọc "% toàn khóa dưới 50" (URL `?below=50`, request API `belowPercent=50`) → drawer
  đúng từng học liệu → gỡ một học viên → tab Tiến độ không còn hàng đó, bật "Hiện học viên đã rời lớp" → hàng hiện với
  badge "Đã rời lớp", "{rows}/{total}" không đổi → kết thúc lớp → nút Mời disabled đúng lý do.
- Lớp `basic01` seed: thành viên không có lời mời → ô Lời mời "—"; `dung.pham` → "Mật khẩu tạm hết hạn"; lời mời
  `failed` hiện "Mailbox không tồn tại (550 5.1.1) · 3 lần thử".
- Lỗi: mời trùng (409 `CONFLICT`) → "Học viên đã có trong lớp."; mời `huong.le@goup.vn` (403) → "Email này thuộc tài
  khoản nội bộ, không mời làm học viên được."; mời `thao.vo@gmail.com` (409 `ACCOUNT_DISABLED`) → "Tài khoản đã bị vô
  hiệu hóa. Kích hoạt lại trước khi mời."; gửi lại cho người đã đổi mật khẩu → đúng toast; gửi lại lần 4 trong giờ →
  "Đã gửi lại quá nhiều lần. Thử lại sau."; giảng viên khác gõ URL lớp lạ → về `/teach` + toast.
- a11y: tabs là `nav` có `aria-label`, `aria-current`; `aria-sort` đúng; hàng báo cáo mở bằng Enter/Space; drawer là
  dialog modal có focus trap và trả focus; dialog `aria-labelledby`; axe 0 lỗi nghiêm trọng; 320px: bảng cuộn trong
  `TableWrap`, filter bar xuống dòng, drawer full width.

## Security notes

- Mật khẩu tạm không bao giờ xuất hiện trong response hay UI; `Alert info` trong InviteDialog nói rõ điều này. Nếu API
  vô tình trả trường `tempPassword`, zod schema `strict()` loại bỏ và test fail.
- Quyền giảng viên do server quyết (`FORBIDDEN`); UI chỉ chuyển hướng. Admin không thấy menu `/teach` (quyết định
  plan.md §11).
- `lastError` của lời mời là chuỗi từ hệ thống mail; render dạng text (React escape), cắt 120 ký tự với `title` đầy
  đủ.
- Email trong form `trim().toLowerCase()` trước khi gửi, nhưng server vẫn chuẩn hóa lại.
- Filter từ URL được `parseFilter` ép kiểu (`z.coerce.number().int().min(1)`), giá trị lạ bỏ qua thay vì gửi thẳng lên
  API; chỉ `toApiParams` quyết định tên tham số gửi lên, không forward nguyên search params. <!-- Red Team: RT-14 -->

## Risks & Rollback

- **Polling queued** gây tải khi nhiều lớp mở: giới hạn 2 phút và chỉ khi tab Học viên hiển thị (`enabled`); fallback
  tắt polling, người dùng tải lại.
- **Report nhiều chặng** làm bảng rộng: `TableWrap` cuộn ngang, `th` mã chặng có `title` tên đầy đủ; chấp nhận cho
  MVP.
- **Shape báo cáo phase 9 lệch** (per-stage pct, `lastActivityAt`): zod báo sớm; sửa tại `api/`. Shape
  `MemberDTO`/`/teach/classes` của phase 7 đã chốt ở trên và được khóa bằng golden JSON dùng chung cho MSW.
- **Tên tab `?tab=`** không khớp route table plan.md (teacher không có tab): ghi Concerns; UI theo prototype.
- Rollback: revert commit `src/features/{classes,reports}`; hai `routes.tsx` về stub rỗng, app vẫn build.

## Success Criteria

- [ ] `/admin/classes`, `/admin/classes/:id` (3 tab), `/teach`, `/teach/classes/:id` khớp 1:1 prototype về bố cục, lời
  thoại, toast, trạng thái rỗng/disabled. <!-- copy, toasts, empty/disabled states follow this phase file and are asserted by component tests; four pages walked live with Playwright (screenshots, axe 0 serious/critical). Not compared side by side with the prototype, so left open. -->
- [x] Vòng đời lớp draft → active → ended và mọi thao tác thành viên chạy thật với API phase 7, mỗi mã lỗi lời mời
  (422/409 `INVALID_TRANSITION`/403/409 `ACCOUNT_DISABLED`/409 `CONFLICT`/429) ra đúng copy; ô Lời mời rỗng khi không có
  lời mời. <!-- Red Team: RT-10 --> <!-- live (API + worker + Mailpit): create draft, taken code on field, activate, invite new (queued → sent, mail received) and existing (added notice), duplicate → "Học viên đã có trong lớp.", resend (new mail), remove → "Đã rời lớp". End, disable/enable, rejoin and the other five error codes are covered by MSW tests only. -->
- [x] Báo cáo FR-40 lọc/sắp xếp đồng bộ URL (tên ngắn `below`/`notlogged`/`inactive`/`dropped`, chia sẻ link giữ filter)
  và gọi API bằng `belowPercent`/`notLoggedIn`/`inactiveDays`/`includeDropped`; mặc định ẩn học viên đã rời lớp, bật
  checkbox mới hiện kèm badge, KPI chỉ tính `active`; drawer theo `class_members.id`, 404 xử lý êm; dùng chung được
  cho admin và giảng viên. <!-- Red Team: RT-14 / RT-15; Validation Session 1 - V5 --> <!-- report-panel/report-filter tests (API param names, includeDropped keeps KPI, sort toggle, Enter opens drawer, 404 closes with toast); live: admin basic01 below=50 + dropped + sort in URL, drawer on Enter with stages and focus return; teacher /teach list, inactive=7 filter, drawer; other teacher's class → /teach with the 403 message. -->
- [ ] Không file nào ngoài `src/features/{classes,reports}` bị sửa; lint, typecheck, test, build xanh. <!-- only src/features/{classes,reports} edited; lint, lint:design, typecheck, production bundle green; feature tests 15 files / 182 green. Full suite 466/469: three src/app shell tests (app-shell ×2, session-isolation) still expect the old route stubs and need an update owned by src/app, so left open. -->
