---
title: "GoUp LMS MVP: fullstack Go + React theo spec và prototype"
description: "Hiện thực 100% spec-lms-mvp.md và design system của prototype bằng monorepo apps/api (Go, Gin, sqlx, go-migrate, cobra) + apps/web (React, Vite, shadcn, Tailwind) + Postgres + Docker, thiết kế database trước, DDD và SOLID ở cả hai tầng."
status: pending
priority: P1
effort: "14 phase (ước lượng ~26 ngày công, xem bảng Phases)"
issue: ""
branch: master
tags: [feature, backend, frontend, database, api, auth, infra, critical]
blockedBy: []
blocks: []
created: 2026-10-05
---

# GoUp LMS MVP: fullstack Go + React theo spec và prototype

## 1. Kết quả mong muốn

Một hệ thống LMS nội bộ chạy được bằng `docker compose`, trong đó:

- Admin tạo chặng, soạn học liệu, phát hành phiên bản, ghép khóa học, áp dụng phiên bản chặng mới cho khóa học bằng một thao tác, tạo lớp, mời học viên qua email.
- Học viên đăng nhập bằng mật khẩu tạm, đổi mật khẩu, học video và markdown, tích hoàn thành.
- Giảng viên xem báo cáo tiến độ lớp mình phụ trách với bộ lọc và sắp xếp.
- Giao diện tái tạo đúng design system của `prototype/` (token, component, bố cục, copy tiếng Việt, hành vi a11y).
- Quy tắc bất biến của phiên bản `published` được bảo vệ hoàn toàn ở tầng ứng dụng (domain + repository với khóa hàng và SQL có điều kiện); tầng database chỉ có ràng buộc khai báo, không function, không trigger (quyết định D1, Validation Session 2). <!-- Updated: Session 2 - D1 -->

Nguồn chân lý: `spec-lms-mvp.md` (FR-01…FR-50, NFR mục 8, kịch bản 7.3) và `prototype/{index.html,goup.css,styles.css,app.js,seed.js}` cùng `docs/design.md`, `docs/prototype.md`.

Bộ dữ liệu mẫu duy nhất (seed, fixture test, MSW, E2E) là `prototype/seed.js` được Phase 02 port sang `lms seed`: chặng `DB`, `DS`, `GO`, `RE`, `WEB` (mỗi chặng v1 `published`), khóa học `BASIC` (v1 `published`), lớp `basic01` (`active`, giảng viên huong.le), `basic02` (`active`, giảng viên bao.pham), `basic03` (`draft`, giảng viên huong.le), 15 tài khoản như seed.js; seed không có lớp `ended` hay khóa học thứ hai, test cần các trạng thái đó tự tạo qua helper `testdb` hoặc API. <!-- Red Team: RT-14 - một bộ fixture -->

## 2. Phạm vi (Scope Challenge: HOLD SCOPE)

Chưa có mã nguồn ứng dụng nào trong repo; chỉ có spec, prototype tĩnh và docs. Yêu cầu của người dùng là toàn bộ spec + toàn bộ design system + DB-first + DDD/SOLID + infra theo mẫu sidecup. Không cắt, không mở rộng.

### Ràng buộc kỹ thuật (do người dùng chốt)

| Lớp | Lựa chọn | Ghi chú |
|---|---|---|
| Backend | Go 1.27, Gin v1.12, sqlx v1.4 trên pgx v5 stdlib, golang-migrate v4 (SQL nhúng), cobra v1.10 | Không dùng ORM |
| Frontend | TypeScript ~6.0, React 19.3, Vite 8, Tailwind CSS 4.3, shadcn CLI v4 với `init -b radix` (gói `radix-ui`), pnpm | react-router v8 data mode + middleware, TanStack Query v5, react-hook-form + zod v4, Vitest 5, MSW 3.0.2 (ghim), Playwright |
| Database | PostgreSQL 16 (image `postgres:16-alpine`) | Chỉ ràng buộc khai báo (PK/UNIQUE/CHECK/FK); không function/trigger (D1) |
| Hạ tầng | Docker multi-stage, compose dev/prod/homelab, Caddy reverse proxy, Makefile gốc, GitHub Actions | Sao chép mẫu từ `~/Documents/personal-workspace/sidecup/infra` và `apps/*/Dockerfile` |
| Tổ chức | Feature-based: `apps/api/internal/features/<feature>`, `apps/web/src/features/<feature>` | DDD: ubiquitous language, entity, value object, aggregate, repository |

### Ngoài phạm vi (non-goals)

- Mọi mục ở spec §3 (thanh toán, tự đăng ký, quiz, điểm danh, chứng chỉ, Excel, auto-tracking, nâng phiên bản lớp `active`, mobile app).
- Điều khiển chỉ có trong bản mẫu: `switch-role`, `demo-login`, `reset-data`, khối `details.guide` hướng dẫn kịch bản trên dashboard, ghi chú "Bản mẫu tĩnh".
- Mời hàng loạt (Q9), lịch theo chặng (Q8), nhắc học viên tự động (P1).
- Nội dung pháp lý PDPA (Q4): chỉ chừa chỗ (trang đăng nhập có liên kết chính sách đọc từ cấu hình `PRIVACY_POLICY_URL`, rỗng thì ẩn).

### Giả định đã chốt cho câu hỏi mở của spec

| Câu hỏi | Giả định trong plan | Có thể đổi ở |
|---|---|---|
| Q1 quy mô | Vài lớp đồng thời, ≤ 100 học viên/lớp, video MP4 progressive ≤ 2 GB | Phase 05, 14 |
| Q2 quyền giảng viên | Chỉ xem lớp được phân công và báo cáo (đúng prototype) | Phase 04 (RBAC), 12 |
| Q3 lớp `ended` | Học viên vẫn xem học liệu, không tích được (đúng prototype) | Phase 08, 13 |
| Q5 kho media | Giao diện `Storage` S3-compatible qua `minio-go`; dev/e2e dùng image MinIO ghim phiên bản (CORS qua env `MINIO_API_CORS_ALLOW_ORIGIN`); prod dùng Cloudflare R2 (`S3_ENDPOINT=<account>.r2.cloudflarestorage.com`, `S3_REGION=auto`, bucket CORS cấu hình trên dashboard R2, ghi ở `docs/runbook.md`); mã nguồn không có nhánh riêng cho R2 <!-- Updated: Validation Session 1 - prod dùng Cloudflare R2 --> | Phase 01, 05, 14 |
| Q6 email | SMTP qua `wneessen/go-mail`; dev dùng Mailpit; prod cấu hình SMTP qua env | Phase 04 |
| Q7 ngưỡng | 72 giờ, ≥ 12 ký tự tạm, ≥ 8 ký tự mới, 3 lần gửi lại email/giờ, khóa 5 lần/15 phút, token reset 30 phút, URL ký media 2 giờ (`MEDIA_URL_TTL=2h`); mọi giá trị đọc từ env với mặc định này; struct `Config` và `.env.example` thuộc Phase 01 <!-- Updated: Validation Session 1 - TTL media giữ 2 giờ; Config thuộc Phase 01 --> | Phase 01 config |
| Chống dò mật khẩu | Khóa theo email (5 lần/15 phút, thông điệp FR-05), không khóa theo IP; limiter IP `RATE_LIMIT_LOGIN_IP_PER_MIN=300`, `RATE_LIMIT_FORGOT_IP_PER_MIN=30`; limiter tắt khi `APP_ENV=e2e`; kiểm tra + ghi thất bại dưới `pg_advisory_xact_lock(hashtext(email_normalized))` <!-- Updated: Validation Session 1 - lockout email + limiter IP cao --> | Phase 04 |
| Bí mật trong email outbox | Mật khẩu tạm / token reset lưu ở cột `secret_enc bytea` mã hóa AES-256-GCM (khóa `OUTBOX_SECRET_KEY`), `payload jsonb` không chứa bí mật; xóa `secret_enc` sau khi gửi thành công hoặc thất bại lần cuối <!-- Updated: Validation Session 1 - mã hóa cột riêng, xóa sau khi gửi --> | Phase 02, 03, 04 |
| Trạng thái thành viên `completed` | Có trong CHECK và value object `MemberStatus`; MVP không có luồng ghi giá trị này <!-- Updated: Validation Session 1 - completed trong CHECK, không có writer --> | Phase 02, 03, 07 |
| Đổi mật khẩu lần đầu | Form 2 ô (mật khẩu mới, nhập lại) đúng prototype; `POST /auth/change-password` bỏ qua `currentPassword` khi user đang `invited`, bắt buộc khi `active` <!-- Updated: Validation Session 1 - first-login 2 ô, API bỏ currentPassword khi invited --> | Phase 04, 10 |
| Lớp `ended`/`draft` với học viên | Trả lộ trình chỉ đọc, không tích được; media vẫn xem được khi lớp `active`/`ended` | Phase 08, 13 |
| Kích hoạt lại tài khoản đã vô hiệu | Về `active` nếu đã đổi mật khẩu, về `invited` nếu `must_change_password` còn `true` | Phase 04 |
| Thành viên `dropped` trong báo cáo | Mặc định ẩn (`includeDropped=false`, đúng `activeMembersOf` của prototype); checkbox "Hiện học viên đã rời lớp" bật `includeDropped=true`, hàng `dropped` có nhãn "Đã rời lớp"; KPI chỉ tính thành viên `active` trừ khi bật <!-- Updated: Validation Session 1 - includeDropped mặc định false, có toggle --> | Phase 09, 12 |
| FR-17 nhiều khóa học | Mỗi khóa học một giao dịch riêng; trong một khóa học là atomic đúng spec; kết quả trả theo từng khóa | Phase 06 |

