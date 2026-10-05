---
phase: 14
title: "Phase 14: E2E, hardening, tài liệu và phát hành"
status: pending
priority: P1
effort: "2 ngày"
dependencies: [11, 12, 13]
---

# Phase 14: E2E, hardening, tài liệu và phát hành

## Goal

Chứng minh toàn hệ thống đạt mọi tiêu chí chấp nhận ở `plan.md` §3 trên stack thật (compose profile `full`), khóa các lỗ hổng vận hành trước khi mở ra Internet, cấu hình Cloudflare R2 cho production (đã chốt, không spike), và để lại tài liệu đủ cho người khác deploy, vận hành và rollback. <!-- Updated: Validation Session 1 - V7/V8 bỏ storage spike, R2 đã chốt, bỏ pilot-checklist -->

Phase này không thêm tính năng. Mọi lỗi tìm thấy được sửa tại feature sở hữu (phase 04–13) và ghi lại ở mục "Defects" của báo cáo phase.

## Context & Requirements

- `prototype/check-flows.mjs` là danh sách kiểm thử chấp nhận của prototype (kịch bản 7.3, mời, đăng nhập lần đầu, lọc báo cáo, tích học liệu, reflow 320px, reduced motion, announcer, tab, modal, toast). Mỗi `check(...)` trong file đó phải có một assertion tương ứng chạy trên bản thật, trừ các mục chỉ có ở prototype (`reset-data`, `details.guide`, `demo-login`, `?as=`), vốn nằm ngoài phạm vi theo `plan.md` §2.
- Spec §8 NFR: HTTPS, hash mật khẩu, không log bí mật, rate limit đăng nhập/quên mật khẩu/mời, lọc HTML, URL ký ngắn hạn, bất biến ở tầng DB, FR-17 một giao dịch, audit log.
- Spec §7.3 ba tiêu chí chấp nhận: basic01/basic02 giữ Database v1 và tiến độ không đổi; basic03 thấy Database v2; cảnh báo FR-18 biến mất.
- `docs/review.md` và `docs/design.md`: render-check 3 viewport, không hex thô trong component, không emoji icon, không chữ dưới 12px, hit area ≥ 44px, gradient chỉ trên một hành động mỗi trang, body ≥ 4.5:1, UI ≥ 3:1.
<!-- Updated: Validation Session 1 - V8 production dùng Cloudflare R2 -->
- Rủi ro `plan.md` §10: MinIO server đã archive (2026-04-25); dev/e2e giữ image MinIO ghim tag; production dùng Cloudflare R2 qua S3 API (`S3_ENDPOINT=<account>.r2.cloudflarestorage.com`, `S3_REGION=auto`, `S3_USE_SSL=true`, `S3_PUBLIC_ENDPOINT`, `S3_BUCKET=lms-media`; presign TTL ≤ 7 ngày, `MEDIA_URL_TTL=2h` giữ nguyên). Không có service minio trong prod compose. CORS bucket cấu hình qua dashboard R2 với JSON ghi ở `docs/runbook.md`.
- Nghiên cứu frontend §4: `<video>` không lộ HTTP status khi URL ký hết hạn; player phải refetch URL, khôi phục `currentTime`, tối đa 2 lần rồi hiện alert.
<!-- Red Team: RT-14 - một seed duy nhất (seed.go port từ prototype/seed.js); RT-13 - seed-reset một lần ở globalSetup -->
- Seed E2E: tái sử dụng `lms seed --reset` của phase 02 (`seed.go` port 1:1 từ `prototype/seed.js`, không có profile riêng): chặng DB/DS/GO/RE/WEB (mỗi chặng v1 published), khóa BASIC v1 published, lớp `basic01` `active` (giảng viên `huong.le@goup.vn`), `basic02` `active` (giảng viên `bao.pham@goup.vn`), `basic03` `draft` (giảng viên `huong.le@goup.vn`); tài khoản admin / giảng viên Hương, Bảo / học viên An / Minh (mật khẩu tạm còn hạn) / Dũng (mật khẩu tạm hết hạn) / Thảo (vô hiệu hóa, đã rời `basic01`). Seed **không có lớp `ended`**: test cần lớp đã kết thúc tự tạo lớp mới rồi gọi API kích hoạt + kết thúc (tag `@serial`); case lớp nháp phía học viên dùng `basic03`. Seed idempotent; `globalSetup` của Playwright gọi `make seed-reset` đúng một lần rồi tạo `storageState` cho từng vai trò. Test làm bẩn trạng thái dùng chung (E01–E05, E07, E11, E12, E17, E20) gắn tag `@serial` và tự `seed-reset` ở `beforeAll`; test tạo dữ liệu mới dùng email duy nhất `e2e+<testId>@example.com` và lọc Mailpit theo người nhận.

## E2E test matrix

Chạy bằng Playwright trong `apps/web/e2e/`, baseURL `http://localhost:8081` (web nginx) với `/api` đi qua cùng origin. Email đọc qua Mailpit API `http://localhost:8025/api/v1/messages` (helper `mailpit.ts` lọc `?query=to:<email>`). Trạng thái DB kiểm qua helper `e2e/support/db.ts` (kết nối `postgres://` của compose, chỉ SELECT). Toàn bộ suite chạy tuần tự (`workers: 1`, `fullyParallel: false`); file `@serial` đứng trước, mọi file khác độc lập thứ tự.

