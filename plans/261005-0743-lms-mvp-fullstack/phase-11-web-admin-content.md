---
phase: 11
title: "Phase 11: Web admin: dashboard, chặng, khóa học"
status: pending
priority: P1
effort: "3 ngày"
dependencies: [5, 6, 9, 10]
---

# Phase 11: Web admin: dashboard, chặng, khóa học

## Goal

Hoàn thành phần nội dung của admin trên web: trang Tổng quan (`/admin`), quản lý Chặng với phiên bản và học liệu
(`/admin/stages`, `/admin/stages/:id`), quản lý Khóa học với phiên bản và danh sách chặng (`/admin/courses`,
`/admin/courses/:id`), kể cả luồng FR-18 "áp dụng phiên bản chặng mới cho khóa học bằng một thao tác" và upload video
qua presigned URL. Mọi trang, nút, modal, toast và thông điệp lỗi bám 1:1 với `prototype/app.js`
(`pageAdminDashboard`, `pageStages`, `pageStageDetail`, `pageCourses`, `pageCourseDetail` và `ACTIONS` tương ứng).

## Context & Requirements

- Phụ thuộc: phase 10 (shell, `shared/ui`, `http`, router, hợp đồng `routes.tsx`), phase 5 (API
  stages/stage-versions/lessons/media), phase 6 (API courses/course-versions/apply), phase 9 (API dashboard).
  <!-- Red Team: RT-07 - correct API owner phases -->
- **Quy tắc sở hữu file:** phase này chỉ thêm/sửa file dưới `src/features/dashboard/**`, `src/features/stages/**`,
  `src/features/courses/**`. Không sửa `src/app`, `src/shared`, `src/styles`. Thiếu component dùng chung thì viết
  trong `features/<f>/components` và ghi vào Concerns để phase sau cân nhắc nâng lên `shared`. Route đăng ký qua
  `features/<f>/routes.tsx` export `routes: RouteObject[]` với `path` tương đối (`''`, `'stages'`,
  `'stages/:stageId'`, `'courses'`, `'courses/:courseId'`) và `handle.title`; `app/router.tsx` của phase 10 đã import
  sẵn.
- API (base `/api/v1`, plan.md §7 là bản chuẩn; zod schema bám đúng tên trường dưới đây): `GET /dashboard` →
  `{kpis:{stages,courses,classes,students}, hints:{draftClasses,notLoggedIn,outdatedCourses,failedInvites},
  outdated:[{courseId,courseCode,courseName,courseVersionId,courseVersionNo,stageId,stageCode,stageName,usingVersionNo,latestVersionId,latestVersionNo}],
  classes:[{id,code,name,status,courseName,courseVersionNo,memberCount,avgPercent}],
  recentActivity:[{id,at,actorName,actionLabel,target:{type,id,label},summary}]}`; `GET /stages` → `{items:[{id,code,name,
  description?,latestVersionNo,latestPublishedNo?,draftVersionId?,versions:[{id,versionNo,status,publishedAt?,lessonCount}],
  usedByCourseCount,outdatedCourseCount}]}`, `POST /stages {code,name,description?}`, `GET /stages/{id}` (kèm `versions[]`,
  `usedBy:[{courseId,courseCode,courseName,courseVersionId,versionNo,stageVersionNo,outdated,classCodes[]}]` và
  `outdatedCourses:[{courseId,courseCode,courseName,courseVersionId,courseVersionNo,usingVersionNo,latestVersionId,latestVersionNo,canApply,blockedReason?,draftVersionId?,draftVersionNo?}]`),
  `GET /stage-versions/{vid}` → `{id,stageId,stageCode,stageName,versionNo,status,publishedAt?,clonedFromVersionNo?,
  lessons:[{id,lessonKey,title,type,required,position,durationSeconds?,markdownSource?,markdownHtml?,videoMediaId?,videoFileName?}],
  usedBy[]}`, `POST /stage-versions/{vid}/clone|publish|archive`, `DELETE /stage-versions/{vid}` → `{stageDeleted}`,
  `POST /stage-versions/{vid}/lessons {lessonKey?,title,type,required,durationSeconds?,markdownSource?,videoMediaId?}`,
  `PATCH/DELETE /stage-versions/{vid}/lessons/{lid}`, `PUT /stage-versions/{vid}/lessons/order {lessonIds}`,
  `POST /stage-versions/{vid}/apply {courseIds}` → `{results:[{courseId,courseCode,newVersionNo?,error?:{code,message}}]}`;
  `POST /media/uploads {kind,fileName,contentType,sizeBytes}` → `201 {mediaId,uploadUrl,expiresAt}`,
  `POST /media/uploads/{id}/complete`, `GET /media/{id}/url` → `{url,expiresAt}`; `GET /courses` → `{items:[{id,code,name,
  description?,latestVersionNo,latestPublishedNo?,draftVersionId?,versions:[{id,versionNo,status,publishedAt?,stageCount,
  outdatedStageCount}],classesUsing:[{classId,code,name,versionNo}]}]}`, `POST /courses`, `GET /courses/{id}`,
  `GET /course-versions/{vid}` → `{id,courseId,courseCode,courseName,versionNo,status,publishedAt?,clonedFromVersionNo?,
  stages:[{position,stageId,stageCode,stageName,stageVersionId,stageVersionNo,lessonCount,requiredCount,latestPublishedNo?,outdated}],
  classes:[{classId,code,name,status,memberCount}],newerPublished}`, `PUT /course-versions/{vid}/stages {stageVersionIds}`,
  `POST /course-versions/{vid}/clone|publish|archive`, `DELETE /course-versions/{vid}` → `{courseDeleted}`. Mọi route
  phiên bản đều phẳng (`/stage-versions/{vid}`, `/course-versions/{vid}`), không lồng dưới `/stages/{id}` hay
  `/courses/{id}`. <!-- Red Team: RT-07 - admin contract aligned with plan.md §7 -->
