---
phase: 13
title: "Phase 13: Web học viên: lớp, lộ trình, học liệu"
status: pending
priority: P1
effort: "2 ngày"
dependencies: [10, 8]
---

# Phase 13: Web học viên: lớp, lộ trình, học liệu

## Goal

Hoàn thành khu vực học viên trên web: danh sách lớp (`/learn`), lộ trình một lớp theo chặng
(`/learn/classes/:classId`)
và trang học liệu (`/learn/classes/:classId/lessons/:lessonId`) với trình phát video qua URL ký ngắn hạn, bài đọc
Markdown
đã lọc HTML, ghi nhận "mở lần đầu" (FR-31) và tự xác nhận hoàn thành (tích / bỏ tích) với mọi điều kiện chặn hiển thị
lý do.
Lời thoại bám `prototype/app.js` (`pageLearn`, `pageLearnClass`, `pageLesson`).

## Context & Requirements

- Phụ thuộc: phase 10 (shell, `shared/ui`, `MarkdownContent`, `http`, router, hợp đồng `routes.tsx`), phase 8 (API
  learning + media URL; golden JSON `apps/api/internal/features/learning/testdata/*.json` dùng làm fixture MSW).
- **Quy tắc sở hữu file:** chỉ thêm/sửa dưới `src/features/learning/**`. Không sửa `src/app`, `src/shared`,
  `src/styles`.
  `features/learning/routes.tsx` export `routes` tương đối dưới `/learn`: `''`, `'classes/:classId'`,
  `'classes/:classId/lessons/:lessonId'`, kèm `handle.title`.
<!-- Red Team: RT-06 - DTO học viên khớp từng trường với plan.md §7 và Phase 08; không còn done/total/pct, canTick, nextLessonId -->
- API (base `/api/v1`), hợp đồng canonical ở `plan.md` §7 (Phase 08 cung cấp, Zod schema ở `model/schemas.ts` phải khớp
  từng trường):
  - `GET /me/classes` → `{items:[{id, code, name, status:'draft'|'active'|'ended', startDate, endDate, teacherName,
    courseName, courseVersionNo, percent, requiredDone, requiredTotal, nextLesson?:{lessonId,title,stageName},
    readOnlyReason?:'draft'|'ended'}]}`. Lớp `draft` có trong danh sách; thành viên `dropped` không được trả.
  - `GET /me/classes/{id}` → `{class:{id,code,name,status,startDate,endDate,teacherName,courseName,courseVersionNo},
    percent, requiredDone, requiredTotal, readOnly, readOnlyReason?, nextLesson?, stages:[{id,name,position,
    lessons:[{id,title,type:'markdown'|'video',required,position,durationSeconds?,firstOpenedAt?,completedAt?}]}]}`.
    Lớp `draft` → `200` với `readOnly=true`, `readOnlyReason='draft'` (lộ trình chỉ đọc, không có nhánh FORBIDDEN).
  - `GET /me/classes/{id}/lessons/{lid}` → `{lesson:{id,title,type,required,position,durationSeconds?,stage:{id,name}},
    content:{type:'markdown',html}|{type:'video',mediaId,url,expiresAt}, progress:{firstOpenedAt,completedAt?},
    readOnly, prev?:{lessonId,title}, next?:{lessonId,title}}`; ghi `first_opened_at` nếu chưa có (FR-31). Video đã
    kèm URL ký lần đầu; ký lại qua `GET /media/{mediaId}/url` → `{url, expiresAt}`.
  - `PUT /me/classes/{id}/lessons/{lid}/completion {completed: boolean}` → `{completedAt?:string|null, percent,
    requiredDone, requiredTotal}` (dùng để cập nhật cache lớp không cần refetch).
  - Ảnh trong markdown dùng `/api/v1/media/{id}/content` (302).