| ID | File | Luồng | Bước | Assertion | FR |
|---|---|---|---|---|---|
| E01 | `content-versioning.spec.ts` | 7.3 bước 1: nhân bản | Admin mở `/admin/stages/:db`, bấm "Nhân bản" trên v1 | URL có `?v=<sv-v2>`, heading "Học liệu của v2", DB: `stage_versions` có đúng 1 draft với `version_no=2`, 3 `lessons` cùng `lesson_key` với v1, `video_media_id` trỏ cùng `media_files` (không sao chép file); nút "Nhân bản" biến mất khi đã có draft | FR-12 |
| E02 | cùng file | 7.3 bước 2: sửa nháp | "Thêm học liệu" video "JOIN và subquery" 12:40 bắt buộc | Draft có 4 học liệu, v1 vẫn 3; `PATCH` trực tiếp lên học liệu của v1 qua API trả `409 VERSION_IMMUTABLE` | FR-11, FR-13 |
| E03 | cùng file | 7.3 bước 3: phát hành | "Phát hành v2" → xác nhận | Badge "Đã phát hành", `published_at` NOT NULL; mục "Khóa học đang dùng phiên bản cũ của chặng này" liệt kê "Lập trình cơ bản v1"; `/admin` hiện cảnh báo FR-18 và KPI "Khóa học dùng chặng cũ" = 1; audit `stage_version.published` | FR-13, FR-18 |
| E04 | cùng file | 7.3 bước 4: áp dụng FR-17 | Bấm "Xem và áp dụng" cho Basic, modal nêu "Nhân bản Lập trình cơ bản v1 thành v2 … Phát hành v2", xác nhận | `course_versions` của BASIC: 2 bản đều `published`; v2 chứa sv-db-v2, không chứa sv-db-v1, cùng số chặng; v1 không đổi; `classes` basic01/02/03 vẫn `course_version_id` = v1; cảnh báo biến mất, hiện "Mọi khóa học đã phát hành đều dùng phiên bản mới nhất"; dashboard hết cảnh báo; audit `course.stage_version_applied` + `course_version.cloned` + `course_version.published` cùng một `at` | FR-17, FR-18 |
| E05 | cùng file | 7.3 bước 5 + tiêu chí chấp nhận | Admin đổi basic03 (draft) sang Basic v2 ở tab Cài đặt, kích hoạt lớp; học viên của basic01 và basic03 đăng nhập | basic03 settings select chỉ liệt kê bản `published`; học viên basic01 thấy 3 học liệu Database, % không đổi so với trước E01; học viên basic03 thấy 4 học liệu Database; thử đổi phiên bản trên lớp `active` → `409 INVALID_TRANSITION` với "Chỉ đổi phiên bản khi lớp còn nháp." (nguyên văn prototype) | FR-21, FR-22, §7.3 |
| E06 | `content-rules.spec.ts` | Chặn phát hành | Tạo chặng mới, phát hành nháp rỗng; tạo khóa học gắn chặng còn draft, phát hành | Nút phát hành disabled + tooltip; API trả `409` "Chặng cần ít nhất một học liệu trước khi phát hành"; khóa học: alert blocker liệt kê chặng chưa phát hành | FR-13, FR-15 |
| E07 | cùng file | Xóa / lưu trữ | Xóa draft; xóa bản published đang dùng; lưu trữ | Draft xóa được (cascade lessons); published có tham chiếu: nút xóa disabled, lý do "Đang dùng trong …", API `409 IN_USE` kèm `details.usedBy`; lưu trữ OK và audit `*.archived` | FR-19 |
| E08 | `invitations.spec.ts` | Mời trùng | `/admin/classes/:basic01`, "Mời học viên" với `  AN.NGUYEN@gmail.com ` | Lỗi trong modal "Học viên đã có trong lớp" (email đã chuẩn hóa) | FR-01 |
| E09 | cùng file | Mời email mới | Mời `e2e+e09@example.com` (email duy nhất theo test, RT-13) | Hàng mới: tài khoản "Chưa đăng nhập", lời mời `queued` rồi `sent` trong ≤ 15 s (poll UI + Mailpit có 1 thư tới địa chỉ đó chứa mật khẩu ≥ 12 ký tự); DB `users.must_change_password=true`, `temp_password_expires_at` ≈ now+72h; response API không chứa mật khẩu | FR-01, FR-50 |
| E10 | cùng file | Email thất bại | Mời địa chỉ mà Mailpit được cấu hình từ chối (`bounce.me@…` qua `MP_SMTP_DISABLE_RDNS`/reject rule, hoặc tạm dừng Mailpit trong test) | Sau 3 lần thử: dot `failed`, hiển thị "3 lần", `last_error` rút gọn; dashboard alert "Lời mời thất bại" đếm 1; "Gửi lại" tạo outbox mới và mật khẩu tạm mới (hash đổi) | FR-01, FR-02 |
| E11 | cùng file (`@serial`) | Mời vào lớp đã kết thúc / tài khoản nội bộ | Tạo lớp mới `e2e11` qua API (BASIC v1, giảng viên Hương), kích hoạt rồi kết thúc (seed không có lớp `ended`); nút mời disabled; mời email của giảng viên vào `basic01` | "Không mời được vào lớp đã kết thúc"; "Email này thuộc tài khoản nội bộ, không mời làm học viên được" | FR-01 |
| E12 | cùng file | Gỡ học viên | "Gỡ" An khỏi basic01 → xác nhận | Member `dropped`, `lesson_progress` của An còn nguyên; An đăng nhập không còn thấy basic01; audit `class.member_dropped` | FR-23 |
| E13 | `auth.spec.ts` | Đăng nhập lần đầu | Minh đăng nhập bằng mật khẩu tạm | Chuyển `/first-login`; gọi thẳng `GET /api/v1/me/classes` trả `403 PASSWORD_CHANGE_REQUIRED`; mật khẩu "short" → "Mật khẩu mới cần tối thiểu 8 ký tự"; trùng mật khẩu tạm → "Mật khẩu mới phải khác mật khẩu tạm"; hợp lệ → toast "Đã lưu mật khẩu. Chào mừng bạn vào lớp." và về `/learn`; DB `must_change_password=false`, `status='active'`, `temp_password_expires_at IS NULL` | FR-03 |
| E14 | cùng file | Hết hạn / vô hiệu | Dũng (hết hạn), Thảo (disabled) đăng nhập | "Mật khẩu tạm đã hết hạn, vui lòng liên hệ quản trị viên."; "Tài khoản đã bị vô hiệu hóa. Vui lòng liên hệ quản trị viên." | FR-03, FR-06 |
| E15 | cùng file | Khóa đăng nhập | 5 lần sai mật khẩu cho một email trong 15 phút, lần 6 đúng | Lần 6 trả `429 TOO_MANY_ATTEMPTS` kèm `Retry-After`; UI hiện thông báo khóa tạm; đăng nhập email khác cùng IP vẫn được | FR-05 |
| E16 | cùng file | Quên mật khẩu | `/forgot` với email có và không tồn tại | Cùng một thông báo thành công; chỉ email tồn tại nhận thư trong Mailpit; link `/reset-password#token=…` (token trong fragment, không vào log/Referer) đặt mật khẩu mới; token dùng lần 2 → lỗi; sửa `expires_at` về quá khứ qua DB helper → "liên kết đã hết hạn"; rate limit lần thứ N trong 1 phút → `429` | FR-04 |
| E17 | cùng file | Vô hiệu hóa đang có phiên | Admin "Vô hiệu hóa" An trong khi An đang đăng nhập ở context Playwright thứ hai | Request kế tiếp của An trả `401`; `sessions` của An bị xóa; `lesson_progress` còn nguyên; "Kích hoạt lại" cho đăng nhập lại | FR-06 |
| E18 | cùng file | Guard vai trò | Giảng viên vào `/admin`, học viên vào `/teach` | Redirect về trang chủ của vai trò; API `GET /dashboard` với giảng viên trả `403 FORBIDDEN` | §4 spec |
| E19 | `learning.spec.ts` | Mở và tích học liệu | An mở học liệu chưa xong đầu tiên, tích, quay lại lộ trình, bỏ tích | `first_opened_at` set sau khi mở; `completed_at` set sau tích; % lộ trình tăng rồi về giá trị cũ; tích lần 2 cùng trạng thái không đổi `completed_at` (idempotent); `PUT completion` khi chưa mở → "Mở học liệu trước khi tích hoàn thành."; học liệu không thuộc phiên bản của lớp → "Học liệu không thuộc phiên bản khóa học của lớp." | FR-31, FR-32, FR-33 |
| E20 | cùng file (`@serial`) | Lớp `ended` / `draft` | Admin tạo lớp `e2e20` (BASIC v1), mời An, kích hoạt, An mở một học liệu, admin kết thúc lớp (seed không có lớp `ended`); An mở lại học liệu. Lớp nháp: An là thành viên `basic03` (draft trong seed) | Lớp ended: xem được nội dung, Alert "Lớp đã kết thúc…", checkbox "Đã học xong" disabled với `title` "Lớp đã kết thúc"; `PUT completion` trả `409 INVALID_TRANSITION`. Lớp draft `basic03`: `GET /me/classes/{id}` trả `200` `readOnly=true`, `readOnlyReason='draft'`, Alert "Lớp chưa bắt đầu.", thẻ ở `/learn` không có nút "Học tiếp" | FR-32, Q3 |
| E21 | cùng file | Video ký URL và tua | Mở học liệu video; chặn request tới storage bằng `page.route` trả 403 một lần, rồi tua | `GET /media/:id/url` trả `{url, expiresAt}`; sau lỗi, client gọi lại `/url`, `currentTime` khôi phục ±1 s; sau 2 lần lỗi liên tiếp hiện alert "Không phát được video. Tải lại trang để thử lại."; URL ký có `X-Amz-Expires` ≥ 7200; GET trực tiếp object không chữ ký → 403 | FR-11, NFR |
| E22 | cùng file | Markdown đã lọc | Admin tạo học liệu markdown chứa `<script>`, `<img onerror>`, link `javascript:`; học viên mở | HTML render không có `script`, không `onerror`, link `javascript:` bị loại; link ngoài có `rel="noopener noreferrer"`; ảnh trong markdown trỏ `/api/v1/media/:id/content` và tải được (302) | FR-11, NFR |
| E23 | `reports.spec.ts` | Lọc và sắp xếp | `/admin/classes/:basic01?tab=report` với `below=30`, `sort=pct-desc`, `sort=pct`, `notlogged=1`, `inactive=7` (URL giữ tên param của prototype; web ánh xạ `below→belowPercent`, `notlogged→notLoggedIn`, `inactive→inactiveDays` khi gọi API, RT-14) | Số hàng giảm đúng theo dữ liệu seed; thứ tự đảo; `notlogged` chỉ còn Dũng; `inactive=7` còn Dũng + Khang; mặc định không có Thảo (dropped), tích "Hiện học viên đã rời lớp" → có Thảo với badge "Đã rời lớp"; filter form ghi vào search params (`below=50`); nút "Xóa bộ lọc" về mặc định; badge "Tiến độ do học viên tự xác nhận" luôn hiện | FR-40, FR-42 |
| E24 | cùng file | Drilldown | Bấm một hàng | Sheet mở với tên, email, lớp, hoạt động cuối, progress-lg, từng chặng với "Mở lần đầu …"/"Chưa mở"/"Tích …"; `SheetTitle` tồn tại; Escape đóng | FR-41 |
| E25 | cùng file | Phạm vi giảng viên | Giảng viên mở `/teach`, `/teach/classes/:basic01`, gọi API báo cáo lớp không phụ trách | Chỉ thấy lớp mình phụ trách; card lớp `active` có đếm "lâu không hoạt động" > 7 ngày, lớp `draft` không có; API lớp khác trả `403` | FR-40 |
| E26 | `a11y.spec.ts` | Reflow 320px | 10 route trong `check-flows.mjs` ở viewport 320×812 | `document.documentElement.scrollWidth <= 320` | NFR giao diện |
| E27 | cùng file | Reduced motion | `emulateMedia({reducedMotion:'reduce'})` | `transitionDuration === '0s'` trên nút chính, `animationDuration === '0s'` trên dialog | review.md |
| E28 | cùng file | Announcer | Điều hướng `/admin` → `/admin/stages` | Vùng `aria-live=polite` chứa đúng tiêu đề trang một lần; `document.title` = "Chặng · GoUp LMS"; `#app` không có `aria-live`; focus chuyển về `h1` | review.md |
| E29 | cùng file | Tabs | Tab lớp | Không có `[role=tablist]`; `nav[aria-label]` chứa `a[aria-current=page]` "Tiến độ" khi `?tab=report` | review.md |
| E30 | cùng file | Dialog | Mở modal nhân bản | `role=dialog` có `aria-labelledby` trỏ tới heading không rỗng; focus bẫy trong dialog; Escape đóng và trả focus về nút mở | review.md |
| E31 | cùng file | Toast | Sau một thao tác thành công | Toast có nút `aria-label="Đóng thông báo"` focus được; bấm đóng → biến mất; tối đa 3 toast; tự tắt sau 4 s | review.md |
| E32 | cùng file | Skip link + keyboard | Tab từ đầu trang | Phần tử focus đầu tiên là "Bỏ qua điều hướng", Enter đưa focus tới `main#main`; mọi icon button có `aria-label` (axe rule `button-name`); `@axe-core/playwright` không có violation mức `serious`/`critical` trên 10 route | review.md |