## 3. Tiêu chí chấp nhận toàn plan

- [ ] `make dev` khởi động Postgres, MinIO, Mailpit, API, Web; `make check` chạy lint, typecheck, unit test, integration test xanh.
- [ ] Mọi FR trong spec §6 có ít nhất một test tự động (unit/integration ở API hoặc E2E ở web) được liệt kê trong phase tương ứng.
- [ ] Mọi assertion của `prototype/check-flows.mjs` được chuyển sang Playwright (Phase 14), trừ các assertion thuộc điều khiển chỉ có trong bản mẫu (`details.guide`, `reset-data`, `switch-role`/`?as=`) đã nằm trong non-goals.
- [ ] Kịch bản 7.3 chạy xanh end-to-end bằng Playwright: nhân bản Database v1 → v2, sửa, phát hành, áp dụng cho Basic (một giao dịch), basic01/basic02 giữ v1, basic03 dùng v2, cảnh báo FR-18 biến mất.
- [ ] Thử sửa học liệu của phiên bản `published` qua API (`PATCH /lessons/{id}`) và qua repository gọi trực tiếp (bỏ qua service) đều bị từ chối với `409 VERSION_IMMUTABLE` / `ErrVersionImmutable`; không có đường ghi nào trong ứng dụng vượt qua được. <!-- Updated: Session 2 - D1 -->
- [ ] Khi `must_change_password = true`, mọi API ngoài allowlist trả `403 PASSWORD_CHANGE_REQUIRED`.
- [ ] Mật khẩu tạm và token không xuất hiện trong log, response Admin, hay audit payload.
- [ ] Render-check 3 viewport (1440×900, 768×1024, 375×812) của web khớp prototype về token màu, chữ, khoảng cách, radius; thêm viewport 320px chỉ kiểm `scrollWidth ≤ innerWidth`.
- [ ] Giao diện không có hex thô ngoài file token, không emoji icon, không chữ < 12px, mọi hit area ≥ 44px (đúng `docs/design.md`).

## 4. Ngôn ngữ chung (Ubiquitous Language)

Dùng nhất quán trong tên bảng, package, type, route, label UI. Tiếng Việt cho UI, tiếng Anh cho mã.

| Thuật ngữ (UI) | Khái niệm | Mã (Go / TS / SQL) | Ghi chú |
|---|---|---|---|
| Chặng | Đơn vị nội dung dùng lại được giữa khóa học | `Stage` / `stages` | Aggregate root nhẹ: `code`, `name` |
| Phiên bản chặng | Bản phát hành bất biến của một chặng | `StageVersion` / `stage_versions` | Aggregate root; chứa `Lesson` |
| Học liệu | Video hoặc markdown trong một phiên bản chặng | `Lesson` / `lessons` | Entity trong aggregate `StageVersion`; `lesson_key` nối giữa phiên bản |
| Khóa học | Chuỗi chặng có thứ tự | `Course` / `courses` | |
| Phiên bản khóa học | Bản phát hành bất biến gồm danh sách phiên bản chặng | `CourseVersion` / `course_versions` + `course_version_stages` | Aggregate root |
| Áp dụng phiên bản chặng | FR-17: nhân bản khóa, thay chặng, phát hành trong một giao dịch | `ApplyStageVersion` | Use case trong feature `courses` |
| Lớp | Một đợt chạy của một phiên bản khóa học | `Class` / `classes` | Aggregate root; chứa `ClassMember` |
| Thành viên lớp | Học viên trong lớp | `ClassMember` / `class_members` | `MemberStatus`: `active` / `dropped` / `completed` (`completed` chưa có luồng ghi ở MVP) |
| Lời mời | Việc gửi email mời hoặc thông báo | `Invitation` / `invitations` | Trạng thái gửi: `queued` / `sent` / `failed` |
| Tiến độ | Cặp (thành viên, học liệu) với mốc mở và tích | `LessonProgress` / `lesson_progress` | % luôn tính, không lưu |
| Nháp / Đã phát hành / Đã lưu trữ | Vòng đời phiên bản | `VersionStatus`: `draft` / `published` / `archived` | Value object có bảng chuyển trạng thái |
| Nháp / Đang chạy / Đã kết thúc | Vòng đời lớp | `ClassStatus`: `draft` / `active` / `ended` | Một chiều |
| Admin / Giảng viên / Học viên | Vai trò | `Role`: `admin` / `teacher` / `student` | |
| Chưa đăng nhập / Đã kích hoạt / Vô hiệu hóa | Trạng thái tài khoản | `UserStatus`: `invited` / `active` / `disabled` | |
| Mật khẩu tạm | Mật khẩu sinh ngẫu nhiên khi mời, hết hạn 72 giờ | `TemporaryPassword` | Chỉ lưu hash |
| Bắt buộc | Học liệu tính vào % | `required` | |

Copy tiếng Việt: dùng đúng chuỗi trong `prototype/app.js` khi prototype có. Các thông điệp mới không có trong prototype (chốt ở đây để backend và frontend dùng chung): "Bạn đã nhập sai quá nhiều lần. Thử lại sau 15 phút.", "Mã đã tồn tại.", "Phiên bản đã phát hành không thể chỉnh sửa.", "Chỉ đổi phiên bản khi lớp còn nháp.", "Giảng viên không hợp lệ hoặc đã bị vô hiệu hóa.", "Lớp chưa bắt đầu hoặc đã kết thúc; không ghi nhận tiến độ.", "Tham số lọc không hợp lệ.", "Liên kết đặt lại mật khẩu không hợp lệ hoặc đã hết hạn.", "Chưa nhận được file. Tải lên lại.", "Không phát được video. Tải lại trang để thử lại.", "Đã rời lớp" (nhãn thành viên `dropped` trong báo cáo).

## 5. Kiến trúc

### 5.1 Cây monorepo

```text
lms/
├── Makefile                      # dev, check-ports, migrate, seed, test, lint, build, e2e, prod-up (mẫu sidecup)
├── .github/workflows/{ci.yml,e2e.yml}
├── infra/
│   ├── .env.example
│   ├── docker-compose.yml        # postgres, minio, mailpit; profile full: api, web
│   ├── docker-compose.prod.yml   # + caddy 80/443, không mở port nội bộ
│   ├── docker-compose.homelab.yml# overlay Traefik
│   └── caddy/Caddyfile           # security headers, /api/* -> api:8080, fallback -> web:80
├── apps/api/
│   ├── cmd/lms/                  # cobra: root, serve, migrate (up|down|version), seed (--reset, --upload-sample), worker, healthcheck  <!-- Updated: Validation Session 1 - bỏ routes --json, outbox list|retry -->
│   ├── internal/app/             # router.go (điểm nối duy nhất), deps.go
│   ├── internal/domain/          # shared kernel: email.go, version_status.go, version_no.go, code.go, percent.go, lesson_key.go, errors.go
│   ├── internal/features/<feature>/   # entity.go, service.go, repository.go, repository_pg.go, handler.go, dto.go, *_test.go
│   ├── internal/platform/        # apperr, clock, config, db (sqlx + tx + pgerr + constraints), httpx, ids, middleware, mail, storage, secretbox, audit, testdb
│   ├── migrations/               # NNNN_<slug>.up.sql / .down.sql, //go:embed
│   ├── Dockerfile, Makefile, go.mod, .golangci.yml
└── apps/web/
    ├── src/app/                  # main.tsx, router.tsx, providers.tsx, query-client.ts, middleware.ts (auth + role), route-announcer.tsx, layouts/{AuthLayout,AppShell}.tsx
    ├── src/features/<feature>/   # api.ts, model.ts, components/, pages/, hooks/, routes.tsx (export routes; reports export thêm teachRoutes), index.ts (public API duy nhất)
    ├── src/shared/               # api/ (http.ts, errors.ts), ui/ (shadcn), lib/, layout/, hooks/, domain/ (Percent, statuses)
    ├── src/styles/               # goup-tokens.css (chép nguyên :root của prototype/goup.css), app.css (@theme inline + @layer base)
    ├── e2e/                      # Playwright
    ├── Dockerfile, nginx.conf, package.json, vite.config.ts, components.json, eslint.config.js
```

Features backend: `identity`, `mailer`, `media`, `stages`, `courses`, `classes`, `learning`, `reports`. Features frontend: `auth`, `dashboard`, `stages`, `courses`, `classes`, `reports`, `learning`.

### 5.2 Phân lớp backend trong mỗi feature (DDD + SOLID)