- Mã lỗi (chỉ ba mã, không có `FORBIDDEN`/`VALIDATION_FAILED` cho luồng học): `404 NOT_FOUND` (không phải thành viên,
  đã `dropped`, lớp không tồn tại, hoặc học liệu không thuộc phiên bản khóa học của lớp); `409 CONFLICT` "Mở học liệu
  trước khi tích hoàn thành."; `409 INVALID_TRANSITION` "Lớp chưa bắt đầu hoặc đã kết thúc; không ghi nhận tiến độ."
  (FE chọn câu theo `readOnlyReason`/trạng thái lớp đã có trong cache).
<!-- Red Team: RT-05 - mọi mutation qua http.ts để luôn gửi X-Requested-With: fetch -->
- Mọi request (kể cả `PUT …/completion`) đi qua `@/shared/http` của phase 10, vốn gắn `X-Requested-With: fetch` và
  `credentials: 'include'`; không gọi `fetch` trực tiếp trong feature này.
- Quy tắc tiến độ: chỉ học liệu `required` tính vào %; học liệu không bắt buộc vẫn tích được nhưng không đổi %.
- Một nút `gradient` mỗi trang: `/learn` dùng cho "Học tiếp" của thẻ đầu tiên đang chạy, các thẻ còn lại `default`;
  trang học liệu dùng cho "Đã học xong".

## Design system / Architecture

- `features/learning/`
  - `api/learning-api.ts`, `api/media-api.ts`, `api/msw-handlers.ts`
  - `model/{schemas.ts, progress.ts, tick-rule.ts, neighbors.ts}`
  - `hooks/{use-my-classes, use-my-class, use-lesson, use-toggle-completion, use-video-url}.ts`
  - `components/{class-card, roadmap, stage-section, lesson-row, tick-checkbox, lesson-viewer,
    video-player, markdown-lesson, lesson-nav, lesson-side-list, complete-bar}.tsx`
  - `pages/{learn-page, learn-class-page, lesson-page}.tsx`, `routes.tsx`, `index.ts`
- `model/tick-rule.ts` (thuần TS): `tickBlocker({readOnlyReason, opened})` →
  `null` | `"Lớp đã kết thúc"` (`readOnlyReason==='ended'`) | `"Lớp chưa bắt đầu"` (`'draft'`) | `"Mở học liệu trước khi
  tích"`. Thứ tự ưu tiên: trạng thái lớp → đã mở. Không có nhánh "không còn là thành viên": API trả `404` và trang điều
  hướng về `/learn`.
- `model/progress.ts`: `stageProgress(stage)` tính từ `lessons[]` (chỉ `required`) bằng `percentOf` của
  `@/shared/domain` (cùng quy tắc làm tròn với `domain.Percent` ở API, RT-12); `percent`/`requiredDone`/`requiredTotal`
  toàn khóa lấy thẳng từ API, không tính lại;
  `lessonSub(lesson)` → "Video · {mm:ss}" từ `durationSeconds` (ví dụ 1104 → "Video · 18:24") / "Bài đọc" + " · không
  bắt buộc" + " · đã mở" (khi mở mà chưa xong).
- `model/neighbors.ts`: `flattenLessons(stages)` theo thứ tự chặng rồi thứ tự học liệu; `neighbors(flat, lessonId)` →
  `{prev, next}`;
  `firstIncomplete(flat)` cho nút "Học tiếp".
- Strategy renderer: `LessonViewer` chọn `VideoPlayer` hay `MarkdownLesson` theo `lesson.type`; thêm loại mới chỉ cần
  thêm component + nhánh map.
- Query keys: `['my-classes']`, `['my-class', id]`, `['lesson', id, lid]`, `['lesson-video', lid]`.
  `useToggleCompletion` optimistic trên `['my-class', id]` và `['lesson', id, lid]`, rollback khi lỗi, `invalidate` cả
  `['my-classes']`.
- `useVideoUrl(content)`: `initialData = {url, expiresAt}` từ `content` của `GET …/lessons/{lid}` (không gọi thêm
  request lần đầu), `queryFn` = `GET /media/{mediaId}/url`, `staleTime = expiresAt − now − 60s`, `gcTime: 0`,
  `refetchOnWindowFocus: false`; expose `refetch` cho cơ chế tự hồi phục khi URL hết hạn.

