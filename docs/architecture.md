# Kiến trúc

Tài liệu khung của Phase 01: layout monorepo và hạ tầng. Phase 03 bổ sung mục backend, Phase 10 bổ sung mục frontend.

## Layout monorepo

```text
apps/api/
  cmd/lms/                 CLI cobra: serve, migrate up|down|version, seed, worker, healthcheck
  internal/app/            router Gin (chuỗi middleware, NoRoute/NoMethod, healthz/readyz), Deps
  internal/domain/         shared kernel: value object và lỗi domain, chỉ stdlib (xem mục Backend)
  internal/platform/config danh sách biến môi trường duy nhất của dự án (Load + validate)
  internal/platform/db     Open (sqlx + pgx), Migrate/Version, Executor, Transact, ExecAffectOne; pgerr/ dịch lỗi Postgres
  internal/platform/…      apperr, httpx, middleware, clock, ids, audit, secretbox, testdb (xem mục Backend)
  migrations/              file SQL nhúng vào binary (embed.FS)
apps/web/
  src/app/                 main, router (route tree + middleware), query client, layout, RouteAnnouncer
  src/features/<f>/        api/, model/, hooks/, components/, pages/, routes.tsx, index.ts
  src/shared/              api (http, lỗi, schema chung), domain, lib, hooks, layout, ui (shadcn đã chỉnh), test
  src/styles/              app.css (Tailwind v4, @theme) + goup-tokens.css (chép nguyên từ prototype/goup.css)
  scripts/design-audit.mjs pnpm lint:design: hex thô, emoji, chữ < 12px, hit area < 44px
  e2e/                     Playwright (chromium, webkit)
infra/
  docker-compose.yml       dev + profile full (E2E)
  docker-compose.prod.yml  production sau Caddy
  docker-compose.homelab.yml  overlay Traefik thay Caddy
  caddy/Caddyfile, postgres/init/, .env.example
```

API và worker dùng chung một image (`/lms`, distroless nonroot); lệnh `serve` hoặc `worker` quyết định vai trò. Healthcheck container chạy `/lms healthcheck` vì image không có shell.

## Service compose

| Service | Dev (`docker-compose.yml`) | Production (`docker-compose.prod.yml`) |
|---|---|---|
| postgres | `postgres:16-alpine`, cổng host `${POSTGRES_HOST_PORT:-5432}`, tạo thêm DB `lms_test` | không publish cổng |
| minio + minio-init | S3 dev ở 9000 (console 9001), tạo bucket `lms-media` riêng tư, CORS qua `MINIO_API_CORS_ALLOW_ORIGIN` | không có: dùng Cloudflare R2 qua `S3_*` |
| mailpit | SMTP 1025, giao diện 8025 | không có: SMTP thật qua `SMTP_*` |
| api | profile `full`, `APP_ENV=e2e`, migrate khi khởi động | migrate khi khởi động (`MIGRATE_ON_START`, mặc định true) |
| worker | profile `full`, `MIGRATE_ON_START=false`, chờ api healthy | như dev |
| web | profile `full`, nginx ở 8081 | nginx sau Caddy |
| caddy | không có | TLS tự động, cổng 80/443 |

Ở dev thường ngày API (`go run`) và web (Vite) chạy ngoài container; chỉ postgres, minio, mailpit chạy trong Docker (`make dev`). Profile `full` dựng toàn bộ bằng container cho E2E (`make e2e`).

## Luồng request

```text
Production:  trình duyệt → Caddy (:443, HSTS, CSP, nosniff, DENY frame)
               ├─ /api/*      → api:8080
               ├─ /internal/* → 404
               └─ còn lại     → web:80 (nginx, SPA)
Homelab:     Traefik (mạng homelab) → cùng một hostname: /api/ → api, /internal không có router (404), còn lại → web, cùng header bảo mật
             Traefik → API_DOMAIN: mọi path trừ /internal → api (thêm HSTS, API tự đặt header còn lại)
Profile full: trình duyệt → web:8081 (nginx) → /api/ proxy sang api:8080
Dev:         trình duyệt → Vite :5173 → /api proxy sang localhost:8080 (xfwd)
```

Triển khai homelab: Cloudflare Tunnel (ingress đặt trong dashboard Zero Trust, cả `DOMAIN` và `API_DOMAIN` trỏ `http://traefik:80`) → Traefik trên mạng docker `homelab`; tunnel và Traefik thuộc repo hạ tầng homelab riêng, không nằm trong repo này. Stack chạy với project `lms-prod` từ `infra/.env` (`make homelab-up`, dừng bằng `make homelab-down`; volume `lms-prod_pgdata` giữ dữ liệu). `TRUSTED_PROXIES` đặt subnet của mạng `homelab` (`docker network inspect homelab`) để giới hạn theo IP thấy IP thật của client.

Web và API luôn cùng origin (cookie phiên, không cần CORS). Trình duyệt tải và xem media trực tiếp từ object storage bằng URL ký sẵn (`S3_PUBLIC_ENDPOINT`), nên CSP của Caddy cho phép `MEDIA_ORIGIN` trong `media-src` và `connect-src`. Ảnh markdown là `/api/v1/media/{id}/content`, trả 302 tới URL ký sẵn trên `MEDIA_ORIGIN`; CSP kiểm cả đích redirect nên `img-src` cũng gồm `MEDIA_ORIGIN` (`infra/caddy/Caddyfile`, `infra/docker-compose.homelab.yml`).

Header chung của API: `X-Request-ID` (nhận từ client nếu khớp `^[A-Za-z0-9-]{1,64}$`, nếu không sinh uuid v7), `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Content-Security-Policy: default-src 'none'; img-src 'self' blob:; frame-ancestors 'none'`, `Cache-Control: no-store` cho `/api/*`. `/healthz` luôn 200; `/readyz` ping database, 503 khi database không phản hồi; `/api/healthz` trùng `/healthz` để kiểm đường proxy.

## Biến môi trường

Danh sách duy nhất nằm ở `apps/api/internal/platform/config/config.go`; `infra/.env.example` (production) và `apps/api/.env.example` (dev) được test `TestEnvExample` giữ khớp với struct. Nhóm biến:

| Nhóm | Biến |
|---|---|
| Ứng dụng | `APP_ENV` (bắt buộc: dev, e2e, production), `HTTP_ADDR`, `PUBLIC_BASE_URL`, `DATABASE_URL`, `MIGRATE_ON_START`, `LOG_LEVEL`, `APP_TZ` |
| Phiên, đăng nhập | `SESSION_TTL`, `COOKIE_SECURE`, `TRUSTED_PROXIES`, `PASSWORD_MIN_LENGTH`, `TEMP_PASSWORD_TTL`, `RESET_TOKEN_TTL`, `LOGIN_MAX_FAILURES`, `LOGIN_LOCK_WINDOW`, `RATE_LIMIT_LOGIN_IP_PER_MIN`, `RATE_LIMIT_FORGOT_IP_PER_MIN` |
| Email | `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `MAIL_FROM`, `MAIL_FAIL_PATTERN` (chỉ e2e), `EMAIL_POLL_INTERVAL`, `OUTBOX_SECRET_KEY` (32 byte base64) |
| Object storage | `S3_ENDPOINT`, `S3_PUBLIC_ENDPOINT`, `S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_USE_SSL`, `MEDIA_URL_TTL`, `MAX_VIDEO_BYTES`, `MAX_IMAGE_BYTES`, `MAX_DOCUMENT_BYTES` |
| Nghiệp vụ, seed | `STALE_DAYS`, `SEED_PASSWORD` |
| Chỉ compose | `DOMAIN`, `API_DOMAIN` (chỉ homelab), `ACME_EMAIL`, `MEDIA_ORIGIN`, `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_HOST_PORT`, `TEST_DATABASE_URL` (test) |

Cấu hình sai thì tiến trình thoát mã 1 trước khi mở cổng, gộp mọi lỗi trong một lần. `APP_ENV=production` còn bắt buộc `PUBLIC_BASE_URL` https, `COOKIE_SECURE=true`, `S3_USE_SSL=true` và `SMTP_USER` không rỗng. Log là JSON (slog) và không chứa giá trị secret.

## Backend

Go, Gin, sqlx trên driver `pgx` stdlib, `log/slog`. Mã trong `apps/api`; feature nằm ở `internal/features/<f>/`, tầng nền ở `internal/platform/*` và `internal/domain`.

### Lớp và phụ thuộc

```text
handler (gin + httpx) → service (use case, Transact, audit) → repository (SQL, pgerr.Map) → db.Executor (*sqlx.DB | *sqlx.Tx)
        └──────────────────── cùng dùng internal/domain (value object, lỗi domain) ────────────────────┘
```

- `internal/domain` chỉ import stdlib và không biết HTTP status; `internal/platform/*` không import `internal/features/*`.
- Handler: bind và kiểm dữ liệu vào (`httpx.BindJSON`, `httpx.UUIDParam`), gọi service, trả `httpx.OK`/`httpx.NoContent`/`httpx.Fail`. Không viết SQL.
- Service: nhận `*sqlx.DB`, `clock.Clock`, `audit.Recorder` qua constructor (từ `app.Deps`); mỗi use case ghi là một `db.Transact`; ghi audit trong cùng transaction.
- Repository: nhận `db.Executor` nên không biết mình đang trong transaction hay không; mọi lỗi sqlx trả qua `pgerr.Map`; `sql.ErrNoRows` thành `apperr.NotFound("<thứ không tìm thấy>")` vì chỉ repository biết đó là gì.
- Feature gọi nhau qua interface hẹp, không vòng. Vòng học: `homework → grading`, `quizzes → grading` (`grading.NextFinder` cho `nextQueueItem`), `reports → grading` (`BacklogReader`), `learning → summary` (`StatusReader`); `homework` và `quizzes` ghi mốc hoàn thành qua `learning.CompletionWriter`. `grading` và `summary` không import feature nào của vòng học, chỉ đọc bảng bằng SQL.

| Gói | Vai trò |
|---|---|
| `domain` | `Email`, `Code` (stage/course/class), `LessonKey` + `Slugify`, `VersionNo`, `VersionStatus`, `ClassStatus`, `Role`, `UserStatus`, `MemberStatus`, `Percent`, `ValidatePassword`; lỗi `domain.Error{Kind, Msg}` với sentinel dùng `errors.Is` |
| `platform/apperr` | Kiểu lỗi ứng dụng duy nhất `*apperr.Error{Status, Code, Message, Details}`, hằng mã lỗi, `FromDomain` |
| `platform/db` | `Open`, `Migrate`, `Executor`, `Transact`/`TransactWithOptions`, `ExecAffectOne`/`ErrNoRowsAffected` |
| `platform/db/pgerr` | `constraints.go` (sổ tên constraint, sinh cùng migration) và `Map`: nơi duy nhất dịch lỗi Postgres |
| `platform/httpx` | `BindJSON`, `OK`, `NoContent`, `Fail`, `UUIDParam`; CSRF `CrossOriginProtection`, `RequireCustomHeader` |
| `platform/middleware` | `RequestID`, `Logger`, `Recover`, `SecurityHeaders`, `RateLimit`, `TrustProxies`/`ClientIP` |
| `platform/clock`, `platform/ids` | `Clock` (`Real{Loc}` theo `APP_TZ`, `Fake` cho test); `ids.New()` uuid v7, `ids.Parse` |
| `platform/audit` | `Recorder`, `PG` (ghi `audit_logs` trong transaction của caller), `Noop`, 28 hằng action |
| `platform/secretbox` | AES-256-GCM `Seal`/`Open` cho `email_outbox.secret_enc` bằng `OUTBOX_SECRET_KEY` |
| `platform/testdb` | Chỉ có với tag `integration`: database thật cho test (xem dưới) |

### Chuỗi middleware

`router.go` gắn theo thứ tự `RequestID → Logger → Recover → SecurityHeaders`, rồi route. Path lạ trả `404 NOT_FOUND` "Không tìm thấy đường dẫn"; sai method trả `405 METHOD_NOT_ALLOWED` kèm header `Allow`.

- `RequestID`: id hợp lệ từ client hoặc uuid v7, đặt vào header, key gin `request_id` và context (`middleware.RequestIDFrom(ctx)`).
- `Logger`: một bản ghi `http request` mỗi request với `method`, `path` (route pattern, không query), `status`, `latency_ms`, `request_id`, `ip`, và `user_id` khi middleware xác thực đã đặt key `user_id`. Không log body, query hay header; status ≥ 500 ghi mức ERROR.
- `Recover`: log panic kèm stack rồi trả `500 INTERNAL`.
- `TrustProxies`: `TRUSTED_PROXIES` rỗng (mặc định) thì không tin `X-Forwarded-For`, IP client là `RemoteAddr`; CIDR sai thì log cảnh báo và vẫn không tin proxy nào.
- `RateLimit(key, perMin, enabled)`: token bucket `x/time/rate` theo key (IP hoặc user id), burst bằng `perMin`, vượt thì `429 RATE_LIMITED` kèm `Retry-After`; `enabled=false` (môi trường e2e) thì bỏ qua. Bộ đếm nằm trong bộ nhớ tiến trình, nên giới hạn chỉ đúng khi chạy **một replica API**; chạy nhiều replica cần chuyển sang kho đếm chung.

### Luồng lỗi

```text
domain.Error{Kind, Msg} ──apperr.FromDomain──┐
lỗi Postgres (*pgconn.PgError) ──pgerr.Map───┼──> *apperr.Error ──httpx.Fail──> {"error":{"code","message","details"}}
lỗi khác ────────────────────────────────────┘      (không phải apperr → 500 INTERNAL, log ERROR kèm request_id)
```

`httpx.Fail` chỉ log lỗi 5xx có cause, không log lỗi 4xx. `details` chỉ có khi `VALIDATION_FAILED` (khóa là đường dẫn field JSON, ví dụ `items[0].title`) hoặc `retry_after` của 429.

| Mã | HTTP | Nguồn thường gặp |
|---|---|---|
| `VALIDATION_FAILED` | 400 (endpoint mà hợp đồng API ghi 422 thì tạo bằng `apperr.New(422, …)`) | `httpx.BindJSON`, `domain` Kind `Invalid`, CHECK 23514 |
| `UNAUTHENTICATED` | 401 | thiếu hoặc hết phiên |
| `TEMP_PASSWORD_EXPIRED` | 401 | đăng nhập bằng mật khẩu tạm quá hạn |
| `PASSWORD_CHANGE_REQUIRED` | 403 | còn mật khẩu tạm, gọi API ngoài allowlist |
| `FORBIDDEN` | 403 | sai vai trò, sai CSRF, Kind `Forbidden` |
| `ACCOUNT_DISABLED` | 403 khi đăng nhập, 409 khi thao tác lên tài khoản đã vô hiệu | feature identity, classes |
| `NOT_FOUND` | 404 | `apperr.NotFound`, Kind `NotFound`, FK 23503 khi tham chiếu không tồn tại, path lạ |
| `CONFLICT` | 409 | Kind `Conflict`, unique 23505, serialization/deadlock |
| `VERSION_IMMUTABLE` | 409 | Kind `Immutable` |
| `DRAFT_EXISTS` | 409 | unique bản nháp duy nhất |
| `IN_USE` | 409 | Kind `InUse`, FK 23503 khi xóa dòng đang được tham chiếu |
| `INVALID_TRANSITION` | 409 | Kind `Transition` |
| `TOO_MANY_ATTEMPTS` | 429 | khóa đăng nhập theo email |
| `RATE_LIMITED` | 429 | `middleware.RateLimit` |
| `INTERNAL`, `METHOD_NOT_ALLOWED` | 500, 405 | mã tầng vận chuyển, không thuộc 14 mã nghiệp vụ |

Thông điệp mặc định tiếng Việt của từng mã nằm ở `apperr/codes.go`; nơi tạo lỗi có thể thay bằng câu cụ thể hơn chép từ prototype.

`pgerr.Map` chỉ dịch 23505 (theo tên constraint: bản nháp → `DRAFT_EXISTS`, email/mã/thành viên/vị trí trong phiên bản → `CONFLICT` với câu riêng), 23503 (`NOT_FOUND` khi `Detail` báo tham chiếu không tồn tại, còn lại `IN_USE`), 23514 (`VALIDATION_FAILED`, câu riêng cho vài CHECK có nghĩa với người dùng), 40001 và 40P01 (`CONFLICT` "Xung đột dữ liệu, vui lòng thử lại"). Mã khác trả nguyên và thành 500. Thêm constraint mới: khai báo hằng trong `constraints.go` cùng migration, thêm case trong `map.go` nếu người dùng cần câu riêng, thêm dòng test trong `map_test.go`. Không gõ chuỗi `uq_*`/`ck_*`/`fk_*` ngoài `constraints.go`; lệnh kiểm `grep -rn '"uq_\|"ck_\|"fk_' internal --include='*.go' | grep -v pgerr/constraints.go` phải rỗng.

### Bất biến thứ tự ghi

DB không có trigger; bất biến phiên bản là hợp đồng của mã Go ([database.md](database.md), mục bất biến phiên bản ở tầng ứng dụng). Mọi use case ghi lên phiên bản chặng, phiên bản khóa học, lớp hoặc thành viên đi đúng thứ tự:

1. `db.Transact` (dùng `TransactWithOptions` với `sql.LevelSerializable` khi cần, ví dụ clone phiên bản).
2. `ByIDForUpdate`: khóa hàng header bằng `SELECT … FOR UPDATE` **trước** mọi đọc hay ghi hàng con, để hai transaction song song (thêm bài và phát hành) tuần tự hóa.
3. Kiểm quy tắc domain trên trạng thái vừa khóa (`ErrVersionImmutable`, `ErrInvalidTransition`…).
4. Ghi hàng con bằng SQL có điều kiện trạng thái cha qua `db.ExecAffectOne`; `db.ErrNoRowsAffected` được dịch thành lỗi domain tương ứng.
5. Chuyển trạng thái chỉ chạm header: `UPDATE … WHERE id = $1 AND status = $from`.
6. `audit.Recorder.Record(ctx, tx, …)` trong cùng transaction; bất kỳ lỗi nào cũng rollback cả nghiệp vụ lẫn audit.

Đổi thứ tự bài học và học liệu dùng `SET CONSTRAINTS ` + `pgerr.UqStageUnitsVersionPosition` + `, ` + `pgerr.UqLessonsVersionPosition` + ` DEFERRED` (tương tự `UqCvsVersionPosition`). Vi phạm constraint deferred chỉ lộ ra lúc COMMIT; `Transact` đưa lỗi commit qua `pgerr.Map` nên vẫn ra `409 CONFLICT`.

`audit.PG` chỉ nhận action thuộc 20 hằng trong `audit/actions.go` (`actions_test.go` khóa danh sách) và xóa đệ quy các khóa `password_hash`, `token_hash`, `password`, `temp_password` (cả dạng camelCase) khỏi `before`/`after` trước khi ghi jsonb.

### Quy tắc CSRF hai lớp

Mọi POST/PUT/PATCH/DELETE dưới `/api/v1` phải qua **cả hai** lớp: `http.CrossOriginProtection` (Sec-Fetch-Site/Origin) và header `X-Requested-With: fetch`. Thiếu một trong hai thì `403 FORBIDDEN`. `httpx.CrossOriginProtection` bọc cả engine trong `cmd/lms/serve.go`, `httpx.RequireCustomHeader()` gắn trên nhóm `/api/v1` trong `router.go`; `apps/web/src/shared/api/http.ts` luôn gửi header này. PUT thẳng lên MinIO/R2 bằng URL ký sẵn không đi qua API nên không cần header.

### Phiên, đăng nhập và email outbox

- `features/identity`: route `/auth/{login,logout,me,change-password,forgot-password,reset-password}` và `/users` (admin: liệt kê, `/{id}/disable`, `/{id}/enable`). Mật khẩu argon2id (m=19456, t=2, p=1) lưu dạng PHC; token phiên và token đặt lại là 32 byte ngẫu nhiên base64url, DB chỉ lưu sha256. Route nghiệp vụ gắn sau `SessionAuth → MustChangePassword → RequireRole(...)`; feature khác đọc người dùng bằng `identity.CurrentUser(c)`.
- Phiên: cookie `__Host-sid` khi `COOKIE_SECURE=true`, `sid` khi không; HttpOnly, SameSite=Lax, Path=/, không Max-Age. Hạn thật ở bảng `sessions` (`SESSION_TTL`), gia hạn trượt và ghi `users.last_active_at` tối đa một lần mỗi phút. Vô hiệu hóa, đặt lại mật khẩu và đổi mật khẩu (trừ phiên hiện tại) xóa phiên của user ngay trong transaction.
- Đăng nhập chạy trong một transaction giữ `pg_advisory_xact_lock` theo email, nên request song song không vượt `LOGIN_MAX_FAILURES` lần sai trong `LOGIN_LOCK_WINDOW`; vượt thì `429 TOO_MANY_ATTEMPTS` kèm `Retry-After`. Thứ tự kiểm: mật khẩu sai → `UNAUTHENTICATED` (không lộ trạng thái), rồi disabled → `ACCOUNT_DISABLED`, rồi mật khẩu tạm hết hạn → `TEMP_PASSWORD_EXPIRED`. `must_change_password=true` chỉ cho `GET /auth/me`, `POST /auth/change-password`, `POST /auth/logout`.
- `features/mailer`: use case ghi email bằng `mailer.Enqueuer.Enqueue(ctx, tx, Message)` trong transaction nghiệp vụ. Payload chỉ chứa dữ liệu không nhạy cảm; mật khẩu tạm hoặc token đặt lại đi vào `Message.Secret`, được niêm phong vào `secret_enc` và chỉ mở lúc gửi. `lms worker` claim hàng tới hạn (`FOR UPDATE SKIP LOCKED`, lease 2 phút), render template lúc gửi, gửi qua SMTP (hoặc chỉ log khi `SMTP_HOST` rỗng); lỗi thì thử lại sau 1 rồi 5 phút, lần thứ 3 thành `failed`. `secret_enc` bị xóa khi `sent` hoặc `failed`. Mỗi giờ worker chạy các `mailer.Cleanup` trong `Cleanups`: dọn `login_attempts` quá 30 ngày và phiên hết hạn, tự nộp lượt quiz quá giờ, và việc hằng ngày của hàng chờ chấm (xem "Việc hằng ngày"). Gửi lại lời mời gọi `OutboxRepo.SupersedeQueued` để hàng cũ mang mật khẩu hết hiệu lực không được gửi; trạng thái gửi đọc qua `mailer.OutboxStatusReader`.

### Thêm một feature mới

1. Tạo `internal/features/<f>/`; quy tắc nghiệp vụ dùng value object của `internal/domain`, lỗi là `domain.Error` với `Kind` đúng nghĩa và `Msg` tiếng Việt.
2. Repository: interface cùng bản Postgres nhận `db.Executor`; mọi lỗi qua `pgerr.Map`, `sql.ErrNoRows` thành `apperr.NotFound`, câu ghi có điều kiện trạng thái qua `db.ExecAffectOne`, tên constraint chỉ qua hằng `pgerr`.
3. Service: constructor nhận `*sqlx.DB`, `clock.Clock`, `audit.Recorder` (thời gian luôn lấy từ `Clock`, id từ `ids.New()`); mỗi use case ghi theo đúng thứ tự ở mục bất biến thứ tự ghi.
4. Handler: DTO có tag `json` và `binding` (`required`, `email`, `min`, `max`, `oneof`, `uuid`…); `httpx.BindJSON`, `httpx.UUIDParam` (id sai định dạng trả 404), trả `httpx.OK`/`NoContent`/`Fail`.
5. Đăng ký route trong `internal/app/router.go` dưới nhóm `/api/v1`, kèm middleware xác thực, vai trò, CSRF và `RateLimit` khi hợp đồng API yêu cầu.
6. Lỗi mới: chọn `Kind` có sẵn để `FromDomain` ánh xạ; constraint mới thì thêm hằng và case `pgerr.Map`; action audit mới phải thêm vào hợp đồng API trước rồi mới vào `actions.go`.
7. Test: unit test domain và service với `clock.Fake`, `audit.Noop`; integration test repository và service với `testdb`.
8. Kiểm: `make lint` và `make test` trong `apps/api` (cần `TEST_DATABASE_URL`); cập nhật tài liệu khi hợp đồng thay đổi.

### Chặng, media và object storage

- `features/stages`: chặng, phiên bản chặng, bài học và học liệu; route admin `/stages` và `/stage-versions/{vid}/…` (phẳng). Aggregate `StageVersion` sở hữu bài học (`stage_units`) và học liệu: chặng mới bắt đầu không có bài học; thêm học liệu bắt buộc `unitId` (thiếu hay sai → 422 "Chọn bài học cho học liệu."); tên bài học trim, 1–120 ký tự, không trùng trong phiên bản theo `lower(btrim(title))` (trùng → 409 `CONFLICT`); `unit_key` sinh từ tên như `lesson_key` và giữ nguyên khi đổi tên và khi nhân bản (clone tạo bài học mới, ánh xạ học liệu sang bài học mới). Sau mọi thao tác, `renumber()` xếp học liệu theo (vị trí bài học, thứ tự trong bài học) và đánh lại `lessons.position` phẳng 1..n, nên tiến độ và nút trước/tiếp vẫn đọc một thứ tự duy nhất. Route bài học: `POST /stage-versions/{vid}/units` `{title}` → 201, `PATCH …/units/{uid}` `{title}`, `DELETE …/units/{uid}` → 204 (xóa cả học liệu trong bài học), `PUT …/units/order` `{unitIds}`; `PUT …/lessons/order` nhận bố cục đầy đủ `{units:[{unitId, lessonIds}]}`. Phát hành cần ít nhất một bài học và không có bài học rỗng. Mọi câu ghi lên phiên bản đi qua `SaveDraft` (xóa học liệu, thêm/sửa bài học, ghi học liệu, xóa bài học, rồi header; mỗi câu có điều kiện `status = 'draft'`) hoặc `TransitionStatus` (chỉ chạm header; publish kèm điều kiện không còn markdown chưa render). Publish render markdown bằng goldmark (GFM, không `WithUnsafe`) rồi bluemonday `NewPolicy` allowlist; ảnh duy nhất được giữ là `<img src="/api/v1/media/{uuid}/content">`. Khi lưu học liệu có source markdown mới (POST, hoặc PATCH có `markdownSource`), ảnh markdown phải là `/api/v1/media/{uuid}/content`: ảnh base64 hay ảnh ngoài trả 422; PATCH chỉ đổi `title`/`required` không kiểm lại source cũ. `POST /stages/markdown-preview` (admin, `{markdownSource}` → `{html}`) áp cùng luật ảnh rồi render bằng đúng goldmark + bluemonday của lúc phát hành, cho tab Xem trước của trang soạn. `lesson_media` được dựng lại theo video và ảnh trong HTML mỗi lần `SaveDraft`. Feature khác đọc qua `stages.VersionRefReader` (repository) và `stages.OutdatedReader` (service: khóa học đang dùng bản cũ, `canApply`/`blockedReason`).
- Quiz và homework là hai loại học liệu nữa của cùng aggregate (`quiz_content.go`, `homework_content.go`), lưu ở bảng con của `lessons`: `quiz_settings`, `questions`, `question_options`, `homework_settings`, `rubric_criteria`, `homework_attachments`. `POST`/`PATCH …/lessons` nhận `quiz {mode, maxAttempts, timeLimitMinutes, shuffle, passPercent}` (practice để trống ba giới hạn; assessment mặc định 2 lượt, 20 phút, 70%; đổi giữa hai chế độ giữ câu hỏi) và `homework {briefSource, passPercent, attachmentMediaIds}` (file kèm phải là media `document` đã `ready`). Câu hỏi: `POST …/lessons/{lid}/questions` `{questionKey?, type, bodySource, explanationSource, points, options:[{id?, body, correct}]}` → 201, `PATCH …/questions/{qid}` gửi lại đủ câu (thay toàn bộ; `questionKey` không đổi được, bỏ trống thì sinh từ `cau-hoi`), `DELETE …/questions/{qid}` → 204, `PUT …/questions/order` `{questionIds}`; tiêu chí: `POST …/criteria` `{description, maxPoints}`, `PATCH …/criteria/{cid}`, `DELETE …/criteria/{cid}` → 204, `PUT …/criteria/order` `{criterionIds}`. Mọi route đi qua `requireAdmin` (giảng viên nhận 403), `lockDraft` rồi `SaveDraft`, và trả `LessonDTO` đầy đủ (trừ DELETE), kể cả `isCorrect` của phương án; DTO này chỉ có ở route admin. Lưu câu hỏi kiểm 2–6 phương án, phương án 1–500 ký tự, ít nhất một đáp án đúng và câu `single` đúng một đáp án (422 `VALIDATION_FAILED`). `SaveDraft` diff bảng con theo id (`repository_pg_content.go`), mọi câu ghi có điều kiện học liệu thuộc bản nháp nên ghi thẳng vào bản đã phát hành trả `VERSION_IMMUTABLE` mà không đổi dòng nào. Publish render thêm `body_html`/`explanation_html` của câu hỏi và `brief_html` của homework (cùng renderer và luật ảnh), chặn bằng lý do của prototype (quiz chưa có câu hỏi hay còn câu chưa có đáp án đúng, homework chưa có đề bài hay tiêu chí chấm), và `TransitionStatus` từ chối nếu còn câu hỏi hay đề bài chưa render. Clone chép sâu: câu hỏi, phương án và tiêu chí nhận id mới, `question_key` và cấu hình giữ nguyên, file kèm chỉ chép `media_id`. Trang học của học viên (`learning`) chỉ đọc loại học liệu và cấu hình, không bao giờ đọc câu hỏi hay phương án.
- `features/media`: `POST /media/uploads` (admin) tạo bản ghi `pending` và trả URL PUT ký sẵn 15 phút; trình duyệt PUT thẳng lên storage; `POST /media/uploads/{id}/complete` `Stat` object rồi chuyển `ready`. `GET /media/{id}/url` trả URL GET ký hạn `MEDIA_URL_TTL`; `GET /media/{id}/content` redirect 302 tới URL đó (ảnh markdown trỏ vào đây nên HTML không chứa chữ ký). Video dùng `kind=video`; trình soạn markdown upload ảnh bằng `kind=image` qua cùng luồng. File kèm homework dùng `kind=document`: server quyết định content type theo đuôi tên file (`.zip`, `.pdf`, `.md`/`.txt`/`.sql` là `text/plain; charset=utf-8`, `.png`, `.jpg`/`.jpeg`), bỏ qua type trình duyệt gửi, ký type chuẩn vào URL PUT (ticket trả `headers` để PUT gửi đúng) và lưu ở `media_files.content_type`; đuôi khác trả 422, dung lượng tối đa `MAX_DOCUMENT_BYTES`. URL GET của `document` luôn kèm `response-content-disposition: attachment` và `response-content-type` bằng type đã lưu (`storage.GetOptions`). File kèm được ghi vào `lesson_media` như video và ảnh, nên quy tắc quyền bên dưới áp dụng nguyên vẹn. Admin xem mọi file; giảng viên/học viên chỉ xem file gắn với học liệu của lớp mình dạy hoặc học (lớp active/ended, thành viên active). Nhóm `/media` có `RateLimit` 120 request/phút theo user. Hợp đồng khác dùng `media.Reader`.
- File học viên nộp đi qua `POST /me/uploads` và `POST /me/uploads/{id}/complete` (chỉ vai trò student, chung limiter với `/media`): chỉ `kind=document` (kind khác → 403), phải đang là thành viên active của một lớp active (→ 403), và có ít hơn `MaxUnattachedDocuments` (10) file chưa gắn bản nộp (→ 409; kiểm dưới advisory lock theo user). Hạn mức chỉ tính file `ready` tạo trong 24 giờ (`UnattachedWindow`) và file `pending` tạo trong 15 phút (`UploadTTL`), nên upload bỏ dở tự nhả chỗ; không có job dọn file mồ côi trên storage. Quyền xem file nộp: người tải lên, giảng viên phụ trách lớp của bản nộp và admin; tải về luôn là `attachment` như mọi `document`.
- `platform/storage`: interface `Storage` (`PresignPut`, `PresignGet`, `Stat`, `Delete`) và `Uploader` (`Put`, cho seed); `MinIO` dùng minio-go với hai client: client nội bộ (`S3_ENDPOINT`) gọi API từ server, client công khai (`S3_PUBLIC_ENDPOINT`) chỉ để ký URL; region cố định nên ký không gọi mạng. `MemStorage` cho test. Bucket luôn private.
- Cloudflare R2 (production): `S3_ENDPOINT=<account>.r2.cloudflarestorage.com`, `S3_PUBLIC_ENDPOINT=https://<account>.r2.cloudflarestorage.com`, `S3_REGION=auto`, `S3_USE_SSL=true`. CORS của bucket (cho phép `PUT`/`GET` từ `PUBLIC_BASE_URL`, header `Content-Type`) đặt trong dashboard Cloudflare; dev dùng `MINIO_API_CORS_ALLOW_ORIGIN` của compose.
- `lms seed --upload-sample` tải một video MP4 2 giây nhúng sẵn lên `storage_key` của mọi bài video mẫu, một file mẫu cùng loại (zip, PNG hoặc văn bản) cho mọi tài liệu mẫu (file kèm đề bài và file học viên nộp), và sửa `size_bytes` cho khớp; chạy lại an toàn.

### Khóa học và phiên bản khóa học

- `features/courses`: route admin `/courses`, `/course-versions/{vid}/…` và `POST /stage-versions/{vid}/apply` (prefix của chặng nhưng ghi vào khóa học nên thuộc courses). Mô tả khóa học nằm trên `course_versions.description`. Mỗi khóa học tối đa một bản nháp (`uq_course_versions_one_draft`; trùng → 409 `DRAFT_EXISTS` kèm `draftVersionId`, `draftVersionNo`); xóa bản nháp cuối cùng xóa luôn khóa học; xóa phiên bản lớp đang dùng → 409 `IN_USE` kèm `usedBy`.
- Cùng bất biến thứ tự ghi như stages: `Create` luôn chèn header `draft` rồi chèn chặng bằng `INSERT … SELECT … WHERE status = 'draft'`; `SaveDraft` xóa/chèn chặng rồi sửa header, mỗi câu có điều kiện draft (0 dòng → `VERSION_IMMUTABLE`); `TransitionStatus` chỉ chạm header. `uq_cvs_version_position` là DEFERRABLE nên sắp lại thứ tự trong một transaction không vướng unique.
- Áp dụng phiên bản chặng mới: mỗi khóa học một transaction (khóa dòng `courses`, clone bản published mới nhất, thay phiên bản của cùng chặng giữ vị trí, chèn draft rồi chuyển published, ghi audit). Kết quả trả theo từng khóa học (`newVersionNo` hoặc `error`); khóa học lỗi không đổi gì, lớp đang chạy vẫn trỏ phiên bản cũ.
- Hợp đồng cho feature khác: `courses.VersionReader.PublishedVersion` trả `VersionSummary` (khóa học, số phiên bản) và lỗi nếu phiên bản chưa phát hành.

### Lớp học

- `features/classes`: route `/classes` (admin: tạo, sửa, `/{id}/activate`, `/{id}/end`, mời, gửi lại, xóa thành viên; admin và giảng viên phụ trách: xem lớp và `/{id}/members`) và `GET /teach/classes` (giảng viên: lớp mình dạy kèm số học viên, chưa đăng nhập, không hoạt động quá 7 ngày, `avgPercent`). Giảng viên xem lớp người khác → 403 `FORBIDDEN` "Bạn chỉ xem được lớp mình phụ trách.". `POST /classes/{id}/invitations` có `RateLimit` 30 request/phút theo IP.
- Trạng thái lớp một chiều `draft → active → ended`; đổi phiên bản khóa học chỉ khi `draft` và phải là bản published; lớp `ended` chỉ đọc. Mỗi câu ghi có điều kiện `WHERE status = <trạng thái nguồn>` qua `db.ExecAffectOne` (0 dòng → 409 `INVALID_TRANSITION`). `activatedAt`/`endedAt` của response lấy từ `audit_logs` (`class.activated`/`class.ended`), không có cột riêng.
- Mời học viên (`DecideInvite`) và gửi lại chạy trong một transaction theo thứ tự khóa: `pg_advisory_xact_lock` theo email (cùng khóa với đăng nhập) → lớp/thành viên → user `FOR UPDATE`. Trong transaction đó ghi user mới hoặc xoay mật khẩu tạm qua `identity.UserProvisioner`, thành viên, `invitations` và email outbox; outbox lỗi thì rollback toàn bộ. Email tài khoản giảng viên/quản trị → 403, tài khoản bị vô hiệu hóa → 409 `ACCOUNT_DISABLED`, đã trong lớp → 409 `CONFLICT`, đã rời lớp → quay lại giữ nguyên tiến độ. Gửi lại tối đa 3 lần mỗi giờ cho một thành viên (`429 RATE_LIMITED`), thay hàng outbox cũ còn chờ bằng `SupersedeQueued`; học viên đã đổi mật khẩu → 409.
- Danh sách thành viên đọc trạng thái lời mời mới nhất bằng join `invitations` với `email_outbox` (`sending` hiển thị là `queued`); thành viên không có lời mời thì không có các trường `invite*`. Mật khẩu tạm không bao giờ có trong response, log, audit hay payload outbox.
- Hợp đồng cho feature khác: `classes.MembershipReader.MemberContext` (lớp không có → `ErrClassNotFound`; không thuộc lớp → `IsMember() = false`) và `classes.ProgressReader.AvgPercentByClass` (feature học tập hiện thực; trước đó `app` dùng bản trả rỗng). Classes đọc người dùng qua `identity.UserReader` (`ActiveTeacher` giữ `FOR SHARE`). Golden JSON của mọi response nằm ở `features/classes/testdata/` (cập nhật bằng `go test -tags integration ./internal/features/classes/ -update`) và được web dùng lại làm fixture MSW.

### Học tập và tiến độ

- `features/learning`: route học viên `/me/classes` (chỉ vai trò student; vai trò khác → 403): danh sách lớp đang học (thành viên `active`, gồm lớp `draft` và `ended`, không gồm lớp đã rời), `GET /me/classes/{id}` (lộ trình), `GET /me/classes/{id}/lessons/{lid}` (nội dung học liệu, ghi lần mở đầu) và `PUT /me/classes/{id}/lessons/{lid}/completion` body `{completed}`. Không là thành viên active, lớp không tồn tại hay học liệu không thuộc phiên bản khóa học của lớp đều trả 404 `NOT_FOUND`.
- Quyền xét bằng `LearningContext` dựng từ `classes.MembershipReader`: lớp `active` xem và ghi tiến độ; lớp `ended` chỉ đọc (xem học liệu không ghi `first_opened_at`); lớp `draft` chỉ có lộ trình chỉ đọc, mở học liệu → 409 `INVALID_TRANSITION` "Lớp chưa bắt đầu…". Ghi tiến độ ngoài lớp `active` → 409 `INVALID_TRANSITION`; tích trước khi mở → 409 `CONFLICT`.
- Mở học liệu là `INSERT … ON CONFLICT DO NOTHING` nên `first_opened_at` chỉ ghi một lần. Tích/bỏ tích idempotent: đọc dòng `FOR UPDATE` trong transaction rồi `UPDATE` có điều kiện trạng thái cũ (0 dòng → 409). Video được ký qua `media.URLSigner` (cùng kiểm quyền với `/media/{id}/url`) trước khi ghi lần mở, ký lỗi thì không ghi gì. `users.last_active_at` do middleware phiên cập nhật (tối đa mỗi phút một lần), learning không ghi riêng.
- Lộ trình nhóm học liệu theo bài học: mỗi chặng có `units[]` (mỗi bài học có `requiredDone/requiredTotal`, `complete` khi đã xong mọi học liệu bắt buộc, `optional` khi không có học liệu bắt buộc) cùng `unitCount`, `lessonCount`, `requiredDone/requiredTotal`, và vẫn giữ `lessons[]` phẳng (có `unitId`). Trang học liệu trả `lesson.unit` và `lesson.stage.position` cho breadcrumb và nhãn "Bài i.j", `crossesUnit` khi học liệu tiếp thuộc bài học khác, kèm `nextUnitTitle`; trước/tiếp vẫn đi qua cả khóa học, kể cả sang chặng sau.
- Phần trăm chỉ tính học liệu bắt buộc: `domain.Percent(done, total)` trong Go (`BuildRoadmap`) và `round(100.0 * done / total)` trong SQL cho kết quả trùng nhau; học liệu kế tiếp là học liệu đầu tiên chưa hoàn thành (kể cả tùy chọn) theo thứ tự chặng rồi vị trí, lớp nháp không có.
- Hợp đồng cho feature khác: `learning.ProgressReader` (`ClassProgress`, `MemberLessonProgress`, `ClassAveragePercent`, `ClassActivityCounts`) chạy trên `db.Executor` của caller; `app` nối `ClassAveragePercent` vào `classes.ProgressReader` cho `avgPercent` của `/teach/classes`. Golden JSON của response và lỗi học viên nằm ở `features/learning/testdata/` (cập nhật bằng `go test -tags integration ./internal/features/learning/ -update`), web học viên dùng làm fixture MSW.

### Homework: hạn nộp, nộp bài, chấm

- `features/homework`: route staff `PUT`/`DELETE /classes/{id}/deadlines/{lid}` (`{dueAt}` RFC 3339 → 200; xóa → 204, xóa khi chưa có hạn là no-op), `GET /classes/{id}/homework` (tổng quan homework của lớp theo thành viên), `GET /submissions/{sid}` (trang chấm: bản nộp, đề bài, rubric, hạn, toàn bộ lịch sử) và `POST /submissions/{sid}/grades` (→ 201); route học viên `GET`/`POST /me/classes/{id}/lessons/{lid}/submissions`. Admin chấm và đặt hạn mọi lớp; giảng viên chỉ lớp mình phụ trách (→ 403). Học liệu không phải homework của phiên bản khóa học của lớp, bản nộp không tồn tại hay id sai định dạng → 404 `NOT_FOUND`. Response đọc có `Cache-Control: private, no-store`.
- Nộp bài cần repo (`https://` trên github.com, gitlab.com, bitbucket.org) hoặc 1–5 file `document` của chính học viên, đã `ready` và chưa gắn bản nộp nào; `commitRef` cần repo. Chỉ thành viên active của lớp `active` nộp được (không là thành viên, đã rời lớp → 403; lớp nháp hay đã kết thúc → 409). `is_late` chốt lúc nộp theo hạn của lớp. Nộp lại khi bản trước đang chờ chấm chuyển bản đó sang `superseded`; sau khi Đạt thì không nộp lại được (409).
- Chấm chỉ bản nộp mới nhất (bản cũ → 409); điểm từng tiêu chí trong `0..maxPoints`, nhận xét bắt buộc; kết quả Đạt khi `total * 100 >= passPercent * maxPoints` (so số nguyên). Mỗi lần chấm hay sửa điểm chèn một dòng `submission_grades` mới; `submissions` chỉ đổi `status`. Trong cùng transaction: ghi điểm, đổi `status`, ghi hoặc xóa `lesson_progress.completed_at` qua `learning` (Đạt giữ mốc hoàn thành đầu tiên; Cần làm lại xóa), audit `homework.graded`/`homework.grade_updated`, và email `homework_graded` vào outbox với link ở khóa `ResultURL` (dựng từ `PUBLIC_BASE_URL`). Một bước lỗi thì rollback toàn bộ.
- Đồng thời: nộp và chấm cùng khóa dòng `class_members` trước (rồi bản nộp `FOR UPDATE`), chuyển trạng thái bằng `UPDATE … WHERE status = <trạng thái nguồn>` qua `db.ExecAffectOne`, nên hai lần nộp hay hai lần chấm đồng thời cho kết quả xác định. Đặt/xóa hạn ghi audit `class.homework_deadline_changed` (`before.dueAt`/`after.dueAt`, null khi chưa có hạn). Đổi phiên bản khóa học của lớp nháp xóa các hạn nộp cũ.
- Golden JSON nằm ở `features/homework/testdata/` (cập nhật bằng `go test -tags integration ./internal/features/homework/ -update`).

### Quiz: làm bài, chấm, tự nộp

- `features/quizzes`: route học viên (chỉ vai trò student) `GET`/`POST /me/classes/{id}/lessons/{lid}/attempts` (màn bắt đầu; bắt đầu lượt → 201, hoặc trả lượt đang mở với `resumed: true` → 200), `GET /me/attempts/{aid}`, `PUT /me/attempts/{aid}/answers/{qid}` `{optionIds}` (mảng rỗng là bỏ chọn; thiếu khóa → 422) và `POST /me/attempts/{aid}/submit`. Không là thành viên active, học liệu không phải quiz của phiên bản khóa học của lớp, hay lượt của người khác → 404 `NOT_FOUND`; dữ liệu sai (phương án không thuộc câu, câu `single` nhiều phương án) → 422. Chỉ lớp `active` bắt đầu, lưu câu và nộp được (lớp nháp hay đã kết thúc → 409 `INVALID_TRANSITION`); lớp `ended` vẫn xem được kết quả.
- Chấm hoàn toàn ở server (`scoring.go`, hàm thuần): câu được điểm khi tập phương án chọn bằng đúng tập đáp án đúng, không có điểm một phần, câu bỏ trống 0 điểm. Quiz đánh giá Đạt khi `score * 100 >= passPercent * max` (so số nguyên); số lượt là `maxAttempts` cộng tổng `quiz_attempt_grants.extra_attempts`, luyện tập không giới hạn. Lượt tốt nhất (tỉ lệ cao nhất, bằng nhau thì lượt sớm hơn) là điểm ghi nhận.
- Lộ đáp án: response dùng ba DTO riêng. `OpenAttemptDTO` (lượt mở) chỉ có câu, phương án `{id, body}` theo thứ tự đã chốt lúc bắt đầu và phương án đã chọn, kèm `deadlineAt` và `serverNow`; `PracticeResultDTO` (luyện tập đã nộp) mới có `isCorrect`, `pointsAwarded` và lời giải; `AssessmentResultDTO` (quiz đánh giá đã nộp) chỉ có điểm, `passed`, `reviewStatus`, lượt tốt nhất, lượt còn lại và nhận xét của giảng viên, không bao giờ có phương án hay đúng/sai từng câu. Integration test duyệt JSON thô theo allow-list khóa. Trang học liệu của `learning` chỉ trả cấu hình và số câu, không có câu hỏi.
- Thứ tự khóa ở mọi đường ghi là `class_members` `FOR UPDATE` rồi `quiz_attempts` `FOR UPDATE`; `uq_quiz_attempts_one_open` là lớp bảo vệ cuối (→ 409 `CONFLICT`). Bắt đầu lượt xáo `question_order`/`option_order` một lần khi quiz bật shuffle (`Shuffler`, test dùng bản cố định) và ghi `first_opened_at` như khi mở học liệu.
- Nộp tay, tự nộp khi đọc và tự nộp của worker dùng chung `autoOrSubmit`: lượt đã nộp thì không ghi gì; quá `deadline_at + SubmitGrace` (30 giây) thì `submitted_at = deadline_at`, `auto_submitted = true`, và `POST …/submit` trả 409 "Đã hết giờ làm bài; hệ thống đã tự nộp với các câu đã lưu." sau khi transaction đã commit lần tự nộp. Mọi câu của lượt được ghi `is_correct`/`points_awarded` (câu bỏ trống có dòng rỗng). Quiz đánh giá đặt `review_status = 'pending'` và chuyển lượt `pending` cũ của cùng thành viên và học liệu sang `superseded`; luyện tập để NULL. Cùng transaction ghi `lesson_progress.completed_at` qua `learning.CompletionWriter`: luyện tập ở lần nộp đầu, đánh giá ở lượt Đạt đầu tiên; không bao giờ xóa.
- Tự nộp lượt quá giờ: đọc màn bắt đầu, đọc lượt và bắt đầu lượt mới tự nộp lượt quá ân hạn của chính học viên trong transaction riêng trước khi đọc. `quizzes.Service` hiện thực `mailer.Cleanup` và nằm trong `Cleanups` của `lms worker` (`app.NewQuizzesService`): mỗi giờ `CloseExpired` đóng tối đa 100 lượt mở đã quá ân hạn, mỗi lượt một transaction; lượt lỗi chỉ ghi log. Lượt bỏ dở không ai mở lại vào hàng chờ chấm chậm tối đa một giờ. Lượt mở của thành viên đã rời lớp hay của lớp không còn `active` giữ nguyên trạng thái mở: không tự nộp, không vào hàng chờ.
- Xác nhận và nhận xét (`ReviewService`): route staff `GET /quiz-attempts/{aid}` (trang xem lượt đã nộp: câu theo thứ tự học viên thấy, phương án kèm đúng/đã chọn, các lần nhận xét mới nhất trước, lịch sử lượt), `POST /quiz-attempts/{aid}/reviews` `{comment, notes:[{questionId, note}]}` (→ 201 `{review, reviewStatus, updated, nextQueueItem}`) và `GET /classes/{id}/quizzes` (tab Quiz của lớp: thống kê và trạng thái từng thành viên đang học). Admin mọi lớp; giảng viên lớp khác → 403 (lượt tự xác định lớp); lượt đang làm hay không tồn tại → 404. Nhận xét chung bắt buộc (tối đa 5000 ký tự), ghi chú từng câu tối đa 1000 ký tự và phải thuộc lượt (→ 422). Lượt `superseded` → 409 "Học viên đã nộp lượt mới hơn. Hãy xác nhận lượt mới nhất.". Mỗi lần lưu chèn một dòng `quiz_attempt_reviews` (kèm `quiz_review_notes`); lần đầu đổi lượt đánh giá `pending` (hay lượt luyện tập NULL) thành `confirmed`, không đổi điểm, `passed` hay `completed_at`. Cùng transaction (khóa `class_members` rồi lượt, như nộp bài): audit `quiz_attempt.confirmed`/`quiz_attempt.commented`, lần sau `quiz_attempt.review_updated` có `before`, và email `quiz_reviewed` với link ở `ResultURL`; payload email không có nội dung câu, phương án, giải thích hay ghi chú. Kết quả quiz bài tập phía học viên đánh số ghi chú (`position`) theo thứ tự học viên thấy, trùng số câu trên trang staff. `nextQueueItem` chỉ có ở lần xác nhận đầu của lượt đánh giá (qua `grading.NextFinder`, hướng import `quizzes → grading`); luyện tập không bao giờ vào hàng chờ.
- Cấp thêm lượt (`ReviewService.Grant`, `grant.go`): route staff `POST /classes/{id}/members/{mid}/grants` `{lessonId, extraAttempts, reason}` → 201 bản ghi cấp lượt `{id, memberId, lessonId, studentName, extraAttempts, reason, grantedByName, grantedAt}`. Admin mọi lớp; giảng viên lớp khác → 403 "Bạn chỉ cấp thêm lượt cho lớp mình phụ trách."; thành viên không thuộc lớp của URL → 404 (ràng ở SQL, không ghi gì). Lớp không `active` → 409 "Lớp không ở trạng thái đang chạy"; thành viên đã rời lớp → 409. Học liệu không phải quiz bài tập của phiên bản khóa học của lớp (kể cả quiz luyện tập, `lessonId` sai định dạng) → 422; `extraAttempts` thiếu hoặc ngoài 1–3, lý do rỗng sau khi trim hoặc quá 500 ký tự → 422. Khóa `class_members` `FOR UPDATE` trước như bắt đầu lượt, nên cấp lượt và bắt đầu lượt của cùng thành viên chạy tuần tự và giới hạn không bị vượt; cùng transaction ghi audit `quiz.attempts_granted` (target lớp, `after` có `classCode`, `memberId`, `studentName`, `lessonId`, `lessonTitle`, `extraAttempts`, `reason`). Không có migration: giới hạn vẫn là `maxAttempts` cộng tổng `quiz_attempt_grants.extra_attempts`; màn bắt đầu của học viên có `granted` (tổng lượt đã được cấp).
- `GET /classes/{id}/quizzes` có thêm ở mỗi quiz `grants[]` (mọi lần cấp của lớp cho quiz đó, kể cả của thành viên đã rời, mới nhất trước) và `questionStats {attempts, rows:[{questionId, position, bodyText, correctPercent}]}`. Mẫu số `attempts` là mọi lượt đã nộp (kể cả lượt bị thay thế) của thành viên đang học; câu đúng là `points_awarded > 0`; `correctPercent` null khi chưa có lượt nào, câu toàn bỏ trống là 0. `bodyText` là văn bản thuần một dòng (bỏ thẻ HTML, tối đa 120 ký tự kể cả "…"). Tab vẫn chạy số truy vấn cố định cho cả lớp.
- Golden JSON nằm ở `features/quizzes/testdata/` (cập nhật bằng `go test -tags integration ./internal/features/quizzes/ -update`; `class-quizzes.json` dựng trên `lms seed`). `lms seed` có lượt quiz theo `seed.js` (đã nộp, chờ xác nhận, đã xác nhận kèm nhận xét và ghi chú từng câu, đã thay thế và một lượt luyện tập đang làm dở), giờ tương đối với `now`.

### Hàng chờ chấm

- `features/grading`: chỉ đọc, không import `homework` hay `quizzes` (đọc bảng bằng SQL). `GET /grading/queue?kind=homework|quiz&classId=&lessonId=&overdue=1` và `GET /grading/recent?limit=` (mặc định 6, tối đa 50) cho admin (mọi lớp) và giảng viên (chỉ lớp có `classes.teacher_id` là mình); học viên → 403. Response có `Cache-Control: private, no-store`; tham số không đọc được → 422 `VALIDATION_FAILED` "Tham số lọc không hợp lệ.". Mọi truy vấn của một request chạy trong transaction `REPEATABLE READ READ ONLY`.
- Hàng chờ là một truy vấn `UNION ALL`: bản nộp `submissions.status = 'pending'` và lượt quiz `quiz_attempts.review_status = 'pending'` (hai partial index `ix_submissions_pending`, `ix_quiz_attempts_pending`), chỉ của thành viên `active`, kể cả lớp đã kết thúc; sắp `submitted_at, item_id`. Học viên đã rời lớp không vào hàng chờ và số đếm, nhưng bài cũ vẫn chấm được qua link trực tiếp. Mỗi mục có `waitedHours` và `overdue` (chờ quá `grading.GradingSLA` = 72 giờ); mục quiz chỉ có `scorePercent`, không có đáp án hay điểm từng câu.
- Bộ lọc áp trên toàn hàng chờ của actor trong Go (`Filter.Match`); `total`, `quizPending`, `overdueCount` luôn đếm trên toàn hàng chờ. `filters.classes` là lớp không phải nháp actor thấy; `filters.lessons` là homework và quiz bài tập của các lớp đó, thu hẹp theo lớp và loại đang chọn, mỗi học liệu một lần.
- `recent` gồm lần chấm hiện hành của mỗi bài nộp đã chấm và nhận xét hiện hành của mỗi lượt quiz bài tập đã xác nhận, mới nhất trước.
- `grading.NextFinder.Next(ctx, ex, actor, after)` trả mục cũ nhất còn lại trong toàn hàng chờ của actor, bỏ qua `after`, đọc trên transaction của caller. `homework` gọi sau lần chấm đầu để điền `nextQueueItem` của `POST /submissions/{sid}/grades` (sửa điểm hoặc hết hàng chờ → null); hướng import là `homework → grading`.
- Golden JSON nằm ở `features/grading/testdata/` (dữ liệu `lms seed` cộng thêm vài lượt quiz có nhận xét chèn bằng SQL; cập nhật bằng `go test -tags integration ./internal/features/grading/ -update`).

### Việc định kỳ của worker: tự nộp quiz, email chấm trễ, nhắc hạn nộp

- `lms worker` gửi outbox mỗi `EMAIL_POLL_INTERVAL` và chạy `Cleanups` mỗi giờ (lần đầu ngay khi khởi động), theo thứ tự trong `cmd/lms/worker.go`: `identity` xóa `login_attempts` cũ, `quizzes.Service.CloseExpired` tự nộp lượt quá ân hạn (mục Quiz), rồi `grading.Jobs` gửi email chấm trễ và nhắc hạn nộp. Một cleanup lỗi chỉ ghi log, không chặn cleanup sau.
- `grading.Jobs` hiện thực `mailer.Cleanup` và nằm trong `Cleanups` của `lms worker`, nên chạy mỗi giờ (lần đầu ngay khi worker khởi động). Ngày và giờ tính theo `APP_TZ` (mặc định `Asia/Ho_Chi_Minh`) qua `JobsDeps.Location`; link trong email dựng từ `PUBLIC_BASE_URL`.
- Email tổng hợp chấm trễ (`grading_digest`): chỉ chạy khi giờ địa phương ≥ 08:00. Mỗi giảng viên không bị khóa có ít nhất một mục quá `GradingSLA` trong hàng chờ của các lớp mình phụ trách (cùng truy vấn hàng chờ, kể cả lớp đã kết thúc) nhận một email liệt kê các mục đó, link ở payload `QueueURL` = `/teach/grading?overdue=1`. Mục vẫn quá hạn sang ngày sau thì có email mới.
- Nhắc hạn nộp (`deadline_reminder`): mỗi cặp (thành viên, homework) có hạn nộp trong `(now, now + 24h]`, lớp và thành viên `active`, tài khoản học viên không bị khóa và chưa có bản nộp nào nhận một email; hạn hiển thị theo `APP_TZ`, link trang homework ở payload `LessonURL`. Homework không có hạn nộp thì không bao giờ được nhắc.
- Khử trùng lặp bằng bảng `scheduled_job_runs(run_key PK)`: trong một transaction cho mỗi email, `INSERT … ON CONFLICT (run_key) DO NOTHING` rồi enqueue chỉ khi chèn được một dòng. Khóa là `grading_digest:<teacherId>:<yyyy-mm-dd theo APP_TZ>` và `deadline_reminder:<memberId>:<lessonId>:<dueAt RFC3339 UTC>`; `dueAt` nằm trong khóa nên đổi hạn nộp thì học viên được nhắc lại theo hạn mới. Nhờ khóa chính, worker khởi động lại hay nhiều replica chạy cùng lúc vẫn chỉ enqueue mỗi khóa một lần; outbox giao ít nhất một lần nên SMTP lỗi giữa chừng có thể hiếm khi gửi trùng.
- Lỗi enqueue của một email làm rollback khóa của email đó (lần chạy sau thử lại) và chỉ được ghi log, không chặn email khác; lỗi đọc danh sách được trả về worker để ghi log. Không ghi audit cho email tự động: outbox đủ để tra cứu.

### Báo cáo lớp và dashboard

- `features/reports`: chỉ đọc, không ghi bảng nào. `GET /classes/{id}/report` và `GET /classes/{id}/report/members/{mid}` cho admin và giảng viên phụ trách (giảng viên lớp khác → 403 "Bạn chỉ xem được lớp mình phụ trách.", học viên → 403); `GET /dashboard` chỉ admin. Mọi response có `Cache-Control: private, no-store` và `selfReported: true` ở báo cáo lớp và drilldown (tiến độ do học viên tự tích).
- Báo cáo lớp là một truy vấn CTE mỗi lớp: một dòng mỗi thành viên kèm % từng chặng theo thứ tự chặng, % tổng (chỉ học liệu bắt buộc, cùng công thức với learning), lời mời mới nhất (`sending` hiển thị `queued`), `lastLoginAt` và `lastActivityAt` (mốc mới hơn giữa `users.last_active_at` và lần mở/hoàn thành học liệu). Header, chặng, dòng và `summary` đọc trong một transaction `REPEATABLE READ READ ONLY` nên không lệch nhau.
- Query `notLoggedIn`, `inactiveDays` (≥ 1; chưa hoạt động lần nào cũng tính), `belowPercent` (0–100), `includeDropped` (mặc định `false`) và `sort` (`name`, `pct`, `pct-desc`, `activity`, `activity-asc`; hòa thì theo tên rồi id, `activity` để null cuối, `activity-asc` để null đầu). Bộ lọc kết hợp là giao. Giá trị không đọc được hoặc ngoài biên → 422 `VALIDATION_FAILED` "Tham số lọc không hợp lệ." trước khi đọc dữ liệu. `Filter.Apply` dịch bộ lọc thành tham số nullable `$3..$6` và chọn `ORDER BY` từ map allowlist, không nối chuỗi từ query; `Filter.Match` là cùng quy tắc trên Go và integration test đối chiếu hai bên. `summary` đếm theo cùng `includeDropped`, ngưỡng mặc định `STALE_DAYS` ngày và 50% được trả lại trong `summary.inactiveDays`/`belowPercent`.
- Drilldown ràng `mid` với lớp của URL ở SQL (thành viên lớp khác hoặc không tồn tại → 404) và liệt kê mọi học liệu của phiên bản khóa học qua `learning.ProgressReader.MemberLessonProgress`; thành viên đã rời lớp vẫn xem được.
- Dashboard: `kpis` (chặng, khóa học, lớp active, học viên active khác nhau trong lớp active), `hints` (lớp nháp, học viên lớp active chưa đăng nhập, khóa học lỗi thời, lời mời thất bại theo lời mời mới nhất của thành viên active trong lớp chưa kết thúc), `outdated[]` từ `stages.OutdatedReader.AllOutdated`, `classes[]` với `avgPercent` từ `learning.ProgressReader.ClassAveragePercent`, `recentActivity` 8 dòng audit mới nhất, `gradingBacklog[]` (mỗi giảng viên không bị khóa, theo tên: `pending` và `overdue` trên hàng chờ các lớp mình phụ trách, `avgHours` là trung bình từ lúc nộp tới lần chấm đầu hoặc lần xác nhận quiz bài tập đầu do chính giảng viên đó làm, null khi chưa có) và `overdueCount` (tổng mục quá hạn toàn hệ thống). Hai trường cuối đọc qua `reports.BacklogReader` (`grading.Service.Backlog`) trên cùng transaction, dùng đúng truy vấn hàng chờ nên khớp số của `GET /grading/queue`; hướng import là `reports → grading`. Audit đọc qua `audit.Reader.Latest`; tên đối tượng tra theo lô, người thao tác qua `identity.UserReader.SnapshotByIDs`; nhãn hành động là bảng tiếng Việt cố định, response không có `before`/`after` thô mà chỉ `summary` đã dựng (đối tượng đã bị xóa thì dựng từ payload).
- Golden JSON nằm ở `features/reports/testdata/` (dữ liệu `lms seed` trên `lms_test`; cập nhật bằng `go test -tags integration ./internal/features/reports/ -update`), web dùng làm fixture zod/MSW.

### Tổng kết tiến độ và nhận xét chặng

- Một định nghĩa hoàn thành: học liệu xong khi và chỉ khi `lesson_progress.completed_at` có giá trị. Mốc này ghi trong cùng transaction với sự kiện quyết định (tích video/markdown, nộp quiz luyện tập lần đầu, lượt quiz bài tập Đạt đầu tiên, chấm Đạt homework; chấm thành Cần làm lại xóa mốc). learning, reports và summary đều đọc cùng cột nên số % khớp nhau; integration test của summary đối chiếu ba nguồn sau từng loại sự kiện.
- `features/summary`: route học viên `GET /me/classes/{id}/summary` (tổng kết khóa kèm nhận xét chặng; không thuộc lớp hoặc đã rời lớp → 403, lớp không tồn tại → 404), route staff `GET /classes/{id}/summary` (ma trận học viên đang học × chặng), `GET /classes/{id}/members/{mid}/stages/{svid}/summary` (drawer) và `PUT …/stages/{svid}/review` `{body}` (→ 200 kèm nhận xét đã lưu). Admin mọi lớp; giảng viên lớp khác → 403; thành viên không thuộc lớp của URL hoặc phiên bản chặng ngoài khóa học của lớp → 404. Các GET có `Cache-Control: private, no-store` và đọc trong transaction `REPEATABLE READ READ ONLY` (`reports.ReadTx` nối ở `app`).
- Trạng thái học liệu, cờ chặng (`complete`, `missing`, `rework`, `overdue`) và "Quá hạn" đều suy ra lúc đọc từ dữ liệu thô (`status.go`, hàm thuần), không có cột hay bảng nào lưu. "Quá hạn" là homework có hạn, `now > dueAt` và chưa nộp lần nào; chặng không có học liệu bắt buộc thì không tính là hoàn thành. Ma trận chạy số truy vấn cố định (mỗi bảng hoạt động một truy vấn cho cả lớp), không tăng theo số học viên.
- Nhận xét tổng kết chặng: một dòng `stage_reviews` mỗi (thành viên, phiên bản chặng), nội dung 1–5000 ký tự sau khi cắt khoảng trắng (→ 422). Lưu khóa `class_members` `FOR UPDATE` trước; lần đầu `revision = 1`, mỗi lần sửa `revision + 1` qua `UPDATE … WHERE revision = <cũ>` (`db.ExecAffectOne`) và người viết là người sửa. Cùng transaction ghi audit `stage_review.saved` (target lớp, `before` là nội dung cũ khi sửa). Trạng thái chặng không chặn việc nhận xét.
- Tổng kết toàn khóa: `GET /me/classes/{id}/summary` có thêm `course {stagesComplete, stagesTotal, stages:[{stageVersionId, name, complete, missingCount, assessments:[{lessonId, title, bestPercent, passed}], homework:[{lessonId, title, total, max, status}]}]}`, dựng thuần từ cùng `StageSummary` của các chặng (`summary.Course`) nên `stagesComplete` luôn bằng số chặng `complete`. Homework `status` là `not_submitted|pending|passed|rework` theo lần nộp mới nhất; `total`/`max` null tới khi được chấm. Không có điểm tổng khóa: JSON không có khóa `totalScore` hay `courseScore` (integration test khóa lại).
- Hướng import là `learning → summary`: `summary.StatusReader.ForMember` cho lộ trình và trang học liệu nhãn trạng thái (`status: {label, tone}`) của quiz và homework. summary không import `learning`, `reports`, `homework` hay `quizzes` (đọc bảng bằng SQL). Golden JSON nằm ở `features/summary/testdata/` (dữ liệu `lms seed`, có hai nhận xét chặng; cập nhật bằng `go test -tags integration ./internal/features/summary/ -update`).

### Integration test với `testdb`

- File test mang `//go:build integration`; cần `TEST_DATABASE_URL` trỏ tới database `lms_test` (thiếu thì test bị skip). Chạy bằng `make test-integration` (hoặc `make test` khi `.env` có biến) trong `apps/api`; lệnh dùng `-p 1` vì mọi package chung một database.
- `testdb.Open(t)`: mỗi tiến trình test giữ advisory lock của `lms_test` (package khác chờ tới khi tiến trình này thoát) và migrate up một lần; trả pool dùng chung.
- `testdb.Reset(t, dbx)`: TRUNCATE mọi bảng trừ `schema_migrations`, đặt lại sequence.
- `testdb.Tx(t, dbx)`: transaction tự rollback khi test kết thúc; là `db.Executor` nên truyền thẳng vào repository. Một câu lỗi làm hỏng transaction hiện tại, nên mỗi trường hợp lỗi dùng một `Tx` riêng (hoặc SAVEPOINT).
- `testdb.Fixture(t, ex, name)`: chạy `fixtures/<name>.sql` sau các fixture nó phụ thuộc. Id cố định là hằng (`testdb.StageDBV1ID`, `testdb.CourseBasicV1ID`…); mật khẩu mọi user là `testdb.FixturePassword` (argon2id cùng tham số seed). INSERT trong fixture dùng `ON CONFLICT (<cột khóa>) DO NOTHING` với cột chỉ định rõ, vì Postgres không nhận constraint unique deferrable làm arbiter.

| Fixture | Dữ liệu | Phụ thuộc |
|---|---|---|
| `admin_quan_tran`, `teacher_huong_le`, `student_an_nguyen` | Ba người dùng active theo `prototype/seed.js` | không |
| `stage_db_published` | Chặng `DB` v1 published, bài markdown `db-table` (bắt buộc) và `db-index` (không bắt buộc) đã render | admin |
| `stage_db_draft` | `DB` v2 draft clone từ v1, `db-table` chưa render | `stage_db_published` |
| `course_basic_published` | Khóa `BASIC` v1 published gồm `DB` v1 | `stage_db_published` |
| `class_basic01_active` | Lớp `basic01` active trên `BASIC` v1, giảng viên Hương, học viên An | khóa học, giảng viên, học viên |

## Frontend

SPA React 19 + Vite + Tailwind v4 trong `apps/web`, React Router v8 (data mode, middleware), React Query cho dữ liệu server, react-hook-form + zod cho form, component shadcn/radix đã chỉnh theo GoUp.

### Lớp và phụ thuộc

Phụ thuộc một chiều `shared → features → app`, ESLint (`apps/web/eslint.config.js`) chặn chiều ngược:

- `src/shared/` không import `@/features/*` hay `@/app/*`; `src/features/` không import `@/app/*`.
- Feature khác chỉ dùng một feature qua `@/features/<f>` (`index.ts`), không đi tắt vào file nội bộ.
- `features/<f>/model/` là logic thuần (schema zod, điều hướng, quy tắc): không React, không gọi mạng.
- Import từ `react-router` (`RouterProvider` từ `react-router/dom`), không dùng `react-router-dom`.

### Route và quyền sở hữu file

`src/app/router.tsx` gom route của chín feature; comment đầu file là hợp đồng đầy đủ.

| Vùng | Guard | Layout | Route từ feature |
|---|---|---|---|
| `/login`, `/forgot` | khách (đã đăng nhập thì về trang chủ của vai trò) | AuthLayout | `auth.routes` |
| `/reset-password` | không cần phiên (token quyết định) | AuthLayout | `auth.publicRoutes` |
| `/first-login` | có phiên và còn mật khẩu tạm | AuthLayout | `auth.sessionRoutes` |
| `/admin` | `requireRole('admin')` | AppShell | `dashboard`, `stages`, `courses`, `classes`, `reports.routes`, `grading.routes` |
| `/teach` | `requireRole('teacher')` | AppShell | `reports.teachRoutes`, `grading.teachRoutes` |
| `/learn` | `requireRole('student')` | AppShell | `learning.routes`, `summary.routes` |

- `routes.tsx` của feature export `RouteObject[]` với path tương đối trong vùng (không lặp `/admin`…); trang gốc của vùng là `{ index: true }`. Trang tải lazy và export `Component` (hoặc dùng `lazyPage` cho `default`).
- `handle: { title }` đặt `document.title` là "{title} · GoUp LMS"; `RouteAnnouncer` đọc tiêu đề qua `aria-live` và chuyển focus tới `h1` của trang sau mỗi lần điều hướng.
- Vòng học: hàng chờ `/admin|teach/grading` (bộ lọc trong query), trang chấm `…/grading/:submissionId` và xác nhận lượt quiz `…/grading/quiz/:attemptId`; trang lớp `/admin/classes/:classId` và `/teach/classes/:classId` có tab `?tab=summary|homework|quiz` dùng chung component của `summary` và `grading`. Học viên làm quiz và nộp homework ngay trên trang học liệu `/learn/classes/:classId/lessons/:lessonId` (kết quả lượt ở `?result=<attemptId>`), xem tổng kết ở `/learn/classes/:classId/summary`. Trang soạn `…/lessons/:lessonId/edit` mở trình soạn quiz hoặc homework theo loại học liệu.
- Người dùng có mật khẩu tạm bị giữ ở `/first-login`; path lạ trong một vùng hiện NotFound trong AppShell (vẫn qua guard).
- Quyền sở hữu: phase feature chỉ thêm file trong `src/features/<f>/**` và sửa `routes.tsx` của feature đó. `src/app`, `src/shared`, `src/styles` thuộc phần nền; cần đổi ở đó thì đề nghị thay đổi, không sửa tại chỗ.

### HTTP, React Query và phiên

- `shared/api/http.ts` là client duy nhất: cùng origin, `credentials: same-origin`, luôn gửi `X-Requested-With: fetch`. Lỗi non-2xx thành `ApiError(status, code, message, details)` từ envelope `{ error: { code, message, details } }`; 204 trả `undefined`; payload sai schema zod thành `INVALID_RESPONSE`; mất kết nối thành `status 0`.
- Người dùng hiện tại nằm trong cache React Query khóa `["auth", "me"]` (`null` là chưa đăng nhập); middleware route đọc từ đó, không gọi lại API mỗi lần điều hướng.
- `app/query-client.ts` xử lý phiên tập trung: 401 từ bất kỳ query/mutation nào (trừ trên `/login`) xóa toàn bộ cache rồi chuyển tới `/login?next=<trang hiện tại>`; 403 `PASSWORD_CHANGE_REQUIRED` chuyển tới `/first-login`; lỗi mutation khác hiện toast, trừ mutation đặt `meta: { silent: true }` vì form tự hiện lỗi. Không retry lỗi 4xx; lỗi mạng và 5xx thử lại tối đa hai lần.
- Đăng nhập thành công cũng xóa cache trước khi lưu người dùng mới, nên dữ liệu của người trước không còn trên cùng tab. `next` chỉ nhận path nội bộ bắt đầu bằng đúng một `/`.
- Token đặt lại mật khẩu chỉ nằm trong fragment (`/reset-password#token=…`), được đọc một lần rồi xóa khỏi thanh địa chỉ bằng `history.replaceState`; `apps/web/nginx.conf` ghi access log theo `log_format lms_web` (chỉ method, path, status), không có query string.

### Trang soạn học liệu markdown

- Bài markdown được thêm và sửa trên trang riêng `/admin/stages/:stageId/versions/:versionId/lessons/new` và `…/lessons/:lessonId/edit` (`features/stages/pages/lesson-editor-page.tsx`); bài video vẫn dùng dialog. Dialog "Thêm học liệu" chọn Markdown thì chuyển tiêu đề và cờ bắt buộc sang trang soạn qua `location.state`.
- Trình soạn là MDXEditor (`components/markdown-editor/`), chỉ nằm trong chunk lazy của trang soạn. Bộ plugin và thanh công cụ chỉ tạo cấu trúc GFM mà server giữ lại (H1–H4, đậm, nghiêng, gạch ngang, danh sách, trích dẫn, khối mã, bảng, đường kẻ, liên kết, ảnh đã upload); `gfm-guard.ts` gỡ định dạng ngoài GFM khi gõ và dán (gạch chân, H5/H6, checklist, link không an toàn, ảnh không phải media nội bộ). Ảnh chỉ vào qua dialog upload (bắt buộc alt) hoặc dán/kéo thả file (alt là tên file bỏ đuôi), tối đa 5 ảnh tải cùng lúc.
- Lưu chỉ gửi `markdownSource` khi người dùng đã sửa nội dung, nên bài cũ không bị chuẩn hoá lại và refetch không ghi đè source. Trước khi gửi, client chặn: nội dung rỗng hoặc có ảnh base64, ảnh/link vi phạm còn sót trong nội dung cũ (`violations()`), body JSON trên 64 KB, ảnh còn đang tải. `VERSION_IMMUTABLE` khi lưu khóa trình soạn và cho chép markdown ra.
- Tab "Xem trước" gọi `POST /stages/markdown-preview` và hiện bằng `MarkdownContent`, giống trang học viên. Rời trang khi còn thay đổi chưa lưu thì hỏi xác nhận (`useBlocker` và `beforeunload`), trừ khi chuyển về `/login` hay `/first-login` do hết phiên.
- MDXEditor để trống tên một số phần tử (ô bảng, vùng mã CodeMirror, nút thêm hàng/cột); `editor-a11y.ts` gắn `aria-label` cho chúng, nút thêm hàng/cột nhận theo tiền tố class CSS-module. Khi nâng MDXEditor, chạy lại `apps/web/e2e/lesson-editor.spec.ts` (có kiểm axe trên trang soạn) để chắc các selector còn khớp. Khối mã trong trình soạn hiện một màu như trang học viên vì màu token của theme CodeMirror không đạt tương phản.

### Design system

- `src/styles/goup-tokens.css` giữ nguyên byte so với `prototype/goup.css`; `src/styles/app.css` thêm token sản phẩm (`--ink*`, trạng thái ok/warn/danger, token shadcn) và ánh xạ sang Tailwind bằng `@theme inline` (`bg-navy-700`, `text-ink-3`, `rounded-pill`, `shadow-card`, `max-w-page`…).
- Font, radius và shadow trong `@theme` là giá trị literal trùng từng ký tự với goup, vì `:root` của goup (không thuộc layer) thắng `@layer theme` của Tailwind.
- Component dùng utility từ token, không hex thô; chữ tối thiểu 12px (trừ tag 11px của goup); vùng bấm tối thiểu 44px (desktop có thể nhỏ hơn nếu có override `max-[720px]:min-h-11`). `pnpm lint:design` (`apps/web/scripts/design-audit.mjs`) kiểm các quy tắc này; ngoại lệ có chủ đích ghi `design-audit-allow <rule>: <lý do>` ngay tại dòng đó.

### Kiểm thử

Vitest + jsdom + Testing Library; API giả bằng MSW (`src/shared/test/msw.ts`, request không có handler làm test fail). `renderRoutes` (`src/shared/test/render.tsx`) dựng route của feature trong memory router; `renderApp` (`src/app/render-app.tsx`) dựng cả cây route thật với middleware và query client cho test guard và phiên.