| File | Vai trò | Nguyên tắc |
|---|---|---|
| `entity.go` | Entity, value object riêng của feature, hành vi nghiệp vụ (`sv.Publish(now)`, `cls.Activate()`), lỗi domain | Bất biến nằm trong aggregate; field unexported, constructor kiểm tra; `Rehydrate*` để repo dựng lại không chạy validate |
| `repository.go` | Interface repository theo aggregate; nhận `db.Executor` (`sqlx.ExtContext`) nên chạy được trên `*sqlx.DB` hoặc `*sqlx.Tx` | DIP: service phụ thuộc interface |
| `repository_pg.go` | Hiện thực sqlx, map `pgconn.PgError` → lỗi domain tại một chỗ (gói `platform/db/pgerr`: `constraints.go` ở Phase 02, `map.go` với `pgerr.Map(err)` ở Phase 03; chỉ còn 23505/23503/23514, không còn mã lỗi tự định nghĩa). Mọi UPDATE/DELETE/INSERT lên aggregate có phiên bản mang điều kiện trạng thái và đi qua `db.ExecAffectOne`; 0 dòng → `ErrVersionImmutable` / `ErrInvalidTransition` <!-- Updated: Session 2 - D1 --> | SRP |
| `service.go` | Use case; nhận `db.Tx` (unit of work `Transact(ctx, func(Executor) error)`), `clock.Clock`, `audit.Recorder`, interface của feature khác | OCP: thêm use case không sửa handler cũ |
| `handler.go` | Gin handler, chỉ bind/validate DTO, gọi service, trả `httpx.OK/Fail` | ISP: `Register(r gin.IRouter, mw ...)` |
| `dto.go` | Request/response struct với tag `json` + `binding` | |

Mẫu thiết kế áp dụng: Repository, Unit of Work (`Transact`), State machine cho `VersionStatus`/`ClassStatus`, Strategy cho `LessonContent` (`VideoContent` / `MarkdownContent` cùng interface `Validate()`), Factory cho clone (`StageVersion.CloneAsDraft(nextNo)`), Outbox cho email, Decorator middleware (auth → must-change-password → role), struct `reports.Filter` + `Apply(qb)` cho bộ lọc báo cáo (không dùng generic `Specification[T]`). Không giấu `*sqlx.Tx` trong `context`. <!-- Updated: Validation Session 1 - bỏ Specification generic -->

Quy tắc ghi cho aggregate có phiên bản (áp dụng ở `stages`, `courses`): hàng con (`lessons`, `lesson_media`, `course_version_stages`) chỉ được ghi khi header còn `draft`, và điều kiện đó nằm **trong câu SQL** (`INSERT ... SELECT ... FROM <header> WHERE id=$1 AND status='draft'`, `UPDATE/DELETE ... WHERE EXISTS (header draft)`); `SaveDraft` ghi con trước, header sau với `WHERE status='draft'` (không đổi `status`); `TransitionStatus(id, from, to)` chỉ `UPDATE` header với `WHERE status = from` (publish chặng thêm `AND NOT EXISTS (markdown chưa render)`); mọi câu đi qua `db.ExecAffectOne`, 0 dòng → `ErrVersionImmutable`/`ErrInvalidTransition`; mọi use case ghi mở `Transact` và đọc header bằng `ByIDForUpdate` trước. Publish render `markdown_html` khi còn nháp rồi mới chuyển trạng thái; FR-17 chèn header nháp → chèn con → chuyển `published`. Hợp đồng đầy đủ: Phase 02 mục "Bất biến phiên bản ở tầng ứng dụng". <!-- Red Team: RT-01 - thứ tự ghi --> <!-- Updated: Session 2 - D1 -->

Giao diện liên feature (định nghĩa ở feature tiêu thụ, adapter nối ở `internal/app/deps.go`, tránh import vòng): `identity.UserProvisioner`, `identity.UserReader`, `identity.ActivityRecorder` (classes, learning dùng); `mailer.Enqueuer`, `mailer.OutboxStatusReader` (identity, classes dùng); `media.URLSigner`, `media.AccessChecker` (stages, learning dùng); `stages.StageVersionReader`, `stages.OutdatedReader` (courses, reports dùng); `courses.VersionReader` (classes dùng); `classes.MembershipReader` (learning, reports dùng); `learning.ProgressReader` (reports, classes dùng); `audit.Recorder` + `audit.Reader.Latest(n)` (reports/dashboard dùng).

### 5.3 Phân lớp frontend

| Thư mục | Vai trò |
|---|---|
| `shared/domain` | Value object thuần TS: `Percent`, `VersionStatus` (+ `canTransition`), `ClassStatus`, `Role`, nhãn tiếng Việt `STATUS_VI` |
| `features/<f>/model.ts` | Type theo API + hàm thuần (`computeProgress`, `isOutdated`), không gọi mạng |
| `features/<f>/api.ts` | Hàm fetch typed dùng `shared/api/http.ts`, query keys |
| `features/<f>/hooks` | TanStack Query hooks (query + mutation + invalidate) |
| `features/<f>/components` | Component trình bày, nhận props, không gọi hook dữ liệu |
| `features/<f>/pages` | Lắp ráp, đọc route params/search params |
| `shared/ui` | shadcn đã chỉnh `cva` theo token GoUp. Button: `variant: default (navy) | gradient (hành động chính, tối đa một nút mỗi trang) | outline (= .un-btn-secondary) | ghost | destructive`, `size: default | sm | icon`. Badge/Alert đặt tên variant theo hình thức; ánh xạ trạng thái nghiệp vụ → variant nằm trong `features/*/model` |

Quy tắc sở hữu: chỉ Phase 10 sửa `src/app`, `src/shared`, `src/styles`; Phase 11–13 chỉ thêm dưới `src/features/<f>` và đăng ký route qua `routes.tsx` mà `app/router.tsx` import. Component dùng class tiện ích Tailwind trỏ tới token (`bg-navy-700`), không hex thô. Tab là `<nav aria-label>` + `NavLink`, không dùng Radix Tabs (giữ URL `?tab=`). Dialog = shadcn Dialog, drawer học viên = Sheet, toast = Sonner (4 giây, tối đa 3, nút đóng).

### 5.4 Hạ tầng và vận hành

- Compose dev: `postgres:16-alpine` (healthcheck `pg_isready`), `minio/minio` ghim tag với `MINIO_API_CORS_ALLOW_ORIGIN=http://localhost:5173,http://localhost:8081` + `minio-init` chạy `mc alias set`, `mc mb --ignore-existing`, `mc anonymous set none` (MinIO community không hỗ trợ `mc cors set`), `axllent/mailpit` (1025/8025). Profile `full` thêm `api` (healthcheck `["/lms","healthcheck"]`, `MIGRATE_ON_START=true`, `depends_on postgres: service_healthy`), `worker` (cùng image, `command: ["worker"]`, `depends_on api: service_healthy`) và `web` (8081:80) cho E2E. <!-- Red Team: RT-04/RT-08 - service worker, healthcheck, CORS qua env -->
- Compose prod và homelab: `postgres`, `api`, `worker`, `web`, `caddy`; không có MinIO (media ở Cloudflare R2 qua env `S3_*`); không publish port nội bộ; `caddy:2-alpine` 80/443 với header HSTS, nosniff, Referrer-Policy, `X-Frame-Options DENY`, Permissions-Policy, CSP (`img-src 'self' blob:`, `media-src 'self' {$MEDIA_ORIGIN}`, `connect-src 'self' {$MEDIA_ORIGIN}`; `MEDIA_ORIGIN` là biến compose-only của Caddy, origin của `S3_PUBLIC_ENDPOINT`, Phase 01 sở hữu), `-Server`; `/api/*` → `api:8080`; `/internal/*` → 404; còn lại → `web:80`.
- Dockerfile api: `golang:1.27-alpine` build → `gcr.io/distroless/static-debian12:nonroot`. Dockerfile web: `node:22-alpine` + pnpm → `nginx:1.27-alpine` với `nginx.conf` SPA fallback và cache asset.
- CI (đúng tập việc của sidecup, không thêm govulncheck/pnpm audit): `go vet`, `golangci-lint v2`, `go test ./...`, `go test -tags integration` với service postgres; `pnpm lint`, `typecheck`, `test`, `build`; `gitleaks`. Workflow E2E riêng dựng profile `full` rồi chạy Playwright (`workers: 1`; job webkit tách riêng, chạy tuần tự). <!-- Updated: Validation Session 1 - CI chỉ gồm việc có ở sidecup -->
- Cấu hình chỉ qua env (`caarlos0/env/v11`), log JSON `slog`, không log body, mật khẩu, token. Struct `Config` và `infra/.env.example` do Phase 01 sở hữu; các phase khác chỉ tham chiếu đúng tên trường. Danh sách env chuẩn: `APP_ENV` (bắt buộc, allowlist `dev|e2e|production`), `HTTP_ADDR=:8080`, `PUBLIC_BASE_URL`, `DATABASE_URL`, `MIGRATE_ON_START=false`, `SESSION_TTL=12h`, `COOKIE_SECURE=false` (nguồn duy nhất quyết định cookie `sid` hay `__Host-sid`), `TRUSTED_PROXIES=` (rỗng; prod đặt CIDR của Caddy), `PASSWORD_MIN_LENGTH=8`, `TEMP_PASSWORD_TTL=72h`, `RESET_TOKEN_TTL=30m`, `LOGIN_MAX_FAILURES=5`, `LOGIN_LOCK_WINDOW=15m`, `RATE_LIMIT_LOGIN_IP_PER_MIN=300`, `RATE_LIMIT_FORGOT_IP_PER_MIN=30`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `MAIL_FROM`, `MAIL_FAIL_PATTERN=` (chỉ có hiệu lực khi `APP_ENV=e2e`), `EMAIL_POLL_INTERVAL=5s`, `OUTBOX_SECRET_KEY` (bắt buộc, base64 32 byte), `S3_ENDPOINT`, `S3_PUBLIC_ENDPOINT`, `S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_USE_SSL`, `MEDIA_URL_TTL=2h`, `MAX_VIDEO_BYTES=2147483648`, `MAX_IMAGE_BYTES=10485760`, `STALE_DAYS=7` (ngưỡng "lâu không hoạt động"), `SEED_PASSWORD` (chỉ seed), `LOG_LEVEL=info`. Test của Phase 01: mọi khóa trong `.env.example` phải có trong struct. `lms seed --reset` chỉ chạy khi `APP_ENV` là `dev` hoặc `e2e`. <!-- Red Team: RT-11 - một nguồn cấu hình -->
- Rate limit dùng `x/time/rate` trong tiến trình (IP cho `/auth/login`, `/auth/forgot-password`; 120/phút mỗi user cho `/media/*`); giới hạn này chỉ đúng khi chạy một instance API (chấp nhận cho MVP, ghi vào runbook). Khóa đăng nhập theo email đọc từ `login_attempts` dưới advisory lock. <!-- Red Team: RT-13/RT-15 - phạm vi limiter -->
- CSRF: `http.CrossOriginProtection` (Sec-Fetch-Site/Origin) và bắt buộc header `X-Requested-With: fetch` trên mọi POST/PUT/PATCH/DELETE dưới `/api/v1`; `shared/api/http.ts` luôn gửi header này. PUT thẳng lên storage (MinIO/R2) không qua API nên không cần header. <!-- Red Team: RT-05 - hợp đồng CSRF -->
- Migrate: `Migrate(dsn)` mở `*sql.DB` riêng cho golang-migrate và đóng sau khi chạy, không dùng pool của ứng dụng; `migrate down` về rỗng báo `ErrNilVersion`. Rollback prod: `pg_dump -Fc` trước mọi lần deploy có migration; hạ image với `MIGRATE_ON_START=false`; migration viết theo expand/contract. <!-- Red Team: RT-04 - migrate và rollback -->