## Pages & components

### `/learn` — LearnPage (title "Lớp của tôi")

- `PageHead` h1 "Xin chào, {lastName}" (từ `me.fullName`, lấy từ cuối), lede "Các lớp bạn đang tham gia."
- `grid-2` ClassCard (`Card card-pad`): `row-between` h2 tên lớp + `Badge` trạng thái lớp (`STATUS_VI`);
  `muted small` "{code} · {courseName} · {fmtDate(startDate)} → {fmtDate(endDate)}"; `ProgressBar size=lg
  value={percent}` + "{requiredDone}/{requiredTotal} học liệu bắt buộc".
  Hàng hành động:
  - `readOnlyReason === 'draft'` → `small muted` "Lớp bắt đầu {fmtDate(startDate)}. Bạn sẽ vào học được khi lớp kích
    hoạt." (không nút).
  - active/ended: nếu có `nextLesson` → `Button` "Học tiếp" → `/learn/classes/{id}/lessons/{nextLesson.lessonId}`;
    không có → `Button default` "Xem lộ trình" → `/learn/classes/{id}`; luôn kèm `Button ghost` "Lộ trình" → lớp.
    Thẻ active đầu tiên dùng `gradient`, thẻ còn lại `default`.
- Thành viên `dropped` không có trong `items` (API loại ở tầng SQL, RT-06); FE không cần lọc thêm.
- Rỗng: EmptyState "Chưa có lớp" / "Khi được mời vào lớp, lớp sẽ hiện ở đây." Loading: 2 Skeleton card.
- Lỗi query → `Alert danger` "Không tải được danh sách lớp." + "Thử lại".

### `/learn/classes/:classId` — LearnClassPage (title = mã lớp)

- `404 NOT_FOUND` (không phải / không còn là thành viên đang học, hoặc lớp không tồn tại) →
  `toast.error("Bạn không còn là thành viên đang học của lớp.")` + `navigate('/learn', {replace:true})`.
- `Crumbs` [Lớp của tôi → `/learn`, `class.code`]; h1 `class.name` + `Badge`; lede "{courseName} v{courseVersionNo} ·
  Giảng viên {teacherName}".
- Card `card-pad row-between`: `section-label` "Tiến độ toàn khóa" + `small` "{requiredDone}/{requiredTotal} học liệu
  bắt buộc. Học liệu không bắt buộc không tính vào %."
  + `ProgressBar size=lg value={percent}` (min-width 260).
- Nếu `readOnly` → `Alert info` theo `readOnlyReason`: `ended` "Lớp đã kết thúc: bạn vẫn xem được học liệu nhưng không
  tích hoàn thành được nữa." / `draft` "Lớp chưa bắt đầu." (API luôn trả lộ trình chỉ đọc cho lớp nháp, RT-06; không có
  nhánh fallback FORBIDDEN).
- Roadmap `.roadmap` mỗi chặng `Card.stage`: StageSection head `.ord` i+1, h2 tên chặng, `small` "{k} học liệu ·
  {done}/{total} bắt buộc đã xong", `ProgressBar` 180px.
  LessonRow (`.is-done` khi `completedAt`): `a.lesson-link` (`TypeIcon` + `.t` tiêu đề + `.sub` `lessonSub`) → trang
  học liệu;
  TickCheckbox `label.check` (`.is-disabled` + `title={blocker}`) với `Checkbox aria-label="Đã học xong: {title}"` và
  chữ "Đã học xong";
  `disabled` khi `tickBlocker !== null`.
- Tick/bỏ tích từ lộ trình: `PUT .../completion {completed}` optimistic; thành công: ghi `percent`, `requiredDone`,
  `requiredTotal` từ response vào cache `['my-class', id]` + toast "Đã ghi nhận hoàn thành." / "Đã bỏ tích.";
  lỗi → revert + toast: `409 CONFLICT` → "Mở học liệu trước khi tích hoàn thành."; `409 INVALID_TRANSITION` → "Lớp đã
  kết thúc" hoặc "Lớp chưa bắt đầu" theo `readOnlyReason` trong cache (không có thì dùng `error.message` của API) rồi
  `invalidate ['my-class', id]`; `404 NOT_FOUND` → "Bạn không còn là thành viên đang học của lớp." + navigate `/learn`.
