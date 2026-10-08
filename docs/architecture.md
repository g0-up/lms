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
| Object storage | `S3_ENDPOINT`, `S3_PUBLIC_ENDPOINT`, `S3_REGION`, `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_USE_SSL`, `MEDIA_URL_TTL`, `MAX_VIDEO_BYTES`, `MAX_IMAGE_BYTES` |
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

| Gói | Vai trò |
|---|---|
| `domain` | `Email`, `Code` (stage/course/class), `LessonKey` + `Slugify`, `VersionNo`, `VersionStatus`, `ClassStatus`, `Role`, `UserStatus`, `MemberStatus`, `Percent`, `ValidatePassword`; lỗi `domain.Error{Kind, Msg}` với sentinel dùng `errors.Is` |
| `platform/apperr` | Kiểu lỗi ứng dụng duy nhất `*apperr.Error{Status, Code, Message, Details}`, hằng mã lỗi, `FromDomain` |
| `platform/db` | `Open`, `Migrate`, `Executor`, `Transact`/`TransactWithOptions`, `ExecAffectOne`/`ErrNoRowsAffected` |
| `platform/db/pgerr` | `constraints.go` (sổ tên constraint, sinh cùng migration) và `Map`: nơi duy nhất dịch lỗi Postgres |
| `platform/httpx` | `BindJSON`, `OK`, `NoContent`, `Fail`, `UUIDParam`; CSRF `CrossOriginProtection`, `RequireCustomHeader` |
| `platform/middleware` | `RequestID`, `Logger`, `Recover`, `SecurityHeaders`, `RateLimit`, `TrustProxies`/`ClientIP` |
| `platform/clock`, `platform/ids` | `Clock` (`Real{Loc}` theo `APP_TZ`, `Fake` cho test); `ids.New()` uuid v7, `ids.Parse` |
| `platform/audit` | `Recorder`, `PG` (ghi `audit_logs` trong transaction của caller), `Noop`, 20 hằng action |
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

Đổi thứ tự bài học dùng `SET CONSTRAINTS ` + `pgerr.UqLessonsVersionPosition` + ` DEFERRED` (tương tự `UqCvsVersionPosition`). Vi phạm constraint deferred chỉ lộ ra lúc COMMIT; `Transact` đưa lỗi commit qua `pgerr.Map` nên vẫn ra `409 CONFLICT`.

`audit.PG` chỉ nhận action thuộc 20 hằng trong `audit/actions.go` (`actions_test.go` khóa danh sách) và xóa đệ quy các khóa `password_hash`, `token_hash`, `password`, `temp_password` (cả dạng camelCase) khỏi `before`/`after` trước khi ghi jsonb.

### Quy tắc CSRF hai lớp

Mọi POST/PUT/PATCH/DELETE dưới `/api/v1` phải qua **cả hai** lớp: `http.CrossOriginProtection` (Sec-Fetch-Site/Origin) và header `X-Requested-With: fetch`. Thiếu một trong hai thì `403 FORBIDDEN`. `httpx.CrossOriginProtection` bọc cả engine trong `cmd/lms/serve.go`, `httpx.RequireCustomHeader()` gắn trên nhóm `/api/v1` trong `router.go`; `apps/web/src/shared/api/http.ts` luôn gửi header này. PUT thẳng lên MinIO/R2 bằng URL ký sẵn không đi qua API nên không cần header.

### Phiên, đăng nhập và email outbox