## 6. Mô hình dữ liệu (chi tiết ở Phase 02)

Thứ tự migration: `0000_init` (no-op để `//go:embed` không rỗng), `0001_schema_conventions`, `0002_users_sessions`, `0003_media_files`, `0004_stages_versions_lessons`, `0005_courses_versions`, `0006_classes_members_invitations`, `0007_lesson_progress`, `0008_email_outbox_password_resets_login_attempts`, `0009_audit_logs`. Không có migration nào chứa `CREATE FUNCTION|PROCEDURE|TRIGGER|TYPE|EXTENSION` (D1). <!-- Updated: Session 2 - D1 -->

| Bảng | Khóa / ràng buộc chính |
|---|---|
| `users` | `email_normalized` UNIQUE, `role`, `status`, `password_hash`, `must_change_password`, `temp_password_expires_at`, `last_login_at`, `last_active_at`, `disabled_at` |
| `sessions` | `token_hash` UNIQUE, `user_id` FK, `expires_at`, `last_seen_at` |
| `password_reset_tokens` | `token_hash` UNIQUE, `user_id`, `expires_at`, `used_at` |
| `login_attempts` | PK `bigserial` (bảng append-only, ngoại lệ duy nhất của quy ước uuid), `email_normalized`, `ip`, `succeeded`, `attempted_at`; index theo (`email_normalized`, `attempted_at`); worker xóa hàng cũ hơn 30 ngày <!-- Updated: Validation Session 1 - bỏ index IP, không khóa theo IP --> |
| `media_files` | `storage_key` UNIQUE, `kind` (`video`/`image`), `content_type`, `size_bytes`, `status` (`pending`/`ready`), `uploaded_by` |
| `stages` | `code` UNIQUE |
| `stage_versions` | UNIQUE(`stage_id`,`version_no`); partial UNIQUE(`stage_id`) WHERE `status='draft'`; `cloned_from_id` FK; `published_at` |
| `lessons` | FK `stage_version_id` ON DELETE CASCADE (xóa theo nháp); UNIQUE(`stage_version_id`,`lesson_key`) (`lesson_key` tối đa 40 ký tự); UNIQUE(`stage_version_id`,`position`) DEFERRABLE; `type` CHECK; `required`; `duration_seconds int NULL CHECK >= 0`; `markdown_source`, `markdown_html` (đã lọc), `video_media_id` FK RESTRICT; CHECK loại-nội dung <!-- Red Team: RT-07 - duration_seconds --> |
| `lesson_media` | PK(`lesson_id`,`media_id`); FK `lesson_id` ON DELETE CASCADE, FK `media_id` ON DELETE RESTRICT; ghi lại khi lưu học liệu từ `video_media_id` và `<img src>` trong HTML đã render; là nguồn kiểm tra quyền truy cập media <!-- Red Team: RT-15 - bảng nối lesson_media --> |
| `courses` | `code` UNIQUE |
| `course_versions` | UNIQUE(`course_id`,`version_no`); partial UNIQUE một nháp; `cloned_from_id`; `published_at` |
| `course_version_stages` | PK(`course_version_id`,`stage_version_id`); UNIQUE(`course_version_id`,`stage_id`) (`stage_id` denormalized, app điền từ `StageVersionRef.StageID`; FK ghép `fk_cvs_stage_version (stage_version_id, stage_id) → stage_versions(id, stage_id)` báo 23503 khi lệch, không trigger <!-- Updated: Session 2 - D1 -->); UNIQUE(`course_version_id`,`position`) DEFERRABLE; FK `stage_version_id` ON DELETE RESTRICT |
| `classes` | `code` UNIQUE (regex `^[a-z0-9][a-z0-9-]{1,29}$`); FK `course_version_id` ON DELETE RESTRICT; `status` CHECK IN (`draft`,`active`,`ended`); `start_date`, `end_date` CHECK end > start; `teacher_id` FK <!-- Red Team: RT-03 - enum lớp --> |
| `class_members` | UNIQUE(`class_id`,`user_id`); `status` CHECK IN (`active`,`dropped`,`completed`); `joined_at`, `dropped_at` <!-- Updated: Validation Session 1 - completed trong CHECK --> |
| `invitations` | FK `class_id`, `user_id`, `sent_by`; `kind` (`invite`/`added`/`resend`); cột `email_outbox_id` tạo ở 0006, FK thêm ở 0008 sau khi có `email_outbox`; trạng thái đọc qua outbox |
| `lesson_progress` | PK(`class_member_id`,`lesson_id`); `first_opened_at` NOT NULL; `completed_at` NULL |
| `email_outbox` | `to_email`, `template` CHECK IN (`invite`,`added`,`resend`,`password_reset`), `payload jsonb` (không chứa bí mật; worker render HTML lúc gửi), `secret_enc bytea NULL` (AES-256-GCM, NULL sau khi `sent` hoặc `failed` lần cuối), `status` CHECK IN (`queued`,`sending`,`sent`,`failed`), `attempts` (chỉ tăng ở `MarkFailedAttempt`, tối đa 3), `run_at`, `locked_until`, `last_error`, `sent_at`; index `(run_at) WHERE status IN ('queued','sending')`; claim: `(status='queued' AND run_at<=now()) OR (status='sending' AND locked_until<now())` với `FOR UPDATE SKIP LOCKED` <!-- Red Team: RT-02 - hợp đồng outbox duy nhất --> |
| `audit_logs` | `actor_id`, `action`, `target_type`, `target_id`, `before jsonb`, `after jsonb`, `at` |

Bất biến phiên bản ở tầng ứng dụng (D1, thay cho bộ trigger của bản plan trước): header `published`/`archived` không sửa/xóa và chỉ `published → archived` (domain `ErrVersionImmutable`/`ErrInvalidTransition` + `UPDATE ... WHERE status='draft'` / `WHERE status=$from`); bảng con chỉ ghi khi cha `draft` (SQL có điều kiện trên header cha); không publish chặng khi còn `markdown_html IS NULL` (domain `ErrNotRendered` + `AND NOT EXISTS` trong câu transition <!-- Red Team: RT-01 - guard publish -->); `stage_id` của `course_version_stages` khớp qua FK ghép. DB giữ `ck_*_published_at`, `ck_*_archived_at` ở mức hàng. Chi tiết và câu SQL mẫu: Phase 02 mục "Bất biến phiên bản ở tầng ứng dụng"; `migrations_test.go` chứng minh `pg_trigger`/`pg_proc`/`pg_type`/`pg_extension` của schema rỗng. <!-- Updated: Session 2 - D1 --> ID dùng `uuid` (v7 sinh ở ứng dụng qua `platform/ids`). Tên ràng buộc (`uq_users_email_normalized`, `uq_stages_code`, `uq_courses_code`, `uq_classes_code`, `uq_stage_versions_one_draft`, `uq_course_versions_one_draft`, `uq_lessons_version_position`, `uq_lessons_version_key`, `uq_cvs_version_stage`, `uq_cvs_version_position`, `uq_class_members_class_user`, `ck_*`, `fk_*`) là hằng Go trong `platform/db/pgerr/constraints.go` (Phase 02), có integration test so với `pg_constraint`; feature chỉ dùng hằng, không gõ chuỗi. <!-- Red Team: RT-11 - registry tên ràng buộc -->