- Rỗng (khóa học không có học liệu, hiếm) → EmptyState "Chưa có học liệu" / "Giảng viên chưa thêm học liệu cho lớp
  này."

### `/learn/classes/:classId/lessons/:lessonId` — LessonPage (title = tiêu đề học liệu)

- Dữ liệu: `useMyClass(classId)` (cho crumbs, side list, trạng thái) + `useLesson(classId, lessonId)`
  (`GET .../lessons/{lid}` ghi `firstOpenedAt`, trả `progress`, `prev`, `next`, `readOnly`; sau thành công
  `setQueryData(['my-class'])` cập nhật `firstOpenedAt` của học liệu để nút tích mở khóa ngay). `prev`/`next` lấy từ
  API; `neighbors()` chỉ dùng để kiểm chéo với cache lớp trong unit test.
- `404 NOT_FOUND` → nếu `useMyClass` đã có dữ liệu (học liệu không thuộc phiên bản khóa học của lớp) →
  `toast.error("Học liệu không thuộc phiên bản khóa học của lớp.")` + navigate lớp; nếu chính lớp cũng `404` → như trang
  lớp (toast "Bạn không còn là thành viên đang học của lớp." + `/learn`).
- `Crumbs` [Lớp của tôi → `/learn`, mã → lớp, tên chặng → lớp (anchor `#stage-{id}`), tiêu đề]; h1 tiêu đề + `Badge
  subtle` "Không bắt buộc" khi `!required`.
- `.viewer` grid `minmax(0,1fr) 320px` (≤960 một cột, aside xuống dưới):
  - Card nội dung: LessonViewer.
    - VideoPlayer: `<video controls preload="metadata" playsInline controlsList="nodownload" className="aspect-video
      w-full bg-navy-deep">` không `crossorigin`,
      `src` từ `useVideoUrl(content)` (URL ký đầu tiên đã có trong `content.url`/`content.expiresAt`, ký lại qua
      `GET /media/{content.mediaId}/url`); đang lấy URL → khung `.video-frame` gradient-navy với Skeleton;
      `onError`: lưu `currentTime` và `!paused`, `refetch()` URL, `onLoadedMetadata` khôi phục `currentTime` và
      `play()` nếu đang chạy; tối đa 2 lần, sau đó `Alert danger` "Không phát được video. Tải lại trang để thử lại."
      thay khung video.
      Đổi tab/quay lại: không refetch tự động; nếu URL hết hạn khi bấm play, luồng `onError` ở trên xử lý.
    - MarkdownLesson: `card-body` + `MarkdownContent html={content.html}` (API trả HTML đã render; DOMPurify lọc lại
      phía client) với class `.md` (prose 72ch, pre nền surface-dark, blockquote viền navy).
  - CompleteBar `.complete-bar`: trái `Button ghost sm` "Bài trước" (disabled + `title="Đây là bài đầu tiên"` khi
    không có) và "Bài tiếp →" (disabled + `title="Đây là bài cuối cùng"`), đi qua toàn bộ học liệu của phiên bản khóa
    học theo `prev`/`next` của API;
    phải: đã xong → `StatusDot ok small` "Đã tích {fmtDateTime(completedAt)}" + `Button outline` "Bỏ tích"; chưa →
    `Button gradient` icon `Check` "Đã học xong"
    (`aria-disabled` + `title` = `tickBlocker`, click khi disabled → `toast.error(title)`). Ở trang này `opened` luôn
    `true` sau khi `useLesson` thành công.
  - Toast: "Đã ghi nhận hoàn thành." / "Đã bỏ tích."; lỗi như trang lớp.