- MSW fixtures: nạp golden JSON do Phase 5/6 commit tại `apps/api/internal/features/{stages,courses,media}/testdata/*.json`;
  dashboard nạp `apps/api/internal/features/reports/testdata/dashboard.json` do Phase 9 commit (cùng dữ liệu seed.js)
  (dữ liệu từ `prototype/seed.js`: chặng DB/DS/GO/RE/WEB mỗi chặng v1 published, khóa học `BASIC` "Lập trình cơ bản"
  v1 published, lớp `basic01` active, `basic02` active, `basic03` draft); không bịa fixture riêng, trạng thái cần thêm
  (bản nháp v2, khóa học thứ hai) lấy từ golden JSON mà Phase 5/6 tạo qua `testdb`. <!-- Red Team: RT-03 - fixtures from seed.js --> <!-- Red Team: RT-07 - golden JSON -->
- Mã lỗi cần xử lý: `VALIDATION_FAILED`, `CONFLICT` (mã trùng), `VERSION_IMMUTABLE`, `DRAFT_EXISTS`, `IN_USE`,
  `INVALID_TRANSITION`, `NOT_FOUND`.
- Quy ước chung: mỗi mutation có `onSuccess` → `toast.success(text)` + `invalidateQueries` key liên quan; lỗi hiển thị
  trong `Alert danger` của modal nếu modal đang mở, ngược lại `toast.error(message)`. Nút gradient duy nhất mỗi trang
  là hành động chính ghi trong bảng (các nút khác dùng `default`/`outline`/`ghost`).

## Design system / Architecture

- `features/dashboard/{api/dashboard-api.ts, model/schemas.ts, hooks/use-dashboard.ts, pages/dashboard-page.tsx,
  components/{kpi-grid,outdated-alerts,class-list,audit-list}.tsx, routes.tsx, index.ts}`.
- `features/stages/{api/stages-api.ts, model/{schemas.ts, select-version.ts, lesson-form.ts},
  hooks/{use-stages,use-stage,use-stage-version,use-stage-mutations,use-lesson-mutations,use-video-upload}.ts,
  components/{stage-table,version-card,lesson-list,lesson-form-dialog,stage-actions-bar,outdated-courses-card,used-by-card,apply-dialog,new-stage-dialog}.tsx,
  pages/{stages-page,stage-detail-page}.tsx, routes.tsx, index.ts}`.
- `features/courses/{api/courses-api.ts, model/{schemas.ts, select-version.ts, publish-blocker.ts},
  hooks/{use-courses,use-course,use-course-version,use-course-mutations,use-course-stage-mutations}.ts,
  components/{course-table,course-version-card,course-stage-list,add-stage-dialog,course-actions-bar,classes-of-version-card,new-course-dialog}.tsx,
  pages/{courses-page,course-detail-page}.tsx, routes.tsx, index.ts}`.
- `model/` thuần TS: zod schema cho response; `selectVersion(versions, vParam)` = `?v` hợp lệ → bản đó, ngược lại bản
  published mới nhất, ngược lại bản cuối; `coursePublishBlocker(version)` → `null` | "Khóa học cần ít nhất một chặng."
  | "Chặng chưa phát hành: {stage vN, …}."; `lessonFormSchema` (zod discriminated union theo `type`), `STATUS_VI`,
  `canTransition` import từ `@/shared/domain`.
- Query keys: `['dashboard']`, `['stages']`, `['stage', id]`, `['stage-version', vid]`, `['courses']`, `['course',
  id]`, `['course-version', vid]`. Clone/publish/archive/delete phiên bản chặng invalidates `['stage', id]`,
  `['stages']`, `['dashboard']`; apply invalidates thêm `['courses']`, `['course', courseId]`.