## 7. Bề mặt API (`/api/v1`, JSON, cookie session)

Envelope lỗi: `{"error":{"code":"...","message":"...","details":{...}}}`. Mã lỗi chuẩn: `VALIDATION_FAILED`, `UNAUTHENTICATED`, `PASSWORD_CHANGE_REQUIRED`, `FORBIDDEN`, `NOT_FOUND`, `CONFLICT`, `VERSION_IMMUTABLE`, `DRAFT_EXISTS`, `IN_USE`, `INVALID_TRANSITION`, `TEMP_PASSWORD_EXPIRED`, `ACCOUNT_DISABLED`, `TOO_MANY_ATTEMPTS`, `RATE_LIMITED`. Message tiếng Việt lấy đúng copy trong `prototype/app.js`. Mọi mutation yêu cầu header `X-Requested-With: fetch` (RT-05). Hợp đồng DTO đầy đủ nằm ở §7.1; phase cung cấp và phase tiêu thụ đều trích nguyên văn từ đó.

| Method & path | Feature | FR | Vai trò |
|---|---|---|---|
| `POST /auth/login`, `POST /auth/logout`, `GET /auth/me` | identity | FR-03, FR-05 | public / mọi vai trò |
| `POST /auth/change-password` | identity | FR-03 | đã đăng nhập (allowlist khi must-change) |
| `POST /auth/forgot-password`, `POST /auth/reset-password` | identity | FR-04 | public, rate limit |
| `GET /users?role=teacher`, `POST /users/{id}/disable`, `POST /users/{id}/enable` | identity | FR-06 | admin |
| `POST /media/uploads` body `{kind, fileName, contentType, sizeBytes}` → `201 {mediaId, uploadUrl, expiresAt}`, `POST /media/uploads/{id}/complete` → `200 MediaDTO`, `GET /media/{id}/url` → `{url, expiresAt}`, `GET /media/{id}/content` (302 tới URL ký, dùng cho `<img>` trong markdown) <!-- Red Team: RT-07 - body upload --> | media | FR-11 | admin (upload); người có quyền với học liệu chứa media qua `lesson_media` (url/content) |
| `GET/POST /stages`, `GET /stages/{id}` (kèm versions, outdated courses) | stages | FR-10, FR-18 | admin |
| `POST /stage-versions/{vid}/clone`, `/publish`, `/archive`, `DELETE /stage-versions/{vid}` (chỉ nháp; là phiên bản cuối thì xóa luôn chặng) <!-- Red Team: RT-07 - route phẳng --> | stages | FR-12, FR-13, FR-19 | admin |
| `GET /stage-versions/{vid}`, `POST /stage-versions/{vid}/lessons`, `PATCH/DELETE /stage-versions/{vid}/lessons/{lid}`, `PUT /stage-versions/{vid}/lessons/order` | stages | FR-11 | admin |
| `GET/POST /courses`, `GET /courses/{id}` | courses | FR-14 | admin |
| `GET /course-versions/{vid}`, `PUT /course-versions/{vid}/stages` body `{stageVersionIds:[...]}`, `POST /course-versions/{vid}/clone`, `/publish`, `/archive`, `DELETE /course-versions/{vid}` (chỉ nháp; là phiên bản cuối thì xóa luôn khóa học) <!-- Red Team: RT-07 - route phẳng --> | courses | FR-14…16, FR-19 | admin |
| `POST /stage-versions/{vid}/apply` body `{courseIds:[...]}` → `200 {results:[{courseId, courseCode, newVersionNo?, error?: {code, message}}]}` (handler thuộc feature `courses`; web hiện kết quả từng dòng, chỉ toast thành công khi mọi dòng ok) | courses | FR-17 | admin |
| `GET/POST /classes`, `GET /classes/{id}`, `PATCH /classes/{id}` (name, dates, teacher, courseVersionId khi draft) | classes | FR-20, FR-21 | admin |
| `POST /classes/{id}/activate`, `POST /classes/{id}/end` | classes | FR-22 | admin |
| `GET /classes/{id}/members?includeDropped=` (mặc định false; tab Thành viên của web luôn gọi `includeDropped=true` vì prototype `membersOf` hiện cả hàng `dropped` mờ với nhãn "Đã rời lớp"; báo cáo dùng mặc định false), `POST /classes/{id}/invitations` body `{email, fullName}` → `201 {kind: "invited"|"added", member: MemberDTO}`, `POST /classes/{id}/members/{mid}/resend` → `200 {member}` (429 `RATE_LIMITED` sau 3 lần/giờ), `DELETE /classes/{id}/members/{mid}` → `200 MemberDTO` <!-- Red Team: RT-10 - hợp đồng lời mời --> | classes | FR-01, FR-02, FR-23 | admin |
| `GET /me/classes`, `GET /me/classes/{id}` (lộ trình + tiến độ; lớp `draft`/`ended` trả lộ trình chỉ đọc kèm `readOnlyReason`), `GET /me/classes/{id}/lessons/{lid}` (ghi first_opened_at), `PUT /me/classes/{id}/lessons/{lid}/completion` body `{completed:bool}` | learning | FR-30…33 | student |
| `GET /classes/{id}/report?notLoggedIn=&inactiveDays=&belowPercent=&includeDropped=&sort=` (`includeDropped` mặc định false; kèm `selfReported: true`), `GET /classes/{id}/report/members/{mid}` (`mid` = `class_members.id`, mọi truy vấn kèm `cm.class_id = {id}`, lệch lớp → 404) <!-- Red Team: RT-15 - phạm vi báo cáo thành viên --> | reports | FR-40, FR-41 | admin, teacher của lớp |
| `GET /teach/classes` → `{items:[{id, code, name, status, startDate, endDate, courseName, courseVersionNo, memberCount, avgPercent, notLoggedIn, inactiveOver7Days}]}` (ngưỡng `STALE_DAYS`) | classes | FR-40 | teacher |
| `GET /dashboard` → `{kpis:{stages, courses, classes, students}, hints:{draftClasses, notLoggedIn, outdatedCourses, failedInvites}, outdated:[{courseId, courseCode, courseName, stageId, stageCode, stageName, currentVersionNo, latestPublishedNo}], classes:[{id, code, name, status, courseName, courseVersionNo, memberCount, avgPercent}], recentActivity:[{id, at, actorName, actionLabel, target:{type, id, label}, summary}]}` (mục "Nhật ký thao tác") | reports | US-7 | admin |
| `GET /healthz`, `GET /readyz` | platform | NFR | public |

Audit actions (`audit_logs.action`): `stage_version.published`, `stage_version.cloned`, `stage_version.archived`, `stage_version.deleted`, `course_version.published`, `course_version.cloned`, `course_version.archived`, `course_version.deleted`, `course.stage_version_applied`, `class.course_version_changed`, `class.activated`, `class.ended`, `class.member_invited`, `class.invitation_resent`, `class.member_dropped`, `user.disabled`, `user.enabled`, `stage.created`, `course.created`, `class.created`. Nhãn tiếng Việt của từng action chép nguyên văn từ bảng nhãn audit trong `prototype/app.js`. <!-- Red Team: RT-07 - audit tạo mới -->

### 7.1 Hợp đồng dữ liệu chốt (nguồn duy nhất cho phase cung cấp và phase tiêu thụ) <!-- Red Team: RT-06/RT-07/RT-10 - DTO chuẩn -->