<!-- Red Team: RT-13 - cấu hình Playwright chống flaky: tuần tự, chromium mặc định, webkit job riêng -->
`playwright.config.ts`: `workers: 1`, `fullyParallel: false`, `retries: 0` (local và CI chromium; chỉ job webkit CI dùng `retries: 1`), project `e2e` chạy chromium mặc định; project `webkit` cùng spec chạy ở job CI riêng tuần tự (sau chromium), không chạy local mặc định. `globalSetup` = `make seed-reset` + đăng nhập từng vai trò qua `POST /api/v1/auth/login` (`request`, header `X-Requested-With: fetch`) lưu `storageState` vào `e2e/.auth/<role>.json`; spec dùng `test.use({storageState})` thay vì lặp UI đăng nhập. Chụp screenshot khi fail; trace `retain-on-failure`.

## Hardening checklist

Mỗi mục có lệnh kiểm chứng; kết quả ghi vào `plans/reports/hardening-<date>-lms-release.md`.

| # | Mục | Cách kiểm | Pass khi |
|---|---|---|---|
| H1 | Security headers qua Caddy (prod compose chạy local với cert nội bộ `tls internal`) | `curl -skI https://localhost/ \| grep -iE 'strict-transport\|x-content-type\|referrer-policy\|x-frame\|permissions-policy\|content-security-policy\|^server'` | HSTS `max-age≥15552000`, `nosniff`, `strict-origin-when-cross-origin`, `DENY`, Permissions-Policy, CSP có `default-src 'self'`, `script-src 'self'`, `media-src 'self' <storage-host>`, `frame-ancestors 'none'`; không có header `Server` |
| H2 | Cookie phiên | `curl -ski -X POST https://localhost/api/v1/auth/login -H 'Content-Type: application/json' -H 'X-Requested-With: fetch' -d @e2e/fixtures/login-admin.json \| grep -i set-cookie` | `__Host-sid=…; Path=/; Secure; HttpOnly; SameSite=Lax`, không có `Domain` |
| H3 | CSRF | Gửi `POST /api/v1/classes` kèm cookie hợp lệ nhưng `Origin: https://evil.test` và `Sec-Fetch-Site: cross-site`; lần 2 cùng origin nhưng thiếu `X-Requested-With` | Cả hai trả `403`; cùng origin đủ header trả `201` |
| H4 | Rate limit | Chạy với `APP_ENV=dev` (limiter tắt khi `APP_ENV=e2e`, V2): vòng `for i in $(seq 1 35); do curl -s -o /dev/null -w '%{http_code}\n' -X POST https://localhost/api/v1/auth/forgot-password -H 'Content-Type: application/json' -H 'X-Requested-With: fetch' -d '{"email":"x@example.com"}'; done \| sort \| uniq -c` (tương tự `/auth/login` với 301+ request, `/classes/:id/invitations`) | Xuất hiện `429` kèm `Retry-After` sau ngưỡng (login 300/phút theo IP, forgot 30/phút theo IP, khóa theo email sau 5 lần sai); `/healthz` không bị giới hạn. Lệnh ghi vào `make hardening`, không có script riêng (V7) |
| H5 | Bất biến phiên bản (tầng ứng dụng, D1) | `make hardening-immutability`: (1) `go test -tags integration -run 'TestImmutability' ./internal/features/stages/... ./internal/features/courses/...` (Phase 05/06: gọi repository trực tiếp lên bản published: UPDATE/INSERT/DELETE lesson, DELETE/INSERT course_version_stages, UPDATE status published→draft; archive); (2) `psql -Atc "SELECT count(*) FROM pg_trigger WHERE NOT tgisinternal"` và `pg_proc` của schema `public`; (3) `PATCH /api/v1/lessons/{id}` lên học liệu của v1 published qua API | (1) mọi test xanh: từng thao tác trả `ErrVersionImmutable`/`ErrInvalidTransition`, dữ liệu không đổi, chỉ `published→archived` thành công; (2) cả hai đếm = 0; (3) `409 VERSION_IMMUTABLE` <!-- Updated: Session 2 - D1 --> |
| H6 | Không lộ bí mật trong log | <!-- Red Team: RT-12 - grep -F theo giá trị thật lấy từ Mailpit, bao cả web và caddy --> Chạy E09 + E13 + E16; lấy mật khẩu tạm và token reset thật từ Mailpit (`curl -s 'localhost:8025/api/v1/search?query=to:e2e+e09@example.com'` rồi đọc body thư; token lấy sau `#token=`); với từng giá trị: `docker compose -f infra/docker-compose.yml logs api worker web caddy \| grep -Fc "<giá trị>"` | Mỗi lệnh trả `0`; thêm `grep -Fc password_hash` và `grep -Fc Authorization` cũng `0`; log JSON không có field `password`, `token`, `body` |
| H7 | Không lộ bí mật trong API/audit | `psql -c "select count(*) from audit_logs where before::text ~* 'password\|token' or after::text ~* 'password\|token'"`; JSON của `GET /classes/:id/members` và `GET /dashboard` | 0; không có khóa `password*`/`token*` trong response |
| H8 | Quét secret trong repo | `gitleaks detect --source . --redact --no-banner` | exit 0 |
| H9 | (đã bỏ) | <!-- Updated: Validation Session 1 - V7 bỏ govulncheck/pnpm audit, giữ gitleaks H8 --> Không chạy `govulncheck`/`pnpm audit` trong MVP | – |
| H10 | Image non-root, không shell | `docker inspect lms-api --format '{{.Config.User}}'`; `docker run --rm --entrypoint sh lms-api -c true` | User `nonroot`; lệnh sh thất bại (distroless); web nginx chạy user `nginx`, port 80 không privileged (dùng `nginx-unprivileged` hoặc `cap_add NET_BIND_SERVICE`) |
| H11 | Port nội bộ không mở | `docker compose -f infra/docker-compose.prod.yml config \| grep -A2 ports` | Chỉ `caddy` có `80`, `443`; postgres/storage/mailpit/api/web không publish |
| H12 | Bucket private | Dev/e2e (MinIO): `curl -sI http://localhost:9000/lms-media/<key>` không chữ ký; prod (R2): `curl -sI https://<account>.r2.cloudflarestorage.com/lms-media/<key>` không chữ ký | `403` ở cả hai |
| H13 | `must_change_password` allowlist | Với phiên của Minh (chưa đổi): gọi lần lượt `GET /auth/me`, `POST /auth/change-password` (sai), `POST /auth/logout`, và 3 route khác | 3 route allowlist không trả `PASSWORD_CHANGE_REQUIRED`; mọi route khác trả `403 PASSWORD_CHANGE_REQUIRED` |
| H14 | Health | `curl -s localhost:8080/readyz` khi tắt postgres | `503` với body không lộ DSN |