- Pattern: Container (page gọi hooks) / Presentational (components nhận props); `VersionPill`, `Lineage`, `PageHead`,
  `Crumbs`, `EmptyState`, `TableWrap`, `ConfirmDialog`, `FormDialog`, `TypeIcon`, `ProgressBar`, `IconButton` lấy từ
  `@/shared/ui`. Lý do disabled đặt vào `title` và `aria-disabled` của nút (giữ focus được), click vào nút
  `aria-disabled` chỉ `toast.error(title)`.

## Pages & components

### `/admin` — DashboardPage (title "Tổng quan")

- `PageHead` h1 "Tổng quan", lede "Tình trạng các lớp đang chạy và việc cần xử lý."
- Khối cảnh báo `stack-sm` (từ `dashboard.outdated[]` và `dashboard.hints.failedInvites`): `Alert warn` mỗi dòng
  "**{courseName} v{courseVersionNo}** vẫn dùng {stageName} v{usingVersionNo} trong khi v{latestVersionNo} đã phát
  hành." + `AlertActions` `Button outline sm` "Xem và áp dụng" → `/admin/stages/{stageId}?v={latestVersionId}`; nếu
  `hints.failedInvites > 0` `Alert danger` "**{n} lời mời gửi thất bại.** Kiểm tra email học viên rồi gửi lại trong
  trang lớp." Không có cảnh báo → không render khối. <!-- Red Team: RT-07 - dashboard field names -->
- `grid-4` KpiCard: "Lớp đang chạy" value `kpis.classes` hint "{hints.draftClasses} lớp nháp"; "Học viên đang học"
  value `kpis.students` hint "{hints.notLoggedIn} chưa đăng nhập lần đầu"; "Lời mời thất bại" value
  `hints.failedInvites` hint "Cần gửi lại"; "Khóa học dùng chặng cũ" value `hints.outdatedCourses` hint "Có thể áp dụng
  một thao tác" (hoặc "Mọi khóa học đã cập nhật" khi 0). <!-- Red Team: RT-07 - kpis/hints mapping -->
- `grid-2`: Card "Lớp học" head + `Button ghost sm` "Tất cả lớp" → `/admin/classes`; `ul.list` mỗi `classes[]`:
  `a.title` "{code} · {name}" → `/admin/classes/{id}`, meta "{courseName} v{courseVersionNo}" + "{memberCount} học
  viên", `Badge` trạng thái lớp (`STATUS_VI`), `ProgressBar` 120px từ `avgPercent` nếu không phải nháp. Card "Nhật ký
  thao tác" `ul.list.audit` 8 mục mới nhất của `recentActivity[]`: title `actionLabel` (nhãn Việt do API dịch từ
  `audit_logs.action`, ví dụ "Tạo chặng", "Phát hành chặng", "Áp dụng chặng cho khóa học", "Xóa phiên bản chặng"),
  meta "{target.label} · {actorName}", `.when` `rel(at)`. Rỗng: EmptyState "Chưa có lớp" / "Chưa có thao tác".
  <!-- Red Team: RT-07 - recentActivity shape -->
- Loading: Skeleton 4 KPI + 2 card; lỗi query → `Alert danger` "Không tải được tổng quan." + nút "Thử lại"
  (`refetch`).

### `/admin/stages` — StagesPage (title "Chặng")

- `PageHead` h1 "Chặng", lede "Chặng dùng chung giữa các khóa học. Sửa học liệu bằng cách nhân bản ra bản nháp mới;
  phiên bản đã phát hành không đổi.", action `Button gradient` icon `Plus` "Tạo chặng".
- `TableWrap` cột: Chặng (`cell-2`: `a.primary` tên → detail, `.secondary` mã), Phiên bản (`Lineage` link `?v={vid}`),
  Khóa học dùng (`num`), Cảnh báo (`Badge warn` "{n} khóa học dùng bản cũ" hoặc "—"). Hàng `is-clickable` → detail
  (click vào ô không phải link). Rỗng: EmptyState "Chưa có chặng" / "Tạo chặng đầu tiên để bắt đầu soạn học liệu." +
  nút "Tạo chặng".
- NewStageDialog (`FormDialog` title "Tạo chặng", confirm "Tạo chặng", cancel "Hủy"): `form-grid` "Mã chặng"
  (`required`, placeholder "vd: DOCKER", help "Duy nhất, dùng trong báo cáo.", uppercase trim), "Tên chặng"
  (`required`, "vd: Docker cơ bản"); ghi chú "Hệ thống tạo kèm phiên bản nháp v1 để bạn thêm học liệu." `POST /stages
  {code,name}` → toast "Đã tạo chặng với bản nháp v1." → navigate `/admin/stages/{id}`. Lỗi client "Nhập mã và tên
  chặng."; `CONFLICT` → lỗi field "Mã chặng đã tồn tại."