- `LessonDTO` (admin): `{id, lessonKey, title, type: 'markdown'|'video', required, position, durationSeconds?, markdownSource?, videoMediaId?, videoFileName?}`.
- `GET /stages` → `{items:[{id, code, name, description?, latestVersionNo, latestPublishedNo?, draftVersionId?, versions:[{id, versionNo, status, publishedAt?, lessonCount}], usedByCourseCount, outdatedCourseCount}]}`; `GET /stages/{id}` thêm `usedBy:[{courseId, courseCode, courseName, courseVersionId, versionNo, stageVersionNo, outdated, classCodes:[...]}]` và `outdatedCourses:[{courseId, courseCode, courseName, currentVersionNo, latestPublishedNo, canApply, blockedReason?}]` (FR-18); `POST /stages {code, name, description?}` tạo kèm nháp v1.
- `GET /stage-versions/{vid}` → `{id, stageId, stageCode, stageName, versionNo, status, publishedAt?, clonedFromVersionNo?, lessons:[LessonDTO], usedBy:[...]}`; `POST /stage-versions/{vid}/lessons` body `{lessonKey, title, type, required, durationSeconds?, markdownSource?, videoMediaId?}` → `201 LessonDTO`; `PUT /stage-versions/{vid}/lessons/order {lessonIds:[...]}`.
- `GET /courses` → `{items:[{id, code, name, latestVersionNo, latestPublishedNo?, draftVersionId?, versions:[{id, versionNo, status, publishedAt?, stageCount, outdatedStageCount}], classesUsing:[{classId, code, name, versionNo}]}]}`; `POST /courses {code, name, description?}` tạo kèm nháp v1.
- `GET /course-versions/{vid}` → `{id, courseId, courseCode, courseName, versionNo, status, publishedAt?, stages:[{position, stageId, stageCode, stageName, stageVersionId, stageVersionNo, lessonCount, requiredCount, latestPublishedNo?, outdated}], classes:[{classId, code, name, status, memberCount}], newerPublished}`.
- `DELETE /stage-versions/{vid}` → `200 {stageDeleted: bool}`; `DELETE /course-versions/{vid}` → `200 {courseDeleted: bool}` (web điều hướng về danh sách khi `true`).
- `MemberDTO`: `{id (class_members.id), userId, email, fullName, accountStatus: 'invited'|'active'|'disabled', memberStatus: 'active'|'dropped'|'completed', tempPasswordExpiresAt?, inviteStatus?: 'queued'|'sent'|'failed' (vắng khi chưa có hàng outbox; UI hiện "—"), inviteKind?: 'invite'|'added'|'resend', inviteAttempts?, inviteLastError?, invitedAt?, lastLoginAt?, lastActiveAt?, joinedAt, droppedAt?}` (các field `invite*` phụ phục vụ dòng "Gửi thất bại · {lastError} · {attempts} lần thử" của prototype; không field nào chứa mật khẩu).
- Lỗi của `POST /classes/{id}/invitations` (copy theo prototype, mã HTTP theo RT-10): 422 `VALIDATION_FAILED` ("Email không hợp lệ." / "Nhập họ tên học viên."), 409 `INVALID_TRANSITION` "Không mời được vào lớp đã kết thúc.", 403 `FORBIDDEN` "Email này thuộc tài khoản nội bộ, không mời làm học viên được.", 409 `ACCOUNT_DISABLED` "Tài khoản đã bị vô hiệu hóa. Kích hoạt lại trước khi mời.", 409 `CONFLICT` "Học viên đã có trong lớp."; resend khi học viên đã đổi mật khẩu → 409 `CONFLICT` "Học viên đã đổi mật khẩu; không cần gửi lại lời mời.". Khi user đang `invited` và mật khẩu tạm còn hạn: không xoay mật khẩu, gửi email `added`; xoay chỉ khi hết hạn hoặc Resend, đồng thời đánh `failed` (`last_error='superseded'`) các hàng `queued` cũ cùng user trong cùng giao dịch; Invite/Resend chạy dưới `pg_advisory_xact_lock(hashtext(lower(email)))`.
- `GET /me/classes` → `{items:[{id, code, name, status, startDate, endDate, teacherName, courseName, courseVersionNo, percent, requiredDone, requiredTotal, nextLesson?: {lessonId, title, stageName}, readOnlyReason?: 'draft'|'ended'}]}`; lớp `draft` có trong danh sách (card "Lớp bắt đầu … Bạn sẽ vào học được khi lớp kích hoạt."); thành viên `dropped` không có trong danh sách (đúng `activeMembersOf`).
- `GET /me/classes/{id}` → `{class:{id, code, name, status, startDate, endDate, teacherName, courseName, courseVersionNo}, percent, requiredDone, requiredTotal, readOnly, readOnlyReason?, nextLesson?, stages:[{id, name, position, lessons:[{id, title, type, required, position, durationSeconds?, firstOpenedAt?, completedAt?}]}]}`; lớp `draft` trả 200 với `readOnly=true`.
- `GET /me/classes/{id}/lessons/{lid}` (ghi `first_opened_at` khi lớp `active`) → `{lesson:{id, title, type, required, position, durationSeconds?, stage:{id, name}}, content: {type:'markdown', html} | {type:'video', mediaId, url, expiresAt}, progress:{firstOpenedAt, completedAt?}, readOnly, prev?: {lessonId, title}, next?: {lessonId, title}}`; ký lại URL video qua `GET /media/{mediaId}/url`.
- `PUT /me/classes/{id}/lessons/{lid}/completion {completed}` → `200 {completedAt, percent, requiredDone, requiredTotal}`. Lỗi học viên: 404 `NOT_FOUND` (không là thành viên / lớp không có / học liệu không thuộc phiên bản khóa của lớp), 409 `CONFLICT` "Mở học liệu trước khi tích hoàn thành.", 409 `INVALID_TRANSITION` "Lớp chưa bắt đầu hoặc đã kết thúc; không ghi nhận tiến độ.".
- `POST /auth/change-password {currentPassword?, newPassword, confirmPassword}` (`currentPassword` bắt buộc khi `active`, bỏ qua khi `invited`); `POST /auth/reset-password {token, newPassword, confirmPassword}`; link reset dạng `PUBLIC_BASE_URL/reset-password#token=<raw>` (fragment, không vào log server). Đăng nhập: xác thực mật khẩu trước, rồi `ACCOUNT_DISABLED` (403), `TEMP_PASSWORD_EXPIRED` (401, "Mật khẩu tạm đã hết hạn, vui lòng liên hệ quản trị viên."); sai mật khẩu → 401 chung.
- Golden JSON của integration test (`apps/api/internal/features/<feature>/testdata/*.json`) do phase backend commit; phase web import làm fixture MSW, không tự chế dữ liệu.

## 8. Routes frontend (ánh xạ 1:1 với prototype)

| Route | Trang prototype | Vai trò |
|---|---|---|
| `/login`, `/forgot`, `/reset-password#token=` (đọc `location.hash` rồi `history.replaceState` xóa token) <!-- Red Team: RT-09 - token ở fragment --> | `pageLogin`, `pageForgot`, (mới theo FR-04) | public |
| `/first-login` | `pageFirstLogin` | mọi vai trò khi `mustChangePassword` |
| `/admin` | `pageAdminDashboard` | admin |
| `/admin/stages`, `/admin/stages/:stageId?v=` | `pageStages`, `pageStageDetail` | admin |
| `/admin/courses`, `/admin/courses/:courseId?v=` | `pageCourses`, `pageCourseDetail` | admin |
| `/admin/classes`, `/admin/classes/:classId?tab=students\|report\|settings` | `pageClasses`, `pageClassDetail` | admin |
| `/teach`, `/teach/classes/:classId` | `pageTeachClasses`, `pageTeachClass` | teacher |
| `/learn`, `/learn/classes/:classId`, `/learn/classes/:classId/lessons/:lessonId` | `pageLearn`, `pageLearnClass`, `pageLesson` | student |

Guard: chưa đăng nhập → `/login`; `mustChangePassword` → `/first-login`; sai vai trò → trang chủ của vai trò (`/admin`, `/teach`, `/learn`). Nav theo vai trò giống `NAV` trong `app.js`. Announcer `aria-live` đọc tiêu đề trang khi đổi route; skip link tới `#main`.

## 9. Phases

| # | Phase | Phụ thuộc | Ước lượng | Kết quả kiểm chứng |
|---|---|---|---|---|
| 01 | [Monorepo, tooling và hạ tầng](./phase-01-monorepo-infra-skeleton.md) | – | 2 ngày | `make dev` chạy, CI xanh với skeleton |
| 02 | [Database schema và migration](./phase-02-database-schema.md) | 01 | 2 ngày | Migrate up/down sạch; test SQL chứng minh ràng buộc khai báo và schema không có function/trigger; hợp đồng bất biến ở tầng ứng dụng |
| 03 | [Platform backend và shared kernel](./phase-03-backend-platform-domain-kernel.md) | 02 | 1.5 ngày | Unit test value object, Transact, pgerr mapping |
| 04 | [Identity, phiên đăng nhập, email outbox](./phase-04-identity-auth-email.md) | 03 | 3 ngày | Đăng nhập, đổi mật khẩu, quên mật khẩu, khóa, vô hiệu hóa; email tới Mailpit |
| 05 | [Chặng, phiên bản chặng, học liệu, media](./phase-05-stages-lessons-media.md) | 03 | 3 ngày | Tạo/sửa/phát hành/nhân bản/lưu trữ chặng; upload và URL ký |
| 06 | [Khóa học, phiên bản khóa, áp dụng FR-17](./phase-06-courses-versions-apply.md) | 05 | 2.5 ngày | Kịch bản 7.3 ở mức API trong một giao dịch |
| 07 | [Lớp, thành viên, lời mời](./phase-07-classes-members-invitations.md) | 04, 06 | 2 ngày | Tạo lớp, đổi phiên bản, vòng đời, mời/gửi lại/gỡ |
| 08 | [Học tập và tiến độ](./phase-08-learning-progress.md) | 07 | 1.5 ngày | Mở học liệu, tích/bỏ tích idempotent, % tính đúng |
| 09 | [Báo cáo lớp và dashboard](./phase-09-class-reports-dashboard.md) | 08 | 1.5 ngày | Bảng báo cáo, lọc, sắp xếp, drilldown, phạm vi giảng viên |
| 10 | [Web foundation: design system, shell, auth](./phase-10-web-foundation-design-system-auth.md) | 01, 04 | 3 ngày | Token = prototype; đăng nhập, first-login, quên/đặt lại mật khẩu |
| 11 | [Web admin: dashboard, chặng, khóa học](./phase-11-web-admin-content.md) | 05, 06, 09, 10 | 3 ngày | Toàn bộ action content của prototype |
| 12 | [Web admin/giảng viên: lớp, học viên, báo cáo](./phase-12-web-classes-reports.md) | 10, 07, 09 | 2.5 ngày | 3 tab lớp, drawer học viên, trang giảng viên |
| 13 | [Web học viên: lớp, lộ trình, học liệu](./phase-13-web-learning.md) | 10, 08 | 2 ngày | Video ký URL, markdown, tích hoàn thành |
| 14 | [E2E, hardening, tài liệu, phát hành](./phase-14-e2e-hardening-release.md) | 11, 12, 13 | 2 ngày | Playwright kịch bản 7.3, render-check, hardening checklist, `docs/runbook.md`, `docs/api.md` |