## Render-check vs prototype

1. Dựng prototype tĩnh: `python3 -m http.server 8090 --directory prototype` (hoặc `pnpm dlx serve prototype -l 8090`). Dựng web qua compose profile `full` (`http://localhost:8081`) với seed e2e để dữ liệu hai bên giống nhau.
2. Script `apps/web/e2e/render-check.spec.ts` (Playwright, project riêng `render`, không chạy trong `pnpm test:e2e` mặc định): với mỗi route ở `plan.md` §8 và mỗi viewport `1440×900`, `768×1024`, `375×812`, đăng nhập đúng vai trò, chụp `e2e/__screenshots__/<route>/<viewport>.png` của web và `prototype/<route>/<viewport>.png` của prototype (prototype dùng `#/…?as=<user>`).
3. So sánh bằng `expect(webShot).toMatchSnapshot` với `maxDiffPixelRatio: 0.02` theo từng vùng ổn định (topbar, page-head, card đầu tiên, bảng) dùng `clip`; vùng có dữ liệu động (thời gian "x phút trước") bị mask bằng `mask: [page.locator('time')]`. Mục tiêu không phải pixel-perfect toàn trang mà là cùng token: màu, chữ, khoảng cách, radius. Diff vượt ngưỡng phải được giải thích hoặc sửa ở phase 10–13.
4. Kiểm computed style trực tiếp (chắc hơn screenshot): trên mỗi route lấy `getComputedStyle` của `header` (`background-color` = `rgb(35, 60, 101)`), link nav active (`border-bottom-color` = `rgb(244, 93, 41)`), nút chính (`background-image` chứa `linear-gradient(260deg`), `body` (`font-family` bắt đầu "Inter Tight", `font-size` 15px, `background-color` `rgb(245, 245, 245)`), `.card` (`border-radius` 3px, `box-shadow` `0 0 7px`), input (`height` 40px). So với cùng phần tử của prototype; lệch → fail.
5. Chạy `node .claude/skills/ak-frontend-design/scripts/render-check.mjs http://localhost:8081/<route> --out plans/reports/render-<date>/ --viewports 375x812,768x1024,1440x900` cho từng route sau khi đăng nhập (script nhận URL; dùng cookie từ `storageState` qua biến môi trường nếu script hỗ trợ, nếu không thì chạy bản Playwright ở bước 2 với cùng tiêu chí: 0 overflow, 0 clipping, 0 chữ < 12px). Kết quả phải là 0 lỗi.
6. Token audit (script `apps/web/scripts/design-audit.mjs`, chạy trong `pnpm lint:design` và CI):
   - Hex thô: `grep -rnE '#[0-9a-fA-F]{3,8}\b' apps/web/src --include='*.tsx' --include='*.ts' --include='*.css' | grep -v 'src/styles/'` → 0 kết quả (ngoại trừ `src/styles/goup-tokens.css`, `src/styles/app.css`).
   - Chữ nhỏ: grep `text-\[(\d+)px\]` với số < 12, và `text-[0-9]*xs` không thuộc scale (`text-xs` = 12px là hợp lệ) → 0.
   - Emoji: grep ký tự Unicode trong khối Emoji (`\p{Extended_Pictographic}`) trong `src/` → 0.
   - Gradient: trong mỗi page component, số phần tử `variant="gradient"` render cùng lúc ≤ 1. Kiểm runtime trong `a11y.spec.ts`: `document.querySelectorAll('[data-variant=gradient]').length <= 1` trên mỗi route.
   - Hit area: trong `a11y.spec.ts` ở 375px, mọi `button, a[href], input, select, [role=button]` hiển thị có `getBoundingClientRect().height >= 44 || width*height >= 44*44` (ngoại trừ link trong đoạn văn `p a`).