- `features/identity`: route `/auth/{login,logout,me,change-password,forgot-password,reset-password}` và `/users` (admin: liệt kê, `/{id}/disable`, `/{id}/enable`). Mật khẩu argon2id (m=19456, t=2, p=1) lưu dạng PHC; token phiên và token đặt lại là 32 byte ngẫu nhiên base64url, DB chỉ lưu sha256. Route nghiệp vụ gắn sau `SessionAuth → MustChangePassword → RequireRole(...)`; feature khác đọc người dùng bằng `identity.CurrentUser(c)`.
- Phiên: cookie `__Host-sid` khi `COOKIE_SECURE=true`, `sid` khi không; HttpOnly, SameSite=Lax, Path=/, không Max-Age. Hạn thật ở bảng `sessions` (`SESSION_TTL`), gia hạn trượt và ghi `users.last_active_at` tối đa một lần mỗi phút. Vô hiệu hóa, đặt lại mật khẩu và đổi mật khẩu (trừ phiên hiện tại) xóa phiên của user ngay trong transaction.
- Đăng nhập chạy trong một transaction giữ `pg_advisory_xact_lock` theo email, nên request song song không vượt `LOGIN_MAX_FAILURES` lần sai trong `LOGIN_LOCK_WINDOW`; vượt thì `429 TOO_MANY_ATTEMPTS` kèm `Retry-After`. Thứ tự kiểm: mật khẩu sai → `UNAUTHENTICATED` (không lộ trạng thái), rồi disabled → `ACCOUNT_DISABLED`, rồi mật khẩu tạm hết hạn → `TEMP_PASSWORD_EXPIRED`. `must_change_password=true` chỉ cho `GET /auth/me`, `POST /auth/change-password`, `POST /auth/logout`.
- `features/mailer`: use case ghi email bằng `mailer.Enqueuer.Enqueue(ctx, tx, Message)` trong transaction nghiệp vụ. Payload chỉ chứa dữ liệu không nhạy cảm; mật khẩu tạm hoặc token đặt lại đi vào `Message.Secret`, được niêm phong vào `secret_enc` và chỉ mở lúc gửi. `lms worker` claim hàng tới hạn (`FOR UPDATE SKIP LOCKED`, lease 2 phút), render template lúc gửi, gửi qua SMTP (hoặc chỉ log khi `SMTP_HOST` rỗng); lỗi thì thử lại sau 1 rồi 5 phút, lần thứ 3 thành `failed`. `secret_enc` bị xóa khi `sent` hoặc `failed`. Worker cũng dọn `login_attempts` quá 30 ngày và phiên hết hạn mỗi giờ. Gửi lại lời mời gọi `OutboxRepo.SupersedeQueued` để hàng cũ mang mật khẩu hết hiệu lực không được gửi; trạng thái gửi đọc qua `mailer.OutboxStatusReader`.

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