Thực thi tuần tự 01 → 02 → 03; sau đó 04 và 05 có thể song song (file ownership tách theo feature); 10 có thể bắt đầu ngay sau 04 với MSW mock; 11, 12, 13 song song sau 10 (mỗi phase sở hữu `src/features/<f>` riêng, chỉ phase 10 sửa `shared/` và `app/`).

## 10. Rủi ro và biện pháp

| Rủi ro | Ảnh hưởng | Biện pháp |
|---|---|---|
| MinIO server đã ngừng bảo trì (archived 2026-04) | Dev image ổn, prod cần dịch vụ khác | `Storage` interface; dev ghim tag MinIO; prod dùng Cloudflare R2 qua cùng API S3 (presign, Range, CORS cấu hình trên R2); không có spike riêng <!-- Updated: Validation Session 1 - prod R2 --> |
| Một code path ghi lên phiên bản bỏ quên điều kiện trạng thái hoặc `ByIDForUpdate` | Bản `published` bị sửa hoặc có học liệu "mồ côi" vì không còn trigger chặn ở DB | Hợp đồng SQL có điều kiện + `db.ExecAffectOne` (Phase 02/03); integration test Phase 05/06 gọi repository trực tiếp lên bản published, test đua AddLesson với Publish, publish chặng, archive khóa, FR-17; checklist review trong `docs/database.md` <!-- Red Team: RT-01 --> <!-- Updated: Session 2 - D1 --> |
| Hợp đồng DTO lệch giữa phase backend và phase web | Web vỡ khi nối API thật | §7.1 là nguồn duy nhất; fixture MSW import từ golden JSON của backend <!-- Red Team: RT-06/RT-07/RT-10 --> |
| URL ký hết hạn giữa lúc tua video | Seek lỗi 403 | Expiry ≥ 2 giờ; player bắt lỗi và xin URL mới (Phase 13) |
| SQL thủ công (psql, công cụ ngoài) sửa bản `published` vì DB không còn lớp chặn | Dữ liệu lệch với lịch sử lớp học | Chấp nhận theo D1; quy tắc vận hành trong `docs/runbook.md` (không SQL tay trên prod, backup trước migration); tài khoản ghi duy nhất là của ứng dụng <!-- Updated: Session 2 - D1 --> |
| FR-17 nhiều khóa học trong một request | Một khóa lỗi làm hỏng cả batch | Mỗi khóa học một giao dịch riêng, trả kết quả từng khóa; spec yêu cầu atomic theo từng khóa |
| sqlx không còn phát triển | Thiếu tính năng mới | Bọc truy cập qua `db.Executor`; không dùng tính năng ngoài v1.4 |
| Tái tạo design system bằng Tailwind lệch so với CSS gốc | Không đạt "100% design system" | Giữ `tokens.css` nguyên bản từ `goup.css`, map vào `@theme inline`; render-check 3 viewport so với prototype |
| Mật khẩu tạm trong email | Rủi ro hộp thư | Chấp nhận theo spec MVP; P1 đổi sang link token |

## 11. Tài liệu tham chiếu

- Spec: `spec-lms-mvp.md`; prototype: `prototype/`; hướng dẫn thiết kế: `docs/design.md`, `docs/prototype.md`, `docs/review.md`.
- Mẫu hạ tầng và skeleton: `~/Documents/personal-workspace/sidecup/{infra,Makefile,apps/api,apps/web,.github/workflows}`.
- Nghiên cứu: `plans/reports/researcher-261005-1437-go-backend-stack.md`, `plans/reports/researcher-261005-1437-react-frontend-stack.md`.
- Quyết định từ nghiên cứu frontend: nền `surface-50` + chữ `ink-2` theo prototype; viền input giữ `gray-300` như prototype (ghi nhận chưa đạt WCAG 1.4.11; chưa hỏi ở Validation Session 1, giữ nguyên prototype cho tới khi có quyết định); không dark mode; mỗi user một vai trò, admin không vào `/teach/*`.

## Red Team Review

### Session 1 — 2026-10-05

Bốn persona (Assumptions, Failure, Scope, Security) đọc toàn bộ 15 file; 40 phát hiện thô gộp còn 15 nhóm; người dùng chấp nhận áp dụng tất cả. Bằng chứng gốc ở `plans/reports/` không lưu; dòng tham chiếu đã kiểm trực tiếp trong mã thư viện (bluemonday `sanitize.go`, minio-go presign, golang-migrate `pgxv5`).

| # | Mức | Phát hiện | Quyết định | Áp dụng ở |
|---|---|---|---|---|
| RT-01 | Critical | Thứ tự ghi của `Save`/Apply/Publish vi phạm bất biến phiên bản; Publish không `FOR UPDATE` | Accept: tách `SaveDraft`/`TransitionStatus`, `ByIDForUpdate`, guard publish, test trên repository thật (Session 2 chuyển lớp chặn từ trigger sang SQL có điều kiện ở ứng dụng) | §5.2, §6, Phase 02, 05, 06 |
| RT-02 | Critical | Ba phiên bản hợp đồng `email_outbox` (cột, trạng thái, `attempts`) | Accept: một hợp đồng ở §6, claim SQL viết tường minh, `secret_enc` | §6, Phase 02, 04, 07 |
| RT-03 | Critical | Enum `ClassStatus`, mã lớp, fixture lệch giữa phase | Accept: `draft/active/ended`, regex chữ thường, fixture từ `prototype/seed.js` | Phase 02, 03, 05–09, 12–14 |
| RT-04 | Critical | Compose thiếu service `worker`; migrate dùng chung pool; rollback không khả thi | Accept: service `worker`, `*sql.DB` riêng, runbook `pg_dump -Fc` + hạ image | §5.4, Phase 01, 02, 14 |
| RT-05 | Critical | CSRF: backend yêu cầu `X-Requested-With`, web không gửi | Accept: `http.ts` luôn gửi header; test | §5.4, Phase 10, 11, 13 |
| RT-06 | Critical | Hợp đồng học viên 08↔13 lệch (lớp nháp, DTO video, mã lỗi) | Accept: §7.1 chuẩn, golden JSON dùng chung | §7.1, Phase 08, 13 |
| RT-07 | Critical | Hợp đồng admin 05/06/09↔11 lệch (route lồng vs phẳng, upload, thiếu field, `AllOutdated` không ai cung cấp) | Accept: route phẳng, DTO đầy đủ, `OutdatedReader` ở Phase 05, `duration_seconds` | §6, §7, Phase 05, 06, 09, 11 |
| RT-08 | Critical | `mc cors set` không tồn tại trên MinIO community; presign gọi mạng khi thiếu `Region` | Accept: CORS qua env, `Region: cfg.S3Region`, test presign offline | §5.4, Phase 01, 05 |
| RT-09 | High | Vô hiệu hóa không thu hồi token reset; reset không atomic; login tiết lộ trạng thái trước khi xác thực; token ở query string; `version` không có cột | Accept: toàn bộ RT-09 | §7.1, §8, Phase 03, 04, 10 |
| RT-10 | High | Hợp đồng lời mời 07↔12 lệch; đua mời cùng email; xoay mật khẩu tạm không cần thiết | Accept: body/response/mã lỗi chuẩn, advisory lock, không xoay khi còn hạn | §7.1, Phase 07, 12 |
| RT-11 | High | Config và tên ràng buộc không có nguồn duy nhất | Accept: Phase 01 sở hữu `Config`; `pgerr/constraints.go` ở Phase 02 | §5.4, §6, mọi phase backend |
| RT-12 | High | `Percent` làm tròn sai; H6 grep `password|token` vô nghĩa | Accept: `math.Round`, test (2,3)=67; H6 grep giá trị thật từ Mailpit | Phase 03, 14 |
| RT-13 | High | Khóa theo IP phá E2E chạy song song | Accept: khóa theo email, limiter IP cao, Playwright `workers: 1` | §2, Phase 04, 14 |
| RT-14 | High | Nhiều bộ fixture, seed flag thừa, tham số URL lệch prototype | Accept: một dataset seed.js, chỉ `--reset`/`--upload-sample`, URL `below`/`notlogged`/`inactive` | Phase 02, 12, 14 |
| RT-15 | Medium | Học viên lớp `draft` xem được media; quét `position()` trong HTML; bluemonday `AllowImages()` cho mọi `src`; drilldown không kiểm `class_id` | Accept: `CanUserAccess` theo trạng thái lớp, `lesson_media`, policy `NewPolicy()`, `WHERE cm.class_id` | §6, §7, Phase 02, 05, 09 |

### Whole-Plan Consistency Sweep