- Aside Card: head h2 "{stage} <span muted>v{n}</span>"; `ul.side-list` các học liệu cùng chặng: link
  `aria-current="page"` cho bài hiện tại,
  icon `CircleCheck` ok khi xong hoặc `.todo-mark` vòng tròn rỗng; `card-pad` `Button ghost sm` "Toàn bộ lộ trình" →
  lớp.
- Phím tắt: không thêm (tránh xung đột với `<video>`); focus sau điều hướng "Bài tiếp" đặt vào h1 qua `RouteAnnouncer`
  của phase 10.

## Files to Create / Modify

- `src/features/learning/**` theo cây Architecture; ghi đè `routes.tsx` stub.
- Test: `model/{tick-rule,progress,neighbors}.test.ts`, `components/video-player.test.tsx` (mock `HTMLMediaElement`),
  `pages/{learn-page,learn-class-page,lesson-page}.test.tsx`.

## Tasks & Steps

<!-- Red Team: RT-14 - fixture MSW là golden JSON Phase 08 sinh từ dữ liệu seed.js -->
1. Zod schemas (khớp DTO ở Context) + `learning-api`, `media-api`, MSW handlers import golden JSON của Phase 08
   (`my-classes.json`, `my-class-basic01.json`, `lesson-video.json`, `lesson-markdown.json`, `completion.json`): lớp
   `basic01` active, chặng Database với "Giới thiệu SQL" (video 18:24, bắt buộc), "Thiết kế bảng và khóa" (markdown,
   bắt buộc), "Đọc thêm: chỉ mục" (markdown, không bắt buộc); lớp `basic03` draft `readOnly`. Schema parse thất bại
   trên golden → test đỏ (phát hiện lệch hợp đồng sớm).
2. `model/`: `tickBlocker`, `stageProgress`, `lessonSub` (1104 → "18:24"), `flattenLessons/neighbors`; unit test đủ
   nhánh, kể cả `percentOf(2,3)=67`, `(1,3)=33`, `(1,2)=50` khớp `domain.Percent` của API.
3. LearnPage + ClassCard (ba trạng thái lớp, gradient chỉ thẻ active đầu).
4. LearnClassPage: head, progress card, Alert theo trạng thái, Roadmap/StageSection/LessonRow/TickCheckbox, optimistic
   toggle + rollback.
5. `useVideoUrl` + VideoPlayer (tự hồi phục URL hết hạn, giới hạn 2 lần), MarkdownLesson.
6. LessonPage: crumbs, viewer, CompleteBar, LessonNav, LessonSideList; cập nhật `firstOpenedAt` vào cache lớp.
7. Test component: LearnClassPage (checkbox disabled + title theo `readOnlyReason` và chưa mở; tick thành công đổi %
   từ response; `409 CONFLICT`/`409 INVALID_TRANSITION` revert + toast; `404` điều hướng `/learn`),
   LessonPage (Bài trước/tiếp disabled ở biên, nút tích disabled khi lớp ended, toast khi click disabled), VideoPlayer
   (error → refetch → khôi phục `currentTime`; lần 3 hiện Alert).
8. Lint/typecheck/build; kiểm thủ công với API phase 8 thật; đối chiếu prototype `#/learn/*`.

## Verification

- Vitest xanh; `pnpm lint typecheck build` xanh; `git diff --stat` chỉ gồm `src/features/learning`.
- Thủ công (API thật, dữ liệu từ phase 11/12): học viên login → `/learn` thấy lớp + % → "Học tiếp" mở bài đầu → video
  phát (URL ký) → "Đã học xong" → toast, % tăng, side list tích → "Bài tiếp" → markdown hiển thị đúng (code block,
  blockquote, ảnh qua `/media/{id}/content`) → về lộ trình: checkbox bài chưa mở disabled với title "Mở học liệu trước
  khi tích" → admin kết thúc lớp → tải lại: Alert ended, mọi tick disabled "Lớp đã kết thúc", vẫn xem được video.