7. Tương phản: `@axe-core/playwright` rule `color-contrast` trên 10 route; vi phạm do viền input `gray-300` (đã ghi nhận ở `plan.md` §11, chờ quyết định validation) được liệt kê riêng, không che.

## Object storage production (Cloudflare R2)

<!-- Updated: Validation Session 1 - V8 thay storage spike bằng cấu hình R2 đã chốt -->
Không spike, không ADR. `minio-go` client (Phase 01/06) dùng với R2 qua S3 API. Prod compose không có service `minio`; biến env (tên theo Phase 01 `config.go`): `S3_ENDPOINT=<account>.r2.cloudflarestorage.com`, `S3_REGION=auto`, `S3_USE_SSL=true`, `S3_PUBLIC_ENDPOINT=<URL ký được trình duyệt truy cập, cùng host endpoint hoặc custom domain>`, `S3_BUCKET=lms-media`, `S3_ACCESS_KEY`/`S3_SECRET_KEY` (R2 API token quyền Object Read & Write giới hạn bucket), `MEDIA_URL_TTL=2h` (presign TTL ≤ 7 ngày theo giới hạn R2). Việc cần làm ở phase này:

1. Tạo bucket `lms-media` private trên R2, đặt CORS qua dashboard với JSON ghi ở `docs/runbook.md` (`AllowedOrigins: ["https://<DOMAIN>"]`, `AllowedMethods: ["GET","PUT","HEAD"]`, `AllowedHeaders: ["Content-Type","Range"]`, `ExposeHeaders: ["ETag","Content-Range","Accept-Ranges"]`, `MaxAgeSeconds: 3600`).
2. Kiểm tay trên prod compose local trỏ R2 thật (chạy một lần, ghi output vào báo cáo hardening): presign PUT rồi `curl -T` kèm `Content-Type` → `200`; presign GET → `200`; `Range: bytes=100-199` → `206` + `Content-Range`; URL hết hạn → `403`; `OPTIONS` preflight từ `https://<DOMAIN>` → `Access-Control-Allow-Methods` chứa `PUT`; phát video qua `<video>` trên Chrome, Safari, Firefox: tua giữa, tua lùi, Network thấy `206`.
3. CSP `media-src 'self' <S3_PUBLIC_ENDPOINT host>` ở Caddyfile (H1) khớp host R2.

## Docs & release