### `/admin/stages/:stageId` — StageDetailPage (title "{stage} v{n}")

- `Crumbs` [Chặng → `/admin/stages`, tên]; `PageHead` h1 tên + `Badge muted` mã; lede "Phiên bản đã phát hành là bất
  biến; mọi thay đổi đi qua một bản nháp mới rồi được áp dụng cho khóa học bằng một thao tác."
- VersionCard `row-between`: `section-label` "Phiên bản" + `Lineage` (bản chọn `aria-current="true"`, `?v=` qua
  `useSearchParams`); bên phải `small muted`: published "Phát hành {fmtDateTime}" / draft "Nhân bản từ v{n}" hoặc "Bản
  nháp đầu tiên" / archived "Đã lưu trữ". `?v` không hợp lệ → `selectVersion` fallback, không lỗi.
- LessonList card: head h2 "Học liệu của v{n}" + `Badge` trạng thái; phải: draft → `Button outline sm` "Thêm học
  liệu"; khác → `span.muted small` icon `Lock` "Không sửa được". Item: `.ord` i+1, `TypeIcon`, `.title`, meta video
  "Video · {videoFileName} · {mm:ss từ durationSeconds}" / markdown "Markdown · {markdownSource.length} ký tự",
  `Badge subtle` "Không bắt buộc" khi <!-- Red Team: RT-07 - LessonDTO fields -->
  `!required`, `muted` "key {lessonKey}"; draft có `inline-actions` IconButton "Chuyển lên"/"Chuyển xuống" (disabled ở
  đầu/cuối, `title` lý do "Đã ở đầu danh sách"/"Đã ở cuối danh sách"), "Sửa", "Xóa". Rỗng: "Chưa có học liệu" + draft
  "Thêm video hoặc bài markdown để phát hành chặng này." / khác "Phiên bản này không có học liệu."
- `complete-bar`: trái "{n} bắt buộc · {m} tùy chọn"; phải theo trạng thái:
  - draft: `Button outline` "Xóa bản nháp", `Button gradient` "Phát hành v{n}" (`aria-disabled` + `title="Cần ít nhất
    một học liệu"` khi rỗng).
  - published: `Button outline` "Lưu trữ", `Button outline` "Xóa" (`aria-disabled` title "Đang được dùng trong
    {courseName vN, …}. Hãy lưu trữ thay vì xóa." khi `usedBy.length`), `Button default` "Mở bản nháp v{n}" (link, nếu
    đã có draft) hoặc "Nhân bản thành bản nháp". <!-- Red Team: RT-03 - prototype delete copy -->
  - archived: "Xóa" (cùng điều kiện) + "Mở bản nháp"/"Nhân bản thành bản nháp".