- `features/stages`: chặng, phiên bản chặng và học liệu; route admin `/stages` và `/stage-versions/{vid}/…` (phẳng). Mọi câu ghi lên phiên bản đi qua `SaveDraft` (ghi con trước, header sau, mỗi câu có điều kiện `status = 'draft'`) hoặc `TransitionStatus` (chỉ chạm header; publish kèm điều kiện không còn markdown chưa render). Publish render markdown bằng goldmark (GFM, không `WithUnsafe`) rồi bluemonday `NewPolicy` allowlist; ảnh duy nhất được giữ là `<img src="/api/v1/media/{uuid}/content">`. Khi lưu học liệu có source markdown mới (POST, hoặc PATCH có `markdownSource`), ảnh markdown phải là `/api/v1/media/{uuid}/content`: ảnh base64 hay ảnh ngoài trả 422; PATCH chỉ đổi `title`/`required` không kiểm lại source cũ. `POST /stages/markdown-preview` (admin, `{markdownSource}` → `{html}`) áp cùng luật ảnh rồi render bằng đúng goldmark + bluemonday của lúc phát hành, cho tab Xem trước của trang soạn. `lesson_media` được dựng lại theo video và ảnh trong HTML mỗi lần `SaveDraft`. Feature khác đọc qua `stages.VersionRefReader` (repository) và `stages.OutdatedReader` (service: khóa học đang dùng bản cũ, `canApply`/`blockedReason`).
- `features/media`: `POST /media/uploads` (admin) tạo bản ghi `pending` và trả URL PUT ký sẵn 15 phút; trình duyệt PUT thẳng lên storage; `POST /media/uploads/{id}/complete` `Stat` object rồi chuyển `ready`. `GET /media/{id}/url` trả URL GET ký hạn `MEDIA_URL_TTL`; `GET /media/{id}/content` redirect 302 tới URL đó (ảnh markdown trỏ vào đây nên HTML không chứa chữ ký). Video dùng `kind=video`; trình soạn markdown upload ảnh bằng `kind=image` qua cùng luồng. Admin xem mọi file; giảng viên/học viên chỉ xem file gắn với học liệu của lớp mình dạy hoặc học (lớp active/ended, thành viên active). Nhóm `/media` có `RateLimit` 120 request/phút theo user. Hợp đồng khác dùng `media.Reader`.
- `platform/storage`: interface `Storage` (`PresignPut`, `PresignGet`, `Stat`, `Delete`) và `Uploader` (`Put`, cho seed); `MinIO` dùng minio-go với hai client: client nội bộ (`S3_ENDPOINT`) gọi API từ server, client công khai (`S3_PUBLIC_ENDPOINT`) chỉ để ký URL; region cố định nên ký không gọi mạng. `MemStorage` cho test. Bucket luôn private.
- Cloudflare R2 (production): `S3_ENDPOINT=<account>.r2.cloudflarestorage.com`, `S3_PUBLIC_ENDPOINT=https://<account>.r2.cloudflarestorage.com`, `S3_REGION=auto`, `S3_USE_SSL=true`. CORS của bucket (cho phép `PUT`/`GET` từ `PUBLIC_BASE_URL`, header `Content-Type`) đặt trong dashboard Cloudflare; dev dùng `MINIO_API_CORS_ALLOW_ORIGIN` của compose.
- `lms seed --upload-sample` tải một video MP4 2 giây nhúng sẵn lên `storage_key` của mọi bài video mẫu và sửa `size_bytes` cho khớp; chạy lại an toàn.

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
- Phần trăm chỉ tính học liệu bắt buộc: `domain.Percent(done, total)` trong Go (`BuildRoadmap`) và `round(100.0 * done / total)` trong SQL cho kết quả trùng nhau; học liệu kế tiếp là học liệu đầu tiên chưa hoàn thành (kể cả tùy chọn) theo thứ tự chặng rồi vị trí, lớp nháp không có.
- Hợp đồng cho feature khác: `learning.ProgressReader` (`ClassProgress`, `MemberLessonProgress`, `ClassAveragePercent`, `ClassActivityCounts`) chạy trên `db.Executor` của caller; `app` nối `ClassAveragePercent` vào `classes.ProgressReader` cho `avgPercent` của `/teach/classes`. Golden JSON của response và lỗi học viên nằm ở `features/learning/testdata/` (cập nhật bằng `go test -tags integration ./internal/features/learning/ -update`), web học viên dùng làm fixture MSW.

### Báo cáo lớp và dashboard