| Tài liệu | Nội dung tối thiểu | Kiểm |
|---|---|---|
<!-- Updated: Validation Session 1 - V7 bộ docs rút gọn: README, architecture, api, database, runbook, design, review -->
| `README.md` (gốc) | Mô tả hệ thống một đoạn; yêu cầu (Docker, Go 1.27, Node 22, pnpm); bảng `make` target (`dev`, `check-ports`, `migrate`, `seed`, `seed-reset`, `test`, `lint`, `build`, `e2e`, `render-check`, `hardening`, `prod-up`, `prod-down`); tài khoản seed dev; link tới `docs/README.md` | Chạy đúng từng lệnh trong bảng trên máy sạch (`git clone` vào thư mục tạm) |
| `docs/README.md` | Mục lục docs (một dòng mỗi file) | Mọi link tồn tại |
| `docs/architecture.md` (một file duy nhất) | Cây monorepo thực tế, bounded context, sơ đồ luồng FR-17, bảng ubiquitous language lấy từ `plan.md` §4, nguyên tắc DDD/SOLID đã áp dụng với ví dụ file thật | Mọi đường dẫn file trong doc tồn tại (kiểm tay bằng `ls`, không có script riêng) |
| `docs/api.md` | Bảng route `/api/v1` viết từ `plan.md` §7 và đối chiếu tay với `router.go` (không có lệnh `lms routes`), envelope lỗi, mã lỗi, ví dụ request/response cho login, invite, apply, completion, dashboard | Mỗi route trong doc có handler trong `router.go` và ngược lại (đọc chéo khi review) |
| `docs/database.md` | Phase 02 sở hữu (ERD Mermaid, bảng, ràng buộc khai báo, quyết định D1 không function/trigger, bảng bất biến ở tầng ứng dụng, chiến lược migrate); phase này chỉ xác minh | `make migrate-status` khớp danh sách; không viết lại |
| `docs/runbook.md` (thay `operations.md` + `release.md`) | <!-- Red Team: RT-04 - backup trước migrate, rollback bằng hạ image + MIGRATE_ON_START=false --> Deploy: `cp infra/.env.example infra/.env`, điền biến (bảng env đầy đủ theo Phase 01 `config.go`, gồm nhóm `S3_*` cho R2), `make prod-up` (compose prod + Caddy tự lấy cert theo `DOMAIN`); R2: tạo bucket, token, JSON CORS; nâng cấp: **luôn** `pg_dump -Fc` trước khi kéo image có migration mới, rồi pull image, `make prod-up` (API tự migrate khi `MIGRATE_ON_START=true`); migration theo expand/contract (không drop cột trong cùng bản phát hành với code bỏ dùng cột); rollback: hạ tag image về bản trước với `MIGRATE_ON_START=false` (schema mới tương thích ngược nhờ expand/contract), chỉ `pg_restore` dump khi dữ liệu hỏng; backup: lệnh `pg_dump -Fc` thủ công/cron ghi trong doc (không có script trong repo) + đồng bộ bucket; restore: `pg_restore`; log: `docker compose -f infra/docker-compose.prod.yml logs -f api worker`; sự cố email: xem `email_outbox` `failed` bằng SQL, requeue bằng `UPDATE email_outbox SET status='queued', attempts=0 WHERE id=…` (không có lệnh `lms outbox`); Mailpit chỉ dev/e2e; xoay vòng secret; lưu ý `__Host-` cookie cần HTTPS | Diễn tập trên máy local: deploy prod compose với `tls internal`, `pg_dump -Fc`, xóa DB, `pg_restore`, kiểm đăng nhập lại được; rollback image + `MIGRATE_ON_START=false` khởi động được |
| `docs/design.md`, `docs/review.md` | Thêm mục "Bản React": vị trí token (`src/styles/`), lệnh `pnpm lint:design`, `make render-check`, `make e2e` thay cho `check-flows.mjs` khi kiểm bản thật; prototype vẫn giữ checklist cũ | Link và lệnh chạy được |
| Tag | Tag annotated `v0.1.0` (không có `CHANGELOG.md`; ghi chú phát hành là message của tag); commit theo conventional commit (`feat:`, `fix:`, `docs:`, `chore:`), không đề cập công cụ AI trong message; không commit `.env`, dump, screenshot baseline chứa dữ liệu thật | `git log --format=%s` không khớp `/claude|ai|gpt/i`; `gitleaks` pass |

## Files to Create / Modify

Tạo mới:

- `apps/web/e2e/playwright.config.ts` (`workers: 1`, `fullyParallel: false`, `retries: 0`, `globalSetup`; projects: `e2e` chromium, `webkit` (chỉ CI job riêng), `a11y`, `render` tách riêng; `webServer` không dùng, baseURL từ env `E2E_BASE_URL` mặc định `http://localhost:8081`) <!-- Red Team: RT-13 -->
- `apps/web/e2e/global-setup.ts` (`make seed-reset` một lần + `storageState` từng vai trò vào `e2e/.auth/`, thư mục này trong `.gitignore`)
- `apps/web/e2e/support/{auth.ts,db.ts,mailpit.ts,seed.ts,a11y.ts}`
- `apps/web/e2e/fixtures/login-*.json`
- `apps/web/e2e/{content-versioning,content-rules,invitations,auth,learning,reports,a11y,render-check}.spec.ts`
- `apps/web/scripts/design-audit.mjs`
- `Makefile`: target `hardening-immutability` (gọi integration test `-run TestImmutability` của Phase 05/06 + hai câu đếm catalog) <!-- Updated: Session 2 - D1 thay immutability_check.sql -->
- `docs/{README,architecture,api,runbook}.md` (`docs/database.md` do Phase 02 viết, chỉ xác minh) <!-- Updated: Validation Session 1 - V7 không còn conformance_test, routes.go, outbox.go, infra/spike, infra/scripts, scripts/*, security.md, pilot-checklist.md, decisions/, CHANGELOG.md -->
- `plans/reports/hardening-<date>-lms-release.md`, `plans/reports/render-<date>/`

Sửa:

- `Makefile` (thêm `e2e`, `e2e-ui`, `render-check`, `seed-reset`, `hardening`, `prod-up`, `prod-down`; không có `backup`, `storage-spike`)
- `apps/web/package.json` (`test:e2e`, `test:a11y`, `render-check`, `lint:design`; dev deps `@playwright/test`, `@axe-core/playwright`, `pg`)
- `.github/workflows/e2e.yml` (dựng profile `full`, chờ `/readyz`, chạy `e2e` chromium + `a11y` (globalSetup tự seed-reset); job `webkit` riêng chạy sau, tuần tự, `retries: 1`; upload trace/screenshot khi fail; `render` chỉ chạy khi `workflow_dispatch`)
- `.github/workflows/ci.yml` (thêm `pnpm lint:design`, `gitleaks`; không thêm `govulncheck`/`pnpm audit`, V7)
- `infra/docker-compose.yml` (profile `full` đặt `APP_ENV=e2e`, `EMAIL_POLL_INTERVAL=1s`, `MAIL_FAIL_PATTERN=^e2e\+bounce` cho service `worker` của Phase 04; worker chỉ đọc `MAIL_FAIL_PATTERN` khi `APP_ENV=e2e`, RT-11)
- `infra/docker-compose.prod.yml` (không có `minio`; `S3_*` trỏ R2), `infra/.env.example`, `infra/caddy/Caddyfile` (H1, `media-src` host R2)
- `README.md`, `docs/design.md`, `docs/review.md`
- Bất kỳ file feature ở phase 04–13 cần sửa để E2E/hardening pass (ghi trong báo cáo phase)

## Tasks & Steps

1. **Chuẩn bị hạ tầng E2E** (0.25 ngày)
   1. Thêm target `make e2e-up` = `docker compose -f infra/docker-compose.yml --profile full up -d --build --wait`; `make seed-reset` = `docker compose -f infra/docker-compose.yml run --rm api seed --reset` (RT-14).
   2. Cài Playwright + axe trong `apps/web`; viết `playwright.config.ts` (RT-13: `workers: 1`, chromium mặc định, `globalSetup`) với project `e2e`, `webkit`, `a11y`, `render`; `support/*` (login qua API, Mailpit client lọc theo người nhận, truy vấn DB read-only, helper axe, helper tạo lớp `ended` qua API cho E11/E20).
   3. E10 dùng `MAIL_FAIL_PATTERN` của worker Phase 04 (chỉ đọc khi `APP_ENV=e2e`; từ chối ở tầng SMTP client, unit test ở Phase 04 chứng minh prod bỏ qua biến); phase này chỉ đặt biến trong compose profile `full`.
2. **Port check-flows sang Playwright** (0.5 ngày): viết E01–E32 theo bảng; mỗi assertion của `check-flows.mjs` phải tìm được ID tương ứng (ghi mapping ở đầu mỗi spec). Chạy local tới khi xanh; lỗi sản phẩm sửa tại feature sở hữu và ghi vào mục Defects.
3. **Hardening** (0.5 ngày): chạy H1–H14 (trừ H9 đã bỏ), lưu output vào báo cáo; sửa cấu hình Caddy/compose/API cho tới khi pass; `make hardening` gom các lệnh curl/psql của H1–H7, H10–H14 (vòng lặp rate limit H4 chạy với `APP_ENV=dev`) và gọi `make hardening-immutability` để chạy lại được.
4. **Render-check và token audit** (0.25 ngày): viết `render-check.spec.ts`, `design-audit.mjs`, chạy trên 3 viewport; so với prototype; ghi diff còn lại và lý do.
5. **Object storage production** (0.25 ngày): cấu hình R2 (bucket, token, CORS JSON), cập nhật prod compose, `.env.example`, Caddyfile; chạy kiểm tay 6 điểm ở mục "Object storage production" và ghi output vào báo cáo hardening.
6. **Tài liệu và phát hành** (0.25 ngày): viết/cập nhật docs theo bảng (`docs/api.md` từ `plan.md` §7; `docs/runbook.md` gồm `pg_dump -Fc` trước migrate, rollback hạ image + `MIGRATE_ON_START=false`); diễn tập runbook deploy–backup–restore–rollback trên local; tag annotated `v0.1.0` chỉ sau khi `make check`, `make e2e`, `make hardening` cùng xanh.
7. **Khép lại**: so lại từng dòng `plan.md` §3 với bằng chứng (ID test, lệnh, ảnh); cập nhật trạng thái phase; ghi báo cáo `plans/reports/`.

## Verification

```bash
# Toàn bộ chất lượng tĩnh + unit + integration
make check

# Dựng stack thật và chạy E2E + a11y (globalSetup tự chạy make seed-reset một lần)
make e2e-up
cd apps/web && pnpm exec playwright test --project=e2e --project=a11y
pnpm exec playwright test --project=webkit   # chỉ trên CI job riêng, tuần tự
pnpm exec playwright test --project=render   # render-check so với prototype, chạy riêng

# Token audit
cd apps/web && pnpm lint:design
grep -rnE '#[0-9a-fA-F]{3,8}\b' apps/web/src --include='*.tsx' --include='*.ts' --include='*.css' | grep -v 'src/styles/' | wc -l   # kỳ vọng 0

# Hardening
make hardening                                   # chạy H1–H14 có script
curl -skI https://localhost/ | grep -iE 'strict-transport|x-content-type|referrer-policy|x-frame|permissions-policy|content-security-policy|^server:'
make hardening-immutability   # H5: TestImmutability_* xanh; pg_trigger/pg_proc = 0; PATCH lesson của v1 → 409 VERSION_IMMUTABLE  <!-- Updated: Session 2 - D1 -->
# H6: giá trị thật lấy từ Mailpit (không in ra terminal/CI log), grep cố định chuỗi trên mọi service
MSG_ID=$(curl -s 'localhost:8025/api/v1/search?query=to:e2e+e09@example.com' | jq -r '.messages[0].ID')
TEMP_PW=$(curl -s "localhost:8025/api/v1/message/$MSG_ID" | jq -r '.Text' | sed -n 's/.*Mật khẩu tạm: \([^ ]*\).*/\1/p')
docker compose -f infra/docker-compose.yml logs api worker web caddy | grep -Fc "$TEMP_PW"   # kỳ vọng 0 (tương tự với token reset sau '#token=')
gitleaks detect --source . --redact --no-banner
docker inspect lms-api --format '{{.Config.User}}'   # nonroot

# Object storage production (R2 thật, chạy một lần, ghi vào báo cáo)
curl -sI "$SIGNED_GET_URL" | grep -iE '^HTTP|accept-ranges'                                 # 200, bytes
curl -sI -H 'Range: bytes=100-199' "$SIGNED_GET_URL" | grep -iE '^HTTP|content-range'       # 206
curl -sI https://<account>.r2.cloudflarestorage.com/lms-media/<key> | head -1               # 403