- OutdatedCoursesCard (chỉ khi bản đang xem là published mới nhất; dữ liệu `stage.outdatedCourses[]` từ `GET
  /stages/{id}`): Card "Khóa học đang dùng phiên bản cũ của chặng này" `ul.list`: title link "{courseName}
  v{courseVersionNo}" → `/admin/courses/{courseId}?v={courseVersionId}`, meta "Đang dùng {stageName} v{usingVersionNo}"
  + `Badge warn` lý do chặn (nếu `blockedReason`), phải `Button default sm` "Áp dụng v{latestVersionNo}" (không dùng
  gradient vì nút chính của trang đã dùng; `aria-disabled = !canApply` + `title` = `blockedReason`, ví dụ "Khóa học
  đang có bản nháp v{draftVersionNo}. Phát hành hoặc xóa bản nháp trước."). Nếu không khóa học nào cũ nhưng có khóa học
  dùng chặng → `Alert ok` "Mọi khóa học đã phát hành đều dùng phiên bản mới nhất của chặng này."
  <!-- Red Team: RT-07 - outdatedCourses fields -->
- UsedByCard "Đang được dùng ở" (từ `stageVersion.usedBy[]`): list "{courseName} v{versionNo}" + `Badge` trạng thái +
  `classCodes.join(', ')` hoặc "chưa có lớp"; rỗng "Chưa có khóa học nào dùng v{n}." <!-- Red Team: RT-07 - usedBy.classCodes -->
- Hành động và lời thoại (ConfirmDialog cancel luôn "Hủy"):
  - Nhân bản: `POST .../clone` → toast "Đã tạo bản nháp mới. Học liệu được sao chép, giữ nguyên lesson_key." →
    `?v={draftId}`. `DRAFT_EXISTS` → toast lỗi "Chặng đã có bản nháp v{n}. Mở bản nháp đó để sửa." (dùng
    `details.draftVersionNo`).
  - Phát hành: title "Phát hành {stage} v{n}?", text "Sau khi phát hành, phiên bản này và {k} học liệu của nó không
    sửa được nữa. Khóa học đang dùng phiên bản cũ sẽ không tự cập nhật; bạn áp dụng ở bước tiếp theo.", confirm "Phát
    hành" → `POST .../publish` → toast "Đã phát hành v{n}." `VALIDATION_FAILED` (0 học liệu) → "Cần ít nhất một học
    liệu."
  - Lưu trữ: "Lưu trữ v{n}?" / "Phiên bản lưu trữ không gắn mới vào khóa học được, nhưng các khóa học và lớp đang dùng
    vẫn hoạt động bình thường." / confirm "Lưu trữ" → toast "Đã lưu trữ."
  - Xóa phiên bản: nếu `usedBy.length` → toast lỗi "Đang được dùng trong {courseName vN, …}. Hãy lưu trữ thay vì
    xóa." không mở dialog (app.js:155); ngược lại dialog danger "Xóa {stage} v{n}?" text draft "Bản nháp và {k} học
    liệu trong đó sẽ bị xóa. Không hoàn tác được." / khác "Phiên bản này không được khóa học nào tham chiếu. Xóa sẽ
    không hoàn tác được."; confirm "Xóa phiên bản" → `DELETE /stage-versions/{vid}` → toast "Đã xóa." → response
    `stageDeleted:false` → navigate detail (bỏ `?v`), `stageDeleted:true` (chặng đã bị xóa cùng phiên bản cuối,
    app.js:157-163) → `/admin/stages` + invalidate `['stages']`. `IN_USE` → toast lỗi message server.
    <!-- Red Team: RT-07 - delete last version deletes stage --> <!-- Red Team: RT-03 - prototype copy -->
  - Thêm/Sửa học liệu: LessonFormDialog title "Thêm học liệu"/"Sửa học liệu", confirm "Thêm"/"Lưu": "Tiêu đề"
    (`required`), `form-grid` "Loại" Select Video/Markdown (khóa khi sửa) + Checkbox "Bắt buộc (tính vào %)" mặc định
    bật; Video: "File video" (input file `accept="video/mp4"`, hiện tên + dung lượng + `Progress` % upload, nút "Chọn
    file khác"), "Thời lượng" (mm:ss, tự điền từ `loadedmetadata` của file local nếu đọc được); Markdown: Textarea
    rows 10 "Nội dung Markdown" help "Hỗ trợ tiêu đề, danh sách, khối mã, trích dẫn, **đậm** và `mã`. HTML được lọc
    khi hiển thị." Lỗi client: "Nhập tiêu đề học liệu.", "Học liệu video bắt buộc có file video.", "Học liệu markdown
    bắt buộc có nội dung." Luồng video: `POST /media/uploads {kind:'video', fileName, contentType, sizeBytes}` (qua
    `http.ts`, tự gắn `X-Requested-With: fetch`) → `{mediaId, uploadUrl, expiresAt}` → `PUT uploadUrl` bằng
    `XMLHttpRequest` thô (để có `progress`) với **chỉ** header `Content-Type` = file type, không `withCredentials`,
    không `X-Requested-With` (đích là MinIO/R2, không phải API) → `POST /media/uploads/{mediaId}/complete` (qua
    `http.ts`) → rồi `POST/PATCH lesson {title, type:'video', required, videoMediaId, durationSeconds}`; markdown gửi
    `{title, type:'markdown', required, markdownSource}`. Nút submit disabled trong khi upload ("Đang tải lên {pct}%").
    Thành công <!-- Red Team: RT-05 - X-Requested-With only via http.ts --> <!-- Red Team: RT-07 - upload and lesson body -->
    toast "Đã thêm học liệu." / "Đã lưu học liệu." `VERSION_IMMUTABLE` → Alert "Phiên bản đã phát hành, không sửa
    được." và `invalidate`.
  - Xóa học liệu: dialog danger "Xóa \"{title}\"?" text "Học liệu bị xóa khỏi bản nháp này. Các phiên bản đã phát hành
    không bị ảnh hưởng." confirm "Xóa học liệu" → toast "Đã xóa học liệu."
  - Sắp xếp: `PUT .../lessons/order {lessonIds}` với optimistic update trên `['stage-version', vid]`, lỗi → rollback +
    toast "Không đổi được thứ tự."
  - Áp dụng (FR-18) ApplyDialog: title "Áp dụng {stage} v{n} cho {course}", confirm "Tạo và phát hành {course}
    v{latest+1}", body "Trong một giao dịch, hệ thống sẽ:" + `ol`: "Nhân bản **{course} v{latest}** thành
    **v{latest+1}**.", "Thay **{stage} v{used}** bằng **v{n}**, giữ nguyên thứ tự và {k−1} chặng còn lại.", "Phát hành
    v{latest+1}." + `Alert info` "Các lớp đang chạy trên v{latest} ({codes} | chưa có) không bị ảnh hưởng. Lớp mới
    hoặc lớp nháp mới gắn được v{latest+1}." → `POST /stage-versions/{vid}/apply {courseIds:[courseId]}` → đọc
    `results[]` theo từng dòng: mọi dòng không có `error` → toast "Đã phát hành {courseName} v{newVersionNo} dùng
    v{n}."; dòng có `error` → Alert danger trong dialog "{courseCode}: {error.message}" (HTTP vẫn 200, không coi là
    thành công). Lỗi toàn cục 422 "Phiên bản chặng chưa phát hành." → Alert trong dialog.
    <!-- Red Team: RT-07 - per-row apply results -->