- Hết hạn URL: đặt TTL ngắn ở API dev (phase 8), tua video sau khi hết hạn → tự refetch và phát tiếp từ vị trí cũ,
  không hiện lỗi; ngắt mạng → sau 2 lần hiện Alert.
- Admin/teacher gõ `/learn` → middleware phase 10 đẩy về trang chủ vai trò (không phải việc của phase này nhưng phải
  kiểm).
- a11y: checkbox có `aria-label` chứa tiêu đề; nút disabled có `title` và vẫn focus được (`aria-disabled`); `<video>`
  có `controls` gốc, không autoplay; `aria-current="page"` ở side list; crumbs `aria-label="Đường dẫn"`; 320px không
  tràn ngang, viewer một cột; reduced-motion không ảnh hưởng video.

## Security notes

- Video chỉ phát qua URL ký ngắn hạn từ `GET /media/{id}/url`; UI không tự ghép URL storage, không log URL ra console.
- `<video>` không đặt `crossorigin` để tránh yêu cầu CORS trên bucket; `controlsList="nodownload"` chỉ là tiện ích,
  không phải biện pháp bảo vệ (ghi rõ trong code comment).
- Markdown render qua `MarkdownContent` (DOMPurify `USE_PROFILES:{html:true}`, `rel="noopener noreferrer"` cho link
  `_blank`); không `dangerouslySetInnerHTML` ngoài component đó.
- Mọi quyết định tích/bỏ tích do server xác thực (thành viên, trạng thái lớp, đã mở); `tickBlocker` ở client chỉ để
  hiển thị lý do, UI luôn hiển thị lỗi server khi lệch.
- Mutation chỉ qua `@/shared/http` (gắn `X-Requested-With: fetch`, cookie `__Host-sid`); không có `fetch` trực tiếp
  trong `src/features/learning` (lint rule `no-restricted-globals` của phase 10 áp dụng).
- Không lưu tiến độ ở `localStorage`; cache React Query mất khi reload là chấp nhận được.

## Risks & Rollback

- **MediaError không có HTTP status** (chỉ code 2/4) nên không phân biệt được hết hạn URL với file hỏng: giới hạn 2
  lần refetch rồi báo lỗi; nếu lỗi lặp ở file cụ thể, admin kiểm file (MP4 faststart là việc của pipeline upload phase
  6).
- **Safari iOS** với `preload="metadata"` và khôi phục `currentTime` trước `loadedmetadata` không có tác dụng: luôn
  set trong `onLoadedMetadata`; test thủ công trên Safari.
- **Lệch hợp đồng với Phase 08**: schema Zod parse golden JSON của Phase 08 trong unit test; đổi DTO ở API phải cập
  nhật golden (cờ `-update`) và test FE đỏ ngay thay vì lỗi runtime.
- **Optimistic toggle** lệch khi admin kết thúc lớp cùng lúc: rollback + toast từ `INVALID_TRANSITION`, `invalidate
  ['my-class']` để Alert ended hiện ra.
- Rollback: revert commit `src/features/learning`; `routes.tsx` về stub rỗng, app vẫn build.

## Success Criteria

- Ba trang `/learn`, `/learn/classes/:id`, `/learn/classes/:id/lessons/:lid` khớp 1:1 prototype về bố cục, lời thoại,
  toast, trạng thái rỗng/disabled và lý do disabled.
- Zod schema parse được golden JSON của Phase 08; mã lỗi xử lý đúng ba mã `NOT_FOUND`, `CONFLICT`,
  `INVALID_TRANSITION`; lớp nháp hiển thị lộ trình chỉ đọc.
- FR-31 ghi "mở lần đầu" khi vào trang học liệu và mở khóa nút tích ngay không cần tải lại; tiến độ chỉ tính học liệu
  bắt buộc, khớp số ở báo cáo phase 12.
- Video phát qua URL ký, tự hồi phục khi hết hạn, lỗi rõ sau 2 lần; markdown an toàn, ảnh hiển thị qua
  `/media/{id}/content`.
- Không file nào ngoài `src/features/learning` bị sửa; lint, typecheck, test, build xanh.