# Docs
ls docs/README.md docs/architecture.md docs/api.md docs/database.md docs/runbook.md docs/design.md docs/review.md

# Diễn tập runbook (lệnh y hệt docs/runbook.md)
make prod-up && docker compose -f infra/docker-compose.prod.yml exec -T postgres pg_dump -Fc -U lms lms > /tmp/lms-$(date +%F).dump
docker compose -f infra/docker-compose.prod.yml exec -T postgres pg_restore -U lms -d lms --clean --if-exists < /tmp/lms-<date>.dump && curl -sk https://localhost/readyz
API_IMAGE_TAG=<tag trước> MIGRATE_ON_START=false make prod-up && curl -sk https://localhost/readyz   # rollback image
```

CI: `e2e.yml` xanh trên PR (chromium + a11y; job webkit tuần tự); `ci.yml` có thêm `pnpm lint:design` và `gitleaks` xanh.

## Risks & Rollback

| Rủi ro | Biện pháp |
|---|---|
| E2E flaky vì worker email chạy ticker 5 s | Poll có timeout 15 s; profile `full` đặt `EMAIL_POLL_INTERVAL=1s` (tên biến theo Phase 01/04, RT-11) |
| Mailpit không hỗ trợ từ chối địa chỉ theo rule | Dùng `MAIL_FAIL_PATTERN` ở worker Phase 04 chỉ khi `APP_ENV=e2e`; code path tách biệt, unit test Phase 04 chứng minh prod bỏ qua biến này |
| E2E flaky do chạy song song / trạng thái dùng chung | `workers: 1`, `fullyParallel: false`, `retries: 0` để lộ flaky thay vì che; `@serial` tự seed-reset; email duy nhất theo test; lớp `ended` do test tự tạo qua API thay vì sửa seed (RT-13) |
| Render-check so screenshot quá nhạy | Dựa chính vào kiểm computed style (bước 4); screenshot chỉ cảnh báo với ngưỡng 2% theo vùng |
| R2 không hỗ trợ một trong 6 điểm kiểm tay (ví dụ CORS, Range) | Ghi rõ điểm fail trong báo cáo; fallback stream video qua `GET /media/{id}/content` (`http.ServeContent` hỗ trợ Range), chấp nhận chi phí băng thông API, nêu để người dùng quyết |
| `__Host-` cookie không hoạt động trên `http://localhost` ở Safari | E2E webkit chạy qua `https://localhost` với `tls internal` + `ignoreHTTPSErrors`; dev local qua Vite proxy giữ cùng origin; prod luôn HTTPS. Ghi vào `docs/runbook.md` |
| Sửa lỗi phát hiện ở E2E kéo dài quá 2 ngày | Lỗi thuộc phase nào thì mở lại phase đó; phase 14 chỉ ghi nhận và chạy lại |
| Rollback | <!-- Red Team: RT-04 - quy trình rollback thống nhất với runbook --> Phase này không thêm migration mới, nên rollback mã = hạ image về tag trước và bỏ tag `v0.1.0`; docs/tests revert theo commit. Quy trình rollback vận hành chung (áp dụng mọi bản phát hành sau) ghi ở `docs/runbook.md`: `pg_dump -Fc` trước deploy có migration, rollback bằng hạ image + `MIGRATE_ON_START=false`, `pg_restore` chỉ khi dữ liệu hỏng |

## Success Criteria

- [ ] Mọi `check(...)` trong `prototype/check-flows.mjs` (trừ mục chỉ-có-ở-prototype đã liệt kê) có assertion tương ứng E01–E32 và toàn bộ xanh trên stack thật.
- [ ] Kịch bản 7.3 (E01–E05) pass gồm cả ba tiêu chí chấp nhận của spec; audit log ghi đủ hành động.
- [ ] H1–H8, H10–H14 pass (H9 đã bỏ, V7) và được ghi bằng output thật trong `plans/reports/hardening-<date>-lms-release.md`; H4 chạy với `APP_ENV=dev` (limiter tắt ở e2e, V2).
- [ ] `playwright.config.ts` có `workers: 1`, `fullyParallel: false`, `retries: 0` (webkit CI `1`), `globalSetup` seed-reset một lần; không test nào gọi seed-reset ngoài `@serial`; không test nào giả định seed có lớp `ended`.
- [ ] `make hardening-immutability` xanh: mọi thao tác ghi lên bản `published` qua repository và qua API bị từ chối (`VERSION_IMMUTABLE`/`INVALID_TRANSITION`), chỉ `published → archived` được phép; schema không có trigger/function. <!-- Updated: Session 2 - D1 -->
- [ ] Không chuỗi mật khẩu tạm, token reset hay hash xuất hiện trong log của `api`, `worker`, `web`, `caddy` (grep cố định theo giá trị thật từ Mailpit), API response, audit payload (H6, H7).
- [ ] Render-check 3 viewport: 0 overflow/clipping/chữ < 12px; computed style của topbar, nav active, nút chính, body, card, input trùng prototype; `pnpm lint:design` 0 vi phạm; hit area ≥ 44px ở 375px; ≤ 1 gradient mỗi trang.
- [ ] axe: 0 vi phạm `serious`/`critical` trên 10 route (ngoại lệ viền input được ghi riêng).
- [ ] Cloudflare R2 cấu hình xong cho production: prod compose không có `minio`, `S3_*` và CSP `media-src` khớp host R2, CORS JSON ghi trong runbook, 6 điểm kiểm tay (presign PUT/GET, Range 206, hết hạn 403, preflight, video 3 trình duyệt) có output trong báo cáo hardening.
- [ ] `README.md`, `docs/{README,architecture,api,database,runbook,design,review}.md` tồn tại và là toàn bộ bộ docs; `docs/api.md` khớp `plan.md` §7 và `router.go`; link và lệnh trong đó chạy được; runbook deploy–backup–restore–rollback (`pg_dump -Fc`, hạ image + `MIGRATE_ON_START=false`) đã diễn tập.
- [ ] `ci.yml` (lint:design + gitleaks) và `e2e.yml` (chromium, a11y, webkit tuần tự) xanh; tag annotated `v0.1.0` tạo sau khi `make check && make e2e && make hardening` xanh; không commit secret, không commit `e2e/.auth/`.
- [ ] Mỗi dòng `plan.md` §3 có bằng chứng được dẫn trong báo cáo phase.