### `/admin/courses` — CoursesPage (title "Khóa học")

- `PageHead` h1 "Khóa học", lede "Khóa học là một chuỗi phiên bản chặng có thứ tự. Lớp gắn với đúng một phiên bản khóa
  học đã phát hành.", `Button gradient` "Tạo khóa học".
- Bảng (từ `GET /courses` items): Khóa học (`cell-2` tên + mã), Phiên bản (`Lineage` từ `versions[]`), Số chặng
  (`num`, `stageCount` của bản published mới nhất hoặc "—"; `Badge warn` "{outdatedStageCount} chặng cũ" khi > 0),
  Lớp đang dùng (`classesUsing.map(c => "{c.code} (v{c.versionNo})").join(', ')` hoặc "—"). Rỗng "Chưa có khóa học" /
  "Tạo khóa học rồi ghép các chặng đã phát hành." <!-- Red Team: RT-07 - courses list fields -->
- NewCourseDialog "Tạo khóa học" confirm "Tạo khóa học": "Mã khóa học" ("vd: ADV"), "Tên khóa học" ("vd: Lập trình
  nâng cao"), ghi chú "Hệ thống tạo kèm phiên bản nháp v1 để bạn ghép chặng." → `POST /courses` → toast "Đã tạo khóa
  học với bản nháp v1." → detail. `CONFLICT` → "Mã khóa học đã tồn tại."

### `/admin/courses/:courseId` — CourseDetailPage (title "{course} v{n}")

- `Crumbs` [Khóa học, tên]; `Badge muted` mã; lede "Nhân bản khóa học là nhân bản nông: phiên bản mới trỏ lại đúng các
  phiên bản chặng cũ cho đến khi bạn đổi."
- VersionCard như chặng. Nếu draft và `coursePublishBlocker` ≠ null → `Alert warn` "Chưa phát hành được: {blocker}".
- CourseStageList card head "Chặng trong v{n}" + Badge; draft → `Button outline sm` "Thêm chặng", khác → lock "Không
  sửa được". Item (`stages[]` của `GET /course-versions/{vid}`): `.ord`, `a.title` `stageName` →
  `/admin/stages/{stageId}?v={stageVersionId}`, meta `VersionPill size=sm` v{stageVersionNo} (22px, 11px) +
  "{lessonCount} học liệu · {requiredCount} bắt buộc" + `Badge warn` "v{latestPublishedNo} đã phát hành" nếu
  `outdated`; draft `inline-actions`: `Button ghost sm` "Dùng v{latestPublishedNo}" (khi `outdated`), IconButton
  lên/xuống, IconButton `X` "Gỡ khỏi khóa học". Rỗng "Chưa có chặng" + draft "Thêm các chặng đã phát hành theo thứ tự
  học." / "Phiên bản này không có chặng." <!-- Red Team: RT-07 - course stage row fields -->
- `complete-bar`: "{n} học liệu bắt buộc toàn khóa"; draft: `outline` "Xóa bản nháp", `gradient` "Phát hành v{n}"
  (`aria-disabled` title = blocker); published/archived: `outline` "Lưu trữ" (chỉ published), `outline` "Xóa"
  (`aria-disabled` title "Đang được dùng bởi lớp {codes}. Hãy lưu trữ thay vì xóa." khi `classes.length`), `default`
  "Mở bản nháp v{n}" / "Nhân bản thành bản nháp". <!-- Red Team: RT-03 - prototype delete copy -->
- ClassesOfVersionCard "Lớp gắn với v{n}" (từ `classes[]`): list "{code} · {name}" → `/admin/classes/{classId}`, meta
  "{memberCount} học viên", Badge trạng thái lớp; rỗng "Chưa có lớp nào gắn với v{n}." <!-- Red Team: RT-07 -->
- Hành động: clone → toast "Đã tạo bản nháp mới (nhân bản nông)." → `?v=draft`; publish (blocker → chỉ toast lỗi
  blocker) dialog "Phát hành {course} v{n}?" text "Sau khi phát hành, danh sách và thứ tự {k} chặng không sửa được.
  Lớp mới có thể gắn với phiên bản này." confirm "Phát hành" → toast "Đã phát hành v{n}."; archive "Lưu trữ v{n}?"
  text "Lớp mới không gắn được phiên bản lưu trữ; các lớp đang dùng vẫn hoạt động bình thường." → "Đã lưu trữ.";
  delete: có lớp → toast lỗi "Đang được dùng bởi lớp {codes}. Hãy lưu trữ thay vì xóa." không mở dialog; dialog
  danger "Xóa {course} v{n}?" text "Không hoàn tác được. Các phiên bản chặng không bị ảnh hưởng." confirm "Xóa phiên
  bản" → `DELETE /course-versions/{vid}` → "Đã xóa." → `courseDeleted:true` → `/admin/courses`, ngược lại detail bỏ
  `?v`. <!-- Red Team: RT-03 - prototype copy --> <!-- Red Team: RT-07 - delete last version deletes course -->
- AddStageDialog: options = phiên bản published của chặng chưa có trong bản nháp (lấy từ `GET /stages`), label
  "{stage} v{n}" + " (mới nhất)" cho bản cao nhất; không còn option → toast lỗi "Không còn chặng đã phát hành nào để
  thêm." không mở dialog; dialog "Thêm chặng vào khóa học" confirm "Thêm", Select "Phiên bản chặng" help "Chỉ liệt kê
  phiên bản đã phát hành của các chặng chưa có trong khóa học." → `PUT /course-versions/{vid}/stages
  {stageVersionIds:[...cũ, mới]}` → toast "Đã thêm chặng." Swap "Dùng v{newer}" → PUT thay id → "Đã đổi sang phiên bản
  mới."; gỡ → PUT bớt id → "Đã gỡ chặng khỏi bản nháp."; lên/xuống → PUT thứ tự mới (optimistic). `VERSION_IMMUTABLE`
  → toast "Phiên bản đã phát hành, không sửa được."

## Files to Create / Modify

- `src/features/dashboard/**` (api, model, hooks, components, pages, `routes.tsx`, `index.ts`).
- `src/features/stages/**` theo cây ở Architecture.
- `src/features/courses/**` theo cây ở Architecture.
- Test: `src/features/*/model/*.test.ts`, `src/features/*/pages/*.test.tsx` (MSW handlers đặt trong
  `src/features/<f>/api/msw-handlers.ts`, không sửa `shared/test`).
- Ghi đè ba file `routes.tsx` stub của phase 10.

## Tasks & Steps

1. Viết zod schemas + api cho `dashboard`, `stages`, `courses` đúng tên trường ở Context; MSW handlers theo plan.md
   §7, fixture nạp từ golden JSON `testdata/*.json` của Phase 5/6 (dữ liệu `prototype/seed.js`: BASIC, basic01…),
   không tự đặt tên khác. <!-- Red Team: RT-03 --> <!-- Red Team: RT-07 -->
2. `model/`: `selectVersion`, `coursePublishBlocker`, `lessonFormSchema`, hàm tính meta ("{n} bắt buộc · {m} tùy
   chọn", "{k} học liệu bắt buộc toàn khóa"); unit test.
3. DashboardPage: KPI, alerts, hai card, skeleton/lỗi; `routes.tsx` `{ index: true, lazy, handle:{title:'Tổng quan'}
   }`.
4. StagesPage + NewStageDialog; StageDetailPage: VersionCard, LessonList, actions bar, Outdated/UsedBy cards.
5. LessonFormDialog + `useVideoUpload` (XHR thô chỉ cho bước PUT lên `uploadUrl`, không header ngoài `Content-Type`;
   init/complete đi qua `http.ts`; hủy khi đóng dialog bằng `xhr.abort()`), reorder optimistic. <!-- Red Team: RT-05 -->
6. ApplyDialog (FR-18) đọc `results[]` từng dòng + cập nhật cache `['course', id]`, `['courses']`, `['dashboard']`.
   <!-- Red Team: RT-07 -->
7. CoursesPage, CourseDetailPage, AddStageDialog, swap/remove/move, actions bar, ClassesOfVersionCard.
8. Test component chính: StageDetailPage (draft vs published hiển thị nút đúng, publish 0 học liệu bị chặn, delete khi
   `usedBy` chỉ toast, `stageDeleted:true` điều hướng về danh sách), ApplyDialog (text đúng; mọi dòng ok → toast; một
   dòng `error` → Alert, không toast), CourseDetailPage (blocker alert, add-stage không còn option); `useVideoUpload`
   (XHR PUT không có `X-Requested-With`, request `complete` có). <!-- Red Team: RT-05 --> <!-- Red Team: RT-07 -->
9. Lint/typecheck/build; kiểm tra thủ công với API phase 5/6/9 chạy thật; đối chiếu từng màn với
   `prototype/index.html#/admin/...`. <!-- Red Team: RT-07 -->

## Verification

- Vitest xanh cho model và page tests; `pnpm lint typecheck build` xanh; không import `@/features/*/*` sâu, không chạm
  `src/app|shared|styles` (`git diff --stat` chỉ có `src/features/{dashboard,stages,courses}`).
- Kịch bản thủ công (API thật): tạo chặng → thêm 1 video (upload thật qua presigned PUT, progress chạy) + 1 markdown →
  phát hành v1 → nhân bản v2 → sửa → phát hành v2 → tạo khóa học → thêm chặng v1 → phát hành → vào chặng v2 thấy "Khóa
  học đang dùng phiên bản cũ" → "Áp dụng v2" → khóa học có v2 published, toast đúng, dashboard hết cảnh báo.
- Lỗi: xóa phiên bản đang dùng → toast lỗi, không gọi DELETE; publish khóa học có chặng chưa phát hành → toast
  blocker; sửa học liệu bản published (gõ URL cũ) → `VERSION_IMMUTABLE` hiển thị đúng.
- a11y: dialog `aria-labelledby`, focus trả về nút mở; IconButton có `aria-label`; hàng bảng clickable vẫn có link
  thật trong ô; `Lineage` `aria-current`; 320px bảng cuộn trong `TableWrap`; axe 0 lỗi nghiêm trọng.

## Security notes

- Upload: trình duyệt PUT thẳng lên `uploadUrl` presigned (ngắn hạn) do API cấp; không gửi credential kèm
  (`XMLHttpRequest` không `withCredentials`) và không gửi `X-Requested-With` (header đó chỉ `http.ts` thêm cho
  request tới API, Phase 10; thêm vào PUT sẽ làm MinIO/R2 từ chối preflight hoặc sai chữ ký); chỉ chấp nhận
  `video/mp4`, giới hạn dung lượng theo `details.maxSize` từ API hoặc 2 GB mặc định, báo lỗi "File vượt dung lượng cho
  phép." <!-- Red Team: RT-05 -->
- Nội dung Markdown chỉ xem trước qua `MarkdownContent` (DOMPurify) của phase 10; không `dangerouslySetInnerHTML` ở
  nơi khác.
- Mọi điều kiện disabled ở UI chỉ là tiện ích; server mới là nơi chặn (`IN_USE`, `VERSION_IMMUTABLE`, `DRAFT_EXISTS`),
  UI phải hiển thị lỗi server thay vì giả định thành công.
- Không hiện `lessonKey`/`mediaId` ở nơi không cần ngoài meta (prototype có "key {lessonKey}", giữ vì admin cần đối
  soát báo cáo).

## Risks & Rollback

- **Response shape phase 5/6/9 lệch bảng §7**: MSW dùng golden JSON của chính các phase đó nên lệch sẽ lộ ở test
  Phase 11 trước khi chạy tay; zod parse fail sớm với lỗi rõ; sửa schema tại `api/` không lan sang components.
  Rollback: giữ schema `passthrough()` tạm. <!-- Red Team: RT-07 -->
- **Presigned PUT bị CORS**: dev đã có `MINIO_API_CORS_ALLOW_ORIGIN=http://localhost:5173` trong compose Phase 1; prod
  cấu hình CORS bucket R2 trên Cloudflare dashboard (runbook Phase 14). Không có proxy upload qua API; nếu PUT lỗi CORS
  thì sửa cấu hình storage, không đổi luồng. <!-- Red Team: RT-08 - CORS via env, no proxy fallback --> <!-- Updated: Validation Session 1 - R2 prod -->
- **Optimistic reorder lệch server** khi hai tab cùng sửa: rollback bằng `onError` + `invalidate`; chấp nhận cho MVP.
- **Thiếu component dùng chung** (ví dụ `OrderButtons`): viết trong feature, ghi Concerns; không sửa `shared` trong
  phase này.
- Rollback phase: revert commit trong `src/features/{dashboard,stages,courses}`; ba `routes.tsx` về stub rỗng, app vẫn
  build.

## Success Criteria

- Năm trang admin (`/admin`, `/admin/stages`, `/admin/stages/:id`, `/admin/courses`, `/admin/courses/:id`) khớp 1:1
  prototype về bố cục, lời thoại, toast, trạng thái rỗng/disabled và lý do disabled.
- Luồng FR-18 chạy end-to-end bằng một thao tác với toast đúng số phiên bản.
- Upload video có tiến độ, hủy được, lỗi rõ; học liệu markdown lưu và xem trước an toàn.
- Mọi mã lỗi `CONFLICT`, `VERSION_IMMUTABLE`, `DRAFT_EXISTS`, `IN_USE`, `INVALID_TRANSITION` có thông điệp Việt hóa
  đúng chỗ (field, alert dialog hoặc toast).
- Không file nào ngoài `src/features/{dashboard,stages,courses}` bị sửa; lint, typecheck, test, build xanh.