- **Files reread**: 15 (`plan.md` + `phase-01` … `phase-14`), sau khi 4 fork hoàn tất và trước `ak plan validate`.
- **Decision deltas checked**: RT-01…RT-15 và V1…V8 đối chiếu từng phase theo bảng Impact on Phases; tally tên audit action trên 15 file chỉ còn đúng 20 hằng chuẩn §7 (các khớp khác là tên cột `user.status`, `user.role`, `class.status` hoặc câu phủ định trong phase-03).
- **Reconciled stale references**:
  - Bộ fixture: decisions nháp ghi "basic03 ended / BASIC là chặng" → sửa theo `prototype/seed.js` (§1 đoạn bộ dữ liệu mẫu; phase-02/05/06/07/09/11/12/13/14 dùng cùng bộ; `basic04` là fixture lớp mới của phase-07).
  - Gói pgerr: `platform/db/pgerr.go` một file → gói `platform/db/pgerr` (`constraints.go` Phase 02, `map.go` Phase 03); phase-08 bỏ `db.MapErr`, ghi đúng phase sở hữu.
  - Copy lỗi lời mời: chuỗi rút gọn RT-10 → chuỗi nguyên văn prototype (§7.1, phase-07, phase-12 trùng 5 chuỗi).
  - DTO bổ sung của phase-05/06/09 (`usedBy.classCodes`, `outdatedCourses`, `requiredCount`, `memberCount`, `DELETE … → {stageDeleted|courseDeleted}`, dashboard `outdated[]`, `classes[].courseName/courseVersionNo`) → gộp vào §7.1; phase-11 nạp golden `reports/testdata/dashboard.json` của Phase 09.
  - CSP: `media-src {$S3_PUBLIC_ENDPOINT}` ở §5.4 → `{$MEDIA_ORIGIN}` + `connect-src`, khớp Caddyfile phase-01 (biến compose-only).
  - Audit `user.password_reset` ở phase-04 nằm ngoài 20 action chuẩn → bỏ; dấu vết qua `password_reset_tokens.used_at`.
  - `includeDropped`: tab Thành viên gọi `true` (prototype `membersOf`), báo cáo mặc định `false` (`activeMembersOf`) — ghi rõ ở §7 để phase-12 không đảo ngược V5.
- **Unresolved contradictions**: không còn. Lưu ý vận hành: H4 (lockout/limiter) của phase-14 phải chạy với `APP_ENV=dev` vì profile `e2e` tắt limiter IP (V2).

## Validation Log

### Session 1 — 2026-10-05

**Trigger**: hard mode, sau Red Team Session 1. 8 câu hỏi về quyết định sản phẩm/kỹ thuật không suy ra được từ spec hay prototype.

| # | Câu hỏi | Lựa chọn | Trả lời | Lý do |
|---|---|---|---|---|
| Q1 | Lưu mật khẩu tạm/token trong `email_outbox` thế nào? | (a) Mã hóa cột riêng, xóa sau khi gửi; (b) plaintext trong payload; (c) không lưu, worker tạo lại | (a) | Worker gửi lại được sau lỗi; bí mật không nằm trong `payload` hay log |
| Q2 | Chống dò mật khẩu | (a) Khóa email + limiter IP cao; (b) khóa cả IP; (c) chỉ khóa email | (a) | Không phá E2E/NAT; vẫn chặn brute force phân tán |
| Q3 | `class_members.status = completed` | (a) Trong CHECK, chưa có writer; (b) bỏ hẳn; (c) thêm luồng ghi | (a) | Giữ ngôn ngữ chung của spec, không thêm feature |
| Q4 | Đổi mật khẩu lần đầu | (a) 2 ô theo prototype, API bỏ `currentPassword` khi `invited`; (b) 3 ô; (c) 2 ô nhưng gửi lại mật khẩu tạm | (a) | 100% prototype; session đã chứng minh mật khẩu tạm |
| Q5 | `includeDropped` mặc định | (a) Ẩn, có toggle theo prototype; (b) hiện mặc định | (a) | Khớp `activeMembersOf`; vẫn giữ lịch sử FR-23 |
| Q6 | TTL URL ký media | (a) 2 giờ; (b) 6 giờ; (c) 24 giờ | (a) Giữ 2 giờ | Player đã có re-sign |
| Q7 | Các bổ sung ngoài spec/prototype/sidecup | (a) Bỏ tất cả; (b) giữ một phần; (c) giữ tất cả | (a) Bỏ tất cả | HOLD SCOPE: không mở rộng |
| Q8 | Object storage prod | (a) Cloudflare R2; (b) AWS S3; (c) chọn sau | (a) R2 | Cùng API S3, không cần MinIO prod |

**Confirmed Decisions**: V1 `secret_enc` + `OUTBOX_SECRET_KEY`; V2 lockout email + limiter IP 300/phút, tắt ở e2e; V3 `completed` trong CHECK; V4 first-login 2 ô; V5 `includeDropped=false` + checkbox "Hiện học viên đã rời lớp"; V6 `MEDIA_URL_TTL=2h`; V7 bỏ `routes --json`, `outbox list|retry`, `/__ui`, template `account_disabled`, `httpx.Paginate`, `Specification[T]`, `migrate force`, `SEED_UPLOAD_SAMPLES`, `storage-spike`, `infra/scripts/backup.sh|restore.sh`, `scripts/ratelimit-probe.sh`, `scripts/docs-links-check.sh`, CI govulncheck/pnpm audit (giữ gitleaks), `docs/security.md`, `docs/pilot-checklist.md`, `docs/decisions/*`, `CHANGELOG.md`, `apps/web/README.md`, `docs/backend-architecture.md`; giữ `design-audit.mjs` và render-check vì chúng kiểm chứng tiêu chí §3 (không phải tính năng mới); V8 Cloudflare R2.

**Action Items**: cập nhật §2, §5, §6, §7, §8, §9, §10 (đã làm trong session này); áp dụng vào 14 phase với marker `Validation Session 1`; tài liệu còn lại: `docs/README.md`, `docs/architecture.md`, `docs/database.md`, `docs/api.md`, `docs/runbook.md`.

**Impact on Phases**: 01 (Config, compose, CI, docs set), 02 (`secret_enc`, CHECK, `lesson_media`), 03 (`secretbox`, enum), 04 (lockout, outbox, change-password), 05 (R2 qua `Storage`, TTL), 07 (MemberStatus), 09/12 (`includeDropped`), 10 (first-login), 14 (runbook R2, E2E tuần tự).

**Chưa hỏi (để session sau)**: viền input `gray-300` vs WCAG 1.4.11; MP4 progressive vs HLS; phạm vi quyền giảng viên ngoài báo cáo (Q2 spec); admin có vào `/teach/*` hay không.

### Session 2 — 2026-10-05

**Trigger**: yêu cầu trực tiếp của người dùng khi rà lại Phase 02: không cho phép function và trigger ở tầng database; chuyển logic đó sang tầng ứng dụng.

| # | Quyết định | Nội dung | Lý do |
|---|---|---|---|
| D1 | Không function/trigger ở DB | Cấm `CREATE FUNCTION`, `CREATE PROCEDURE`, `CREATE TRIGGER`, `CREATE TYPE`, `CREATE EXTENSION` trong mọi migration. DB chỉ còn ràng buộc khai báo (PK, UNIQUE, CHECK, FK, partial unique index, DEFERRABLE). Hàm built-in trong câu SQL của ứng dụng (`now()`, `lower()`, `pg_advisory_xact_lock()`) vẫn dùng được | Toàn bộ logic nghiệp vụ nằm ở Go để test unit, debug và review tại một nơi; schema chỉ mô tả dữ liệu |

**Hệ quả áp dụng**:
- Bỏ migration `0010_immutability_triggers` (bốn function + sáu trigger, SQLSTATE `LMS01`). Còn 9 migration; `0001_extensions_and_enums` đổi thành `0001_schema_conventions` (chỉ `COMMENT ON SCHEMA`, không `pgcrypto`).
- Thay `cvs_sync_stage_id()` bằng FK ghép `fk_cvs_stage_version (stage_version_id, stage_id) → stage_versions(id, stage_id)` với `uq_stage_versions_id_stage`; thêm `ck_stage_versions_archived_at`, `ck_course_versions_archived_at`.
- Bất biến "bản published không sửa", "con chỉ ghi khi cha draft", "không publish khi còn markdown chưa render" chuyển thành hợp đồng ba lớp ở ứng dụng (domain → `ByIDForUpdate` → SQL có điều kiện + `db.ExecAffectOne`), định nghĩa ở Phase 02 mục "Bất biến phiên bản ở tầng ứng dụng", helper ở Phase 03, hiện thực ở Phase 05/06.
- `pgerr.Map` bỏ nhánh `LMS01`; `pgerr/constraints.go` bỏ hằng tên trigger; `constraints_test.go` không còn ngoại lệ.
- Phase 14 H5 đổi từ `immutability_check.sql` (psql) sang `make hardening-immutability` chạy integration test repository `-run TestImmutability` của Phase 05/06 và kiểm catalog `pg_trigger`/`pg_proc` rỗng.
- Giới hạn chấp nhận: SQL thủ công ngoài ứng dụng không bị chặn; quy tắc vận hành ghi ở `docs/database.md` và `docs/runbook.md`.

**Impact on Phases**: 02 (viết lại toàn bộ phần trigger), 03 (`pgerr.Map`, `db.ExecAffectOne`), 05, 06 (guarded writes, test repository thay test trigger), 14 (H5). Phase 01, 04, 07–13 không đổi (phase-11 chỉ dùng mã lỗi `VERSION_IMMUTABLE`, vẫn đúng).

<!-- slug: lms-mvp-fullstack -->