- `features/reports`: chỉ đọc, không ghi bảng nào. `GET /classes/{id}/report` và `GET /classes/{id}/report/members/{mid}` cho admin và giảng viên phụ trách (giảng viên lớp khác → 403 "Bạn chỉ xem được lớp mình phụ trách.", học viên → 403); `GET /dashboard` chỉ admin. Mọi response có `Cache-Control: private, no-store` và `selfReported: true` ở báo cáo lớp và drilldown (tiến độ do học viên tự tích).
- Báo cáo lớp là một truy vấn CTE mỗi lớp: một dòng mỗi thành viên kèm % từng chặng theo thứ tự chặng, % tổng (chỉ học liệu bắt buộc, cùng công thức với learning), lời mời mới nhất (`sending` hiển thị `queued`), `lastLoginAt` và `lastActivityAt` (mốc mới hơn giữa `users.last_active_at` và lần mở/hoàn thành học liệu). Header, chặng, dòng và `summary` đọc trong một transaction `REPEATABLE READ READ ONLY` nên không lệch nhau.
- Query `notLoggedIn`, `inactiveDays` (≥ 1; chưa hoạt động lần nào cũng tính), `belowPercent` (0–100), `includeDropped` (mặc định `false`) và `sort` (`name`, `pct`, `pct-desc`, `activity`, `activity-asc`; hòa thì theo tên rồi id, `activity` để null cuối, `activity-asc` để null đầu). Bộ lọc kết hợp là giao. Giá trị không đọc được hoặc ngoài biên → 422 `VALIDATION_FAILED` "Tham số lọc không hợp lệ." trước khi đọc dữ liệu. `Filter.Apply` dịch bộ lọc thành tham số nullable `$3..$6` và chọn `ORDER BY` từ map allowlist, không nối chuỗi từ query; `Filter.Match` là cùng quy tắc trên Go và integration test đối chiếu hai bên. `summary` đếm theo cùng `includeDropped`, ngưỡng mặc định `STALE_DAYS` ngày và 50% được trả lại trong `summary.inactiveDays`/`belowPercent`.
- Drilldown ràng `mid` với lớp của URL ở SQL (thành viên lớp khác hoặc không tồn tại → 404) và liệt kê mọi học liệu của phiên bản khóa học qua `learning.ProgressReader.MemberLessonProgress`; thành viên đã rời lớp vẫn xem được.
- Dashboard: `kpis` (chặng, khóa học, lớp active, học viên active khác nhau trong lớp active), `hints` (lớp nháp, học viên lớp active chưa đăng nhập, khóa học lỗi thời, lời mời thất bại theo lời mời mới nhất của thành viên active trong lớp chưa kết thúc), `outdated[]` từ `stages.OutdatedReader.AllOutdated`, `classes[]` với `avgPercent` từ `learning.ProgressReader.ClassAveragePercent`, `recentActivity` 8 dòng audit mới nhất. Audit đọc qua `audit.Reader.Latest`; tên đối tượng tra theo lô, người thao tác qua `identity.UserReader.SnapshotByIDs`; nhãn hành động là bảng tiếng Việt cố định, response không có `before`/`after` thô mà chỉ `summary` đã dựng (đối tượng đã bị xóa thì dựng từ payload).
- Golden JSON nằm ở `features/reports/testdata/` (dữ liệu `lms seed` trên `lms_test`; cập nhật bằng `go test -tags integration ./internal/features/reports/ -update`), web dùng làm fixture zod/MSW.

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

`src/app/router.tsx` gom route của bảy feature; comment đầu file là hợp đồng đầy đủ.

| Vùng | Guard | Layout | Route từ feature |
|---|---|---|---|
| `/login`, `/forgot` | khách (đã đăng nhập thì về trang chủ của vai trò) | AuthLayout | `auth.routes` |
| `/reset-password` | không cần phiên (token quyết định) | AuthLayout | `auth.publicRoutes` |
| `/first-login` | có phiên và còn mật khẩu tạm | AuthLayout | `auth.sessionRoutes` |
| `/admin` | `requireRole('admin')` | AppShell | `dashboard`, `stages`, `courses`, `classes`, `reports.routes` |
| `/teach` | `requireRole('teacher')` | AppShell | `reports.teachRoutes` |
| `/learn` | `requireRole('student')` | AppShell | `learning.routes` |

- `routes.tsx` của feature export `RouteObject[]` với path tương đối trong vùng (không lặp `/admin`…); trang gốc của vùng là `{ index: true }`. Trang tải lazy và export `Component` (hoặc dùng `lazyPage` cho `default`).
- `handle: { title }` đặt `document.title` là "{title} · GoUp LMS"; `RouteAnnouncer` đọc tiêu đề qua `aria-live` và chuyển focus tới `h1` của trang sau mỗi lần điều hướng.
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
