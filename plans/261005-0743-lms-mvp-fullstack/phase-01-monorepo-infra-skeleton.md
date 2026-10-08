---
phase: 01
title: "Phase 1: Monorepo, tooling và hạ tầng"
status: pending
priority: P1
effort: "2 ngày"
dependencies: []
---

# Phase 1: Monorepo, tooling và hạ tầng

## Goal

Dựng skeleton chạy được của monorepo theo cây ở [plan.md §5.1](./plan.md) và hạ tầng theo [plan.md §5.4](./plan.md): `apps/api` (Go + cobra + Gin + sqlx + go-migrate nhúng) trả `/healthz`, `apps/web` (Vite + React + Tailwind v4 + shadcn Radix) hiển thị trang trống, compose dev dựng Postgres + MinIO + Mailpit, compose prod/homelab + Caddy (có service `worker` riêng cho outbox email), Makefile gốc và CI xanh. Phase này là **chủ sở hữu duy nhất** của `platform/config` và `infra/.env.example`: mọi phase sau chỉ tham chiếu tên field/biến đã khai báo ở đây, không tự thêm biến mới. <!-- Red Team: RT-11 - Phase 01 sở hữu Config + .env.example --> Chưa có nghiệp vụ; mọi phase sau chỉ thêm file vào khung này.

## Context & Requirements

- Mẫu sao chép: `~/Documents/personal-workspace/sidecup/{Makefile,infra/*,apps/api/{Dockerfile,Makefile,.golangci.yml},apps/web/{Dockerfile,nginx.conf,vite.config.ts,components.json},.github/workflows/*}`. Giữ cấu trúc, đổi tên `sidecup` → `lms`, binary `/api` → `/lms`, bỏ WebSocket (`/ws/*`), notifier, Zalo, `SELLER_*`, `SESSION_SECRET` (session lưu DB nên không cần secret ký). Sidecup dùng GORM và `switch` lệnh; ở đây dùng sqlx và cobra theo yêu cầu người dùng.
- NFR liên quan: HTTPS qua Caddy (spec §8 bảo mật), cấu hình qua env, log JSON không chứa secret, không commit `.env`.
- Phiên bản (báo cáo nghiên cứu, 2026-10-05): Go 1.27, gin v1.12.0, sqlx v1.4.0, pgx v5.11.0, golang-migrate v4.20.1, cobra v1.10.2, caarlos0/env v11.4.1, testify v1.12.1, golangci-lint v2.14.0; Node 22, pnpm, Vite 8.3, React 19.3, TypeScript ~6.0.3 (không dùng TS 7), Tailwind 4.3, shadcn CLI 4.x với `-b radix`, Vitest 5, Playwright 1.63, ESLint 10 + typescript-eslint 8.71.
- Quyết định: mọi ngưỡng nghiệp vụ (Q7) là env có mặc định; `APP_ENV` bắt buộc, allowlist `dev|e2e|production`; dev chạy API và web ngoài container (nhanh), chỉ Postgres/MinIO/Mailpit trong Docker; profile `full` cho E2E; MinIO ghim tag `RELEASE.2025-04-22T22-12-26Z` (bản cuối trước khi archive; chỉ dùng dev/e2e). <!-- Red Team: RT-11 - APP_ENV required allowlist -->
- Object storage production là **Cloudflare R2** (S3-compatible): code chỉ chạm S3 API qua interface `Storage` (Phase 05, minio-go v7), nên dev dùng MinIO còn prod trỏ `S3_ENDPOINT=<account>.r2.cloudflarestorage.com`, `S3_REGION=auto`, `S3_USE_SSL=true`. Không có service storage trong prod compose, không có storage spike/ADR. <!-- Updated: Validation Session 1 - prod storage = Cloudflare R2 -->
- CORS cho MinIO dev cấu hình bằng env `MINIO_API_CORS_ALLOW_ORIGIN` trên service `minio` (MinIO community không hỗ trợ `mc cors set`). <!-- Red Team: RT-04 - bỏ mc cors set -->
- Phạm vi tooling chỉ gồm những gì sidecup có: compose dev/prod/homelab, Caddyfile, `.env.example`, CI (`ci.yml` có gitleaks, `e2e.yml`), docs `README/architecture/database/api/runbook`. Không thêm `infra/scripts/backup.sh|restore.sh`, `scripts/docs-links-check.sh`, `govulncheck`, `pnpm audit`, `lms routes --json`, `migrate force`, `CHANGELOG.md`, `docs/security.md`, `docs/pilot-checklist.md`, `docs/decisions/*`. <!-- Updated: Validation Session 1 - bỏ mọi bổ sung ngoài spec/sidecup -->

## Architecture / Design

### Compose dev (`infra/docker-compose.yml`)

| Service | Image | Port host | Ghi chú |
|---|---|---|---|
| `postgres` | `postgres:16-alpine` | 5432 | `POSTGRES_DB/USER/PASSWORD=lms`; healthcheck `pg_isready`; volume `pgdata` |
| `minio` | `minio/minio:RELEASE.2025-04-22T22-12-26Z` | 9000 (S3), 9001 (console) | `MINIO_ROOT_USER/PASSWORD=lmsminio/lmsminio123`; `MINIO_API_CORS_ALLOW_ORIGIN=http://localhost:5173,http://localhost:8081` <!-- Red Team: RT-04 - CORS qua env -->; healthcheck `mc ready local`; volume `miniodata` |
| `minio-init` | `minio/mc:RELEASE.2025-04-16T18-13-26Z` | – | chạy một lần, chỉ 3 lệnh: `mc alias set`, `mc mb --ignore-existing local/lms-media`, `mc anonymous set none`. Không có `mc cors set` (MinIO community không hỗ trợ) <!-- Red Team: RT-04 - bỏ mc cors set và cors.json --> |
| `mailpit` | `axllent/mailpit:v1.28` | 1025 (SMTP), 8025 (UI) | `MP_SMTP_AUTH_ACCEPT_ANY=1`, `MP_SMTP_AUTH_ALLOW_INSECURE=1` |
| `api` (profile `full`) | build `../apps/api` | – | `APP_ENV=e2e`, `MIGRATE_ON_START=true`, `PUBLIC_BASE_URL=http://localhost:8081`, S3 endpoint `minio:9000`, `S3_PUBLIC_ENDPOINT=http://localhost:9000`, `S3_REGION=us-east-1`, SMTP `mailpit:1025`, `SEED_PASSWORD` (chỉ e2e); healthcheck `["/lms","healthcheck"]`; `depends_on`: `postgres: condition: service_healthy`, `minio-init: condition: service_completed_successfully`, `mailpit: condition: service_started` <!-- Red Team: RT-04 - postgres service_healthy --> |
| `worker` (profile `full`) | cùng image với `api` (`build ../apps/api`) | – | `command: ["worker"]`, cùng khối env với `api` (dùng YAML anchor `x-api-env`), `MIGRATE_ON_START=false`, `depends_on api: condition: service_healthy`; không có healthcheck HTTP, `restart: unless-stopped` <!-- Red Team: RT-04 - service worker riêng --> |
| `web` (profile `full`) | build `../apps/web` | 8081 → 80 | `depends_on api: service_healthy` |

### Compose prod (`infra/docker-compose.prod.yml`)

Giống sidecup: `postgres` (không publish, healthcheck `pg_isready`), `api` (`APP_ENV=production`, `MIGRATE_ON_START=true`, `depends_on postgres: condition: service_healthy`, mọi secret `${VAR:?đặt VAR trong infra/.env}`), `worker` (cùng image `api`, `command: ["worker"]`, cùng env qua anchor, `MIGRATE_ON_START=false`, `depends_on api: condition: service_healthy`) <!-- Red Team: RT-04 - worker trong prod -->, `web`, `caddy:2-alpine` 80/443(+udp), volumes `pgdata caddy_data caddy_config`. Không có service storage trong prod compose: `S3_*` trỏ tới Cloudflare R2 (`S3_ENDPOINT=<account>.r2.cloudflarestorage.com`, `S3_PUBLIC_ENDPOINT` cùng endpoint hoặc custom domain, `S3_REGION=auto`, `S3_USE_SSL=true`, `S3_BUCKET=lms-media`); CORS của bucket R2 (PUT từ origin web, GET) đặt trên Cloudflare dashboard, JSON mẫu ghi trong `docs/runbook.md` (Phase 14). <!-- Updated: Validation Session 1 - R2 thay server S3 ngoài --> `docker-compose.homelab.yml` là overlay Traefik: tắt caddy bằng profile, `ports: !reset []`, alias `lms-postgres`, labels router `lms-web`/`lms-api`, giữ nguyên service `worker`, cùng bộ header với Caddyfile.

### Caddyfile

```caddyfile
{ email {$ACME_EMAIL} }
{$DOMAIN} {
  encode zstd gzip
  header {
    Strict-Transport-Security "max-age=31536000; includeSubDomains"
    X-Content-Type-Options "nosniff"
    Referrer-Policy "strict-origin-when-cross-origin"
    X-Frame-Options "DENY"
    Permissions-Policy "camera=(), microphone=(), geolocation=()"
    Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' blob:; media-src 'self' {$MEDIA_ORIGIN}; connect-src 'self' {$MEDIA_ORIGIN}; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"
    -Server
  }
  handle /internal/* { respond 404 }
  handle /api/* { reverse_proxy api:8080 }
  handle { reverse_proxy web:80 }
}
```

`MEDIA_ORIGIN` (ví dụ `https://<account>.r2.cloudflarestorage.com` hoặc custom domain R2) là origin của `S3_PUBLIC_ENDPOINT`; `connect-src` cần nó vì upload dùng presigned PUT từ trình duyệt, `media-src` cần nó cho `<video src>`. `img-src` chỉ `'self' blob:` vì ảnh trong Markdown bắt buộc đi qua `/api/v1/media/{id}/content` (Phase 05 sanitizer chỉ cho phép `src` dạng này). <!-- Red Team: RT-15 - CSP img-src 'self' blob: -->

### Makefile gốc (target và ý nghĩa)

| Target | Hành động |
|---|---|
| `help` | in danh sách |
| `check-ports` | kiểm 8080, 5173, 5432, 9000, 9001, 1025, 8025; báo tiến trình giữ cổng và thoát 1, không tự đổi cổng (theo `.claude/rules/process-management.md`) |
| `db-up` | `compose up -d --wait postgres minio minio-init mailpit` |
| `dev` | `check-ports db-up migrate-up` rồi chạy song song `apps/api make run` và `apps/web pnpm dev`, `trap 'kill 0'` |
| `dev-api`, `dev-web`, `dev-down` (alias `down`) | riêng lẻ |
| `migrate-up`, `migrate-down`, `migrate-version`, `seed`, `seed-reset`, `worker` | ủy quyền sang `apps/api/Makefile` (`go run ./cmd/lms migrate up` …); `seed-reset` = `lms seed --reset` (chỉ chạy được khi `APP_ENV` ∈ `dev|e2e`) <!-- Red Team: RT-14 - target seed-reset --> |
| `test`, `test-api`, `test-web` | api: unit luôn, integration khi có `TEST_DATABASE_URL`; web: `pnpm test` |
| `lint` | `apps/api make lint` (`go vet`, `go vet -tags integration`, `golangci-lint run`) + `pnpm lint && pnpm typecheck` |
| `check` | `lint test` |
| `build` | `docker build -t lms-api apps/api`, `docker build -t lms-web apps/web` |
| `e2e` | `compose --profile full up -d --build --wait`, `exec api /lms seed --reset`, `E2E_BASE_URL=http://localhost:8081 pnpm e2e`, `down` giữ exit code <!-- Red Team: RT-14 - seed --reset trước E2E --> |
| `prod-up`, `prod-down`, `homelab-config`, `homelab-up`, `homelab-down` | như sidecup với `--env-file infra/.env` |

### cobra skeleton (`apps/api/cmd/lms`)

```text
main.go        -> cmd.Execute()
root.go        -> rootCmd "lms"; PersistentPreRunE: config.Load(), slog JSON handler theo LOG_LEVEL
serve.go       -> app.New(cfg, deps).Run(ctx): migrate nếu MIGRATE_ON_START, gin engine, http.Server, graceful shutdown 10s
migrate.go     -> up | down [--steps N] | version (gọi platform/db/migrate.go); không có `force` <!-- Updated: Validation Session 1 - bỏ migrate force -->
seed.go        -> từ chối khi APP_ENV=production; flag --reset (chỉ dev|e2e) và --upload-sample; Phase 02 điền nội dung
worker.go      -> vòng lặp outbox email + dọn login_attempts; Phase 04 điền nội dung; phase này chỉ in "worker: chưa có job" và block tới khi nhận SIGTERM (để service compose `worker` không restart-loop)
healthcheck.go -> GET http://127.0.0.1:8080/healthz, exit 0/1 (dùng trong HEALTHCHECK distroless không có curl)
```

`internal/app/router.go` là nơi duy nhất nối middleware và feature: `New(d Deps) *gin.Engine` gắn `RequestID, Logger, Recover, SecurityHeaders`, `GET /healthz` (200 `{"status":"ok"}`), `GET /readyz` (ping DB, 503 nếu lỗi), group `/api/v1` trống cho phase sau. `Deps` giữ `Cfg`, `DB *sqlx.DB`, `Clock`, `Logger`.

### Web skeleton

- `pnpm create vite@latest apps/web --template react-ts`; `pnpm add -D @tailwindcss/vite tailwindcss vitest jsdom @testing-library/react @testing-library/user-event @testing-library/jest-dom @playwright/test eslint @eslint/js typescript-eslint eslint-plugin-react-hooks eslint-plugin-react-refresh`; `pnpm dlx shadcn@latest init -b radix` chọn `new-york`, base color `neutral`, CSS variables; chỉnh `components.json` aliases về `@/shared`, `@/shared/lib/utils`, `@/shared/ui`, `@/shared/lib`, `@/shared/hooks`, `tailwind.css: src/styles/app.css`.
- `vite.config.ts`: alias `@` → `src`, `server.port 5173 strictPort`, proxy `/api` → `VITE_API_PROXY ?? http://localhost:8080` với `xfwd`, preview 4173, `manualChunks` gom `react|react-dom|scheduler|react-router` vào `react-vendor`.
- `package.json` scripts: `dev`, `build` (`tsc -b && vite build`), `typecheck` (`tsc -b --noEmit`), `lint` (`eslint .`), `test` (`vitest run`), `e2e` (`playwright test`), `preview`.
- `eslint.config.js` flat: `@eslint/js` recommended, `typescript-eslint` recommended-type-checked, `react-hooks` flat recommended, `react-refresh`, luật `no-restricted-imports` cấm `@/features/*/*` (chỉ qua `index.ts`), cấm `../../features`.
- `playwright.config.ts`: `baseURL = E2E_BASE_URL ?? http://localhost:5173`, projects `chromium` và `webkit`, `testDir e2e`, trace on-first-retry.
- Dockerfile như sidecup (bỏ `ARG VITE_SELLER_NAME`, bỏ check-size), `nginx.conf` như sidecup bỏ `/ws/`.

## Files to Create / Modify

```text
.gitignore                                    # .env, .env.*, !.env.example, node_modules, dist, bin, coverage, test-results, playwright-report, .DS_Store
.editorconfig                                 # utf-8, lf, 2 spaces (ts/yaml/json), tab (go, Makefile)
Makefile                                      # target ở trên
README.md                                     # chạy nhanh: make dev, URL dev (5173, 8080, 8025, 9001), cấu trúc repo, link docs/
.github/workflows/ci.yml                      # job api (postgres service, vet, golangci-lint v2.14, test -tags integration), web (pnpm lint/typecheck/test/build), secrets (gitleaks). Không có govulncheck/pnpm audit (giống sidecup)
.github/workflows/e2e.yml                     # make e2e, upload playwright-report khi fail
docs/README.md                                # điều hướng docs: architecture, database (Phase 02), api (Phase 14), runbook (Phase 14), design, prototype
docs/architecture.md                          # một file duy nhất: layout monorepo, hạ tầng compose/Caddy/R2; Phase 03 thêm mục backend, Phase 10 thêm mục frontend
infra/.env.example                            # mọi biến (danh sách ở Task 3)
infra/docker-compose.yml                      # dev + profile full (api, worker, web)
infra/docker-compose.prod.yml                 # prod + worker + caddy, không minio
infra/docker-compose.homelab.yml              # overlay Traefik, giữ worker
infra/caddy/Caddyfile
apps/api/go.mod, go.sum                       # module lms/api, go 1.27
apps/api/Dockerfile                           # golang:1.27-alpine → distroless static nonroot, ENTRYPOINT ["/lms"], CMD ["serve"]
apps/api/Makefile                             # run, migrate-up/down/version, seed, seed-reset, worker, test, test-unit, test-integration, lint, fmt, bin
apps/api/.golangci.yml                        # version "2", linters như sidecup, build-tags integration
apps/api/.env.example                         # bản dev local của các biến API (DATABASE_URL localhost, S3 localhost:9000, SMTP localhost:1025)
apps/api/cmd/lms/{main,root,serve,migrate,seed,worker,healthcheck}.go
apps/api/internal/app/{router,deps}.go
apps/api/internal/platform/config/{config,config_test,env_example_test}.go   # env_example_test: khóa trong infra/.env.example ⊆ tag env của Config <!-- Red Team: RT-11 - test đồng bộ .env.example -->
apps/api/internal/platform/db/{db,migrate}.go  # Open(dsn) sqlx+pgx; Migrate(ctx, dsn, direction, steps) mở *sql.DB riêng, iofs+pgx5; chi tiết Transact ở Phase 03 <!-- Red Team: RT-04 - Migrate mở kết nối riêng -->
apps/api/migrations/.gitkeep                   # SQL thật ở Phase 02; embed.go với //go:embed *.sql đặt tại apps/api/migrations/embed.go
apps/api/migrations/embed.go
apps/web/package.json, pnpm-lock.yaml, tsconfig*.json, vite.config.ts, index.html
apps/web/components.json, eslint.config.js, vitest.config.ts, playwright.config.ts
apps/web/src/main.tsx, src/app/router.tsx (route "/" render "GoUp LMS"), src/styles/app.css (@import "tailwindcss" + @import "./goup-tokens.css")
apps/web/src/styles/goup-tokens.css           # chép nguyên khối :root của prototype/goup.css (dòng 4–51), không sửa
apps/web/src/shared/lib/utils.ts              # cn re-export do shadcn sinh
apps/web/src/test/setup.ts                    # jest-dom
apps/web/e2e/smoke.spec.ts                    # mở "/" thấy "GoUp LMS"
apps/web/Dockerfile, nginx.conf
```

## Tasks & Steps

1. **Khởi tạo repo**: thêm `.gitignore`, `.editorconfig`, `README.md` khung. Kiểm `git check-ignore infra/.env apps/api/.env` trả về đường dẫn (bị ignore).
2. **`apps/api` skeleton**
   1. `cd apps/api && go mod init lms/api && go get github.com/gin-gonic/gin@v1.12.0 github.com/jmoiron/sqlx@v1.4.0 github.com/jackc/pgx/v5@v5.11.0 github.com/golang-migrate/migrate/v4@v4.20.1 github.com/spf13/cobra@v1.10.2 github.com/caarlos0/env/v11@v11.4.1 github.com/stretchr/testify@v1.12.1`. Sửa `go.mod` dòng `go 1.27`.
   2. `platform/config/config.go`:
      ```go
      // Danh sách biến DUY NHẤT của toàn dự án. Phase sau chỉ tham chiếu field, không thêm biến.
      type Config struct {
        AppEnv string `env:"APP_ENV,required"`                   // allowlist: dev|e2e|production
        HTTPAddr string `env:"HTTP_ADDR" envDefault:":8080"`
        PublicBaseURL string `env:"PUBLIC_BASE_URL,required"`
        DatabaseURL string `env:"DATABASE_URL,required"`
        MigrateOnStart bool `env:"MIGRATE_ON_START" envDefault:"false"`
        LogLevel string `env:"LOG_LEVEL" envDefault:"info"`
        AppTZ string `env:"APP_TZ" envDefault:"Asia/Ho_Chi_Minh"`

        SessionTTL time.Duration `env:"SESSION_TTL" envDefault:"12h"`
        CookieSecure bool `env:"COOKIE_SECURE" envDefault:"false"`   // nguồn DUY NHẤT quyết định cờ Secure của cookie
        TrustedProxies []string `env:"TRUSTED_PROXIES" envSeparator:","` // mặc định rỗng; prod đặt CIDR mạng Caddy/Traefik
        PasswordMinLength int `env:"PASSWORD_MIN_LENGTH" envDefault:"8"`
        TempPasswordTTL time.Duration `env:"TEMP_PASSWORD_TTL" envDefault:"72h"`
        ResetTokenTTL time.Duration `env:"RESET_TOKEN_TTL" envDefault:"30m"`
        LoginMaxFailures int `env:"LOGIN_MAX_FAILURES" envDefault:"5"`
        LoginLockWindow time.Duration `env:"LOGIN_LOCK_WINDOW" envDefault:"15m"`
        RateLimitLoginIPPerMin int `env:"RATE_LIMIT_LOGIN_IP_PER_MIN" envDefault:"300"`
        RateLimitForgotIPPerMin int `env:"RATE_LIMIT_FORGOT_IP_PER_MIN" envDefault:"30"`

        SMTPHost string `env:"SMTP_HOST" envDefault:"localhost"`
        SMTPPort int `env:"SMTP_PORT" envDefault:"1025"`
        SMTPUser string `env:"SMTP_USER"`
        SMTPPassword string `env:"SMTP_PASSWORD"`
        MailFrom string `env:"MAIL_FROM" envDefault:"GoUp LMS <no-reply@localhost>"`
        MailFailPattern string `env:"MAIL_FAIL_PATTERN"`           // chỉ có hiệu lực khi AppEnv == e2e
        EmailPollInterval time.Duration `env:"EMAIL_POLL_INTERVAL" envDefault:"5s"`
        OutboxSecretKey string `env:"OUTBOX_SECRET_KEY,required"`  // base64 của 32 byte; AES-256-GCM cho email_outbox.secret_enc

        S3Endpoint string `env:"S3_ENDPOINT,required"`           // host[:port] API nội bộ (dev minio:9000, prod <account>.r2.cloudflarestorage.com)
        S3PublicEndpoint string `env:"S3_PUBLIC_ENDPOINT,required"` // URL trình duyệt thấy, dùng để ký
        S3Region string `env:"S3_REGION" envDefault:"us-east-1"`  // R2: auto
        S3Bucket string `env:"S3_BUCKET" envDefault:"lms-media"`
        S3AccessKey string `env:"S3_ACCESS_KEY,required"`
        S3SecretKey string `env:"S3_SECRET_KEY,required"`
        S3UseSSL bool `env:"S3_USE_SSL" envDefault:"false"`
        MediaURLTTL time.Duration `env:"MEDIA_URL_TTL" envDefault:"2h"`
        MaxVideoBytes int64 `env:"MAX_VIDEO_BYTES" envDefault:"2147483648"`
        MaxImageBytes int64 `env:"MAX_IMAGE_BYTES" envDefault:"10485760"`

        StaleDays int `env:"STALE_DAYS" envDefault:"7"`
        SeedPassword string `env:"SEED_PASSWORD"`                   // chỉ lệnh seed, dev/e2e
        Location *time.Location `env:"-"`
      }
      ```
      <!-- Red Team: RT-11 - Config canonical, đổi tên LOGIN_MAX_FAILS→LOGIN_MAX_FAILURES, MIN_PASSWORD_LENGTH→PASSWORD_MIN_LENGTH, thêm COOKIE_SECURE/TRUSTED_PROXIES/STALE_DAYS/MAIL_FAIL_PATTERN/EMAIL_POLL_INTERVAL/OUTBOX_SECRET_KEY, bỏ TEMP_PASSWORD_LENGTH/EMAIL_MAX_ATTEMPTS/SMTP_TLS/PRIVACY_POLICY_URL/RATE_LIMIT_INVITE_PER_MIN -->
      <!-- Updated: Validation Session 1 - SESSION_TTL=12h, RATE_LIMIT_LOGIN_IP_PER_MIN=300 (lockout theo email, limiter IP cao), MEDIA_URL_TTL giữ 2h -->
      Hằng số nghiệp vụ không phải env: độ dài mật khẩu tạm 12, số lần gửi email tối đa 3, SMTP dùng STARTTLS tự động khi server hỗ trợ (go-mail `TLSOpportunistic`). `Load()` parse, `validate()`: `AppEnv` ∈ allowlist (sai → lỗi "APP_ENV phải là dev, e2e hoặc production"); `PublicBaseURL` absolute; `PasswordMinLength >= 8`; `OutboxSecretKey` decode base64 ra đúng 32 byte; `production` bắt buộc `PublicBaseURL` https, `CookieSecure == true`, `S3UseSSL == true`, `SMTPUser` không rỗng. Cờ Secure của cookie đọc **chỉ** từ `CookieSecure`, không suy ra từ URL. <!-- Red Team: RT-11 - bỏ SecureCookies() --> Helper: `IsProduction()`, `IsE2E()`, `SeedResetAllowed()` = `AppEnv ∈ {dev, e2e}`. Test bảng cho từng nhánh validate; thêm test `env_example_test.go`: parse `infra/.env.example` và `apps/api/.env.example`, mọi key phải là tag `env:` của `Config` (hoặc nằm trong allowlist biến compose-only `DOMAIN ACME_EMAIL MEDIA_ORIGIN POSTGRES_* TEST_DATABASE_URL`) để file mẫu không trôi khỏi struct. <!-- Red Team: RT-11 - test .env.example ⊆ struct -->
   3. `platform/db/db.go`: `Open(ctx, dsn) (*sqlx.DB, error)` dùng `pgx.ParseConfig` → `stdlib.OpenDB` → `sqlx.NewDb(sqlDB, "pgx")`, `SetMaxOpenConns(20)`, `SetConnMaxIdleTime(5m)`, `PingContext`. `platform/db/migrate.go`: `Migrate(ctx, dsn string, direction string, steps int) error` **tự mở một `*sql.DB` riêng** bằng `sql.Open("pgx", dsn)` chỉ cho golang-migrate, `pgxv5.WithInstance(sqlDB, &pgxv5.Config{})`, `migrate.NewWithInstance("iofs", iofs.New(migrations.FS, "."), "pgx5", drv)`, và `defer m.Close()` (đóng cả handle riêng này). Không bao giờ truyền pool của app vào migrate vì `WithInstance` dùng chung pool và `Close()` sẽ đóng pool đó. <!-- Red Team: RT-04 - migrate dùng *sql.DB riêng --> `Version(ctx, dsn) (uint, bool, error)`; khi DB chưa có migration nào golang-migrate trả `migrate.ErrNilVersion`, CLI in `no migration` và exit 0. `migrations/embed.go`: `package migrations; //go:embed *.sql; var FS embed.FS` (thêm file `0000_init.up.sql` rỗng chứa `SELECT 1;` và `.down.sql` để embed không lỗi vì glob rỗng; Phase 02 giữ file này).
   4. `cmd/lms/*.go` theo bảng trên. `serve.go`: `signal.NotifyContext`, `srv := &http.Server{ReadHeaderTimeout: 5s, Handler: engine}`, shutdown 10s. `healthcheck.go`: `http.Get("http://127.0.0.1" + cfg.HTTPAddr + "/healthz")`, timeout 2s.
   5. `internal/app/router.go`: middleware tối thiểu dùng inline ở phase này (RequestID = `X-Request-ID` hoặc uuid; Logger slog với method, path, status, latency, request_id, không log body/query chứa token; Recover trả 500 envelope `{"error":{"code":"INTERNAL","message":"Lỗi hệ thống"}}`; SecurityHeaders `X-Content-Type-Options nosniff`, `Cache-Control no-store` cho `/api/*`). Phase 03 chuyển sang `platform/middleware` và `platform/httpx`.
   6. `Dockerfile`: `golang:1.27-alpine` build `-trimpath -ldflags="-s -w" -o /out/lms ./cmd/lms`; runtime distroless `COPY /out/lms /lms`, `USER nonroot`, `ENTRYPOINT ["/lms"]`, `CMD ["serve"]`. `.golangci.yml` `version: "2"`, `default: standard`, enable `bodyclose errorlint gosec misspell noctx rowserrcheck sqlclosecheck unconvert`, exclusions `_test.go` gosec/noctx, formatters gofmt goimports.
   7. Kiểm: `go build ./... && go vet ./... && golangci-lint run && go test ./...`; `APP_ENV=dev DATABASE_URL=... go run ./cmd/lms serve` rồi `curl -s localhost:8080/healthz` → `{"status":"ok"}`; `go run ./cmd/lms migrate version` → `0` sau khi apply `0000`; `migrate down` rồi `migrate version` → `no migration` (ErrNilVersion), exit 0. <!-- Red Team: RT-04 - down về nil version -->
3. **`infra/`**: viết 4 compose/overlay + Caddyfile + `.env.example` (không có `minio/cors.json`). `.env.example` liệt kê đúng tập biến của `Config` ở Task 2 cộng biến compose-only: `DOMAIN`, `ACME_EMAIL`, `MEDIA_ORIGIN`, `POSTGRES_DB/USER/PASSWORD` (ghi `openssl rand -hex 24`), `APP_ENV=production`, `PUBLIC_BASE_URL`, `MIGRATE_ON_START`, `LOG_LEVEL`, `APP_TZ`, `SESSION_TTL=12h`, `COOKIE_SECURE=true`, `TRUSTED_PROXIES=` (prod: CIDR mạng compose của Caddy, ví dụ `172.16.0.0/12`), `PASSWORD_MIN_LENGTH=8`, `TEMP_PASSWORD_TTL=72h`, `RESET_TOKEN_TTL=30m`, `LOGIN_MAX_FAILURES=5`, `LOGIN_LOCK_WINDOW=15m`, `RATE_LIMIT_LOGIN_IP_PER_MIN=300`, `RATE_LIMIT_FORGOT_IP_PER_MIN=30`, `SMTP_HOST/PORT/USER/PASSWORD`, `MAIL_FROM`, `MAIL_FAIL_PATTERN=` (chỉ e2e), `EMAIL_POLL_INTERVAL=5s`, `OUTBOX_SECRET_KEY=` (ghi `openssl rand -base64 32`), `S3_ENDPOINT=<account>.r2.cloudflarestorage.com`, `S3_PUBLIC_ENDPOINT=https://<account>.r2.cloudflarestorage.com`, `S3_REGION=auto`, `S3_BUCKET=lms-media`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_USE_SSL=true`, `MEDIA_URL_TTL=2h`, `MAX_VIDEO_BYTES=2147483648`, `MAX_IMAGE_BYTES=10485760`, `STALE_DAYS=7`. <!-- Red Team: RT-11 - danh sách env canonical --> <!-- Updated: Validation Session 1 - R2 trong .env.example --> Mỗi biến có chú thích một dòng tiếng Việt và giá trị mặc định an toàn hoặc rỗng cho secret. `apps/api/.env.example` là bản localhost (`APP_ENV=dev`, `DATABASE_URL=postgres://lms:lms@localhost:5432/lms?sslmode=disable`, `TEST_DATABASE_URL=postgres://lms:lms@localhost:5432/lms_test?sslmode=disable`, `PUBLIC_BASE_URL=http://localhost:5173`, `COOKIE_SECURE=false`, `S3_ENDPOINT=localhost:9000`, `S3_PUBLIC_ENDPOINT=http://localhost:9000`, `S3_REGION=us-east-1`, `S3_USE_SSL=false`, `SMTP_HOST=localhost`, `SMTP_PORT=1025`, `OUTBOX_SECRET_KEY=` và `SEED_PASSWORD=` để trống, dev tự điền; README ghi lệnh sinh key). Compose profile `full` đặt `OUTBOX_SECRET_KEY` cố định cho e2e với chú thích "chỉ e2e".
   Thêm service `postgres` dev tạo thêm DB `lms_test` qua `infra/postgres/init/01-test-db.sql` mount vào `/docker-entrypoint-initdb.d/` (`CREATE DATABASE lms_test;`).
   Kiểm: `docker compose -f infra/docker-compose.yml config --quiet`; `make db-up` xong `docker compose ps` thấy 3 service healthy và `minio-init` exited 0; `curl -s localhost:8025/api/v1/info` 200; `mc` log có `Bucket created successfully`; `curl -si -X OPTIONS -H 'Origin: http://localhost:5173' -H 'Access-Control-Request-Method: PUT' localhost:9000/lms-media/x` trả `Access-Control-Allow-Origin: http://localhost:5173` (CORS qua env hoạt động). <!-- Red Team: RT-04/RT-08 - kiểm CORS MinIO -->
4. **Makefile gốc** theo bảng; `check-ports` dùng `lsof -nP -iTCP:$p -sTCP:LISTEN`. `apps/api/Makefile` có `WITH_ENV = set -a; [ -f .env ] && . ./.env; set +a;`, target `run` = `go run ./cmd/lms serve`, `worker` = `go run ./cmd/lms worker`, `seed` = `go run ./cmd/lms seed`, `seed-reset` = `go run ./cmd/lms seed --reset`.
5. **`apps/web` skeleton** theo mục Web skeleton. Sau `shadcn init -b radix`, kiểm `components.json` có `"style": "new-york"`, không có `"rsc": true`, và `package.json` chứa `radix-ui` (không phải nhiều gói `@radix-ui/react-*`). Tạo `src/styles/goup-tokens.css` bằng cách chép dòng 4–51 của `prototype/goup.css` (đổi `--font-sans` v.v. không được phép, giữ nguyên). `app.css` chỉ `@import "tailwindcss"; @import "./goup-tokens.css";` (ánh xạ `@theme inline` là việc của Phase 10). Thêm `src/test/setup.ts`, một unit test mẫu `src/app/router.test.tsx` render route `/` thấy chữ "GoUp LMS", `e2e/smoke.spec.ts`.
   Kiểm: `pnpm lint && pnpm typecheck && pnpm test && pnpm build`; `pnpm dev` mở `http://localhost:5173` thấy "GoUp LMS"; `curl -s localhost:5173/api/healthz` qua proxy trả JSON của API.
6. **Dockerfile web + nginx.conf**: như sidecup, bỏ `/ws/`, giữ `/internal/ → 404`, `/api/ → api:8080`, asset cache 1 năm, `index.html no-cache`. Kiểm `docker build -t lms-web apps/web` và `docker build -t lms-api apps/api` thành công; `docker run --rm lms-api --help` in các subcommand (`serve migrate seed worker healthcheck`).
7. **Profile full**: `docker compose -f infra/docker-compose.yml --profile full up -d --build --wait` → `curl -s localhost:8081/api/healthz` 200 (qua nginx) và `curl -s localhost:8081/` là HTML SPA. `down` sau khi kiểm.
8. **CI**: `ci.yml` (jobs `api`, `web`, `secrets`) và `e2e.yml` như sidecup với tên `lms`, `golangci/golangci-lint-action@v8 version: v2.14.0`, Node 22, pnpm action. `api` job dùng service postgres `lms_test` và `TEST_DATABASE_URL`. Kiểm bằng `act` nếu có, hoặc đẩy branch và xem workflow xanh (phase này test integration chỉ là `db_integration_test.go` ping + migrate up/down).
9. **README.md + docs**: README có mục Chạy nhanh (`cp apps/api/.env.example apps/api/.env`, sinh `OUTBOX_SECRET_KEY` bằng `openssl rand -base64 32`, `make dev`), bảng URL dev, cấu trúc thư mục, lệnh `lms` CLI (`seed --reset` chỉ `APP_ENV=dev|e2e`), liên kết `docs/README.md`, `spec-lms-mvp.md`, `plans/`. Tạo `docs/README.md` (điều hướng tới `architecture.md`, `database.md`, `api.md`, `runbook.md`, `design/`, `prototype/`) và `docs/architecture.md` khung: layout monorepo, service compose (api, worker, web, postgres, minio/R2, mailpit, caddy), luồng request qua Caddy/nginx, biến môi trường theo nhóm. Không tạo `docs/backend-architecture.md`, `docs/security.md`, `docs/pilot-checklist.md`, `docs/decisions/*`, `CHANGELOG.md`. <!-- Updated: Validation Session 1 - bộ docs tối giản theo sidecup -->

## Verification

```bash
make check-ports                      # exit 0 khi cổng trống
make db-up && docker compose -f infra/docker-compose.yml ps   # postgres, minio, mailpit healthy; minio-init Exited (0)
cd apps/api && go build ./... && go vet ./... && golangci-lint run ./... && go test -race -count=1 ./...
cd apps/api && TEST_DATABASE_URL=postgres://lms:lms@localhost:5432/lms_test?sslmode=disable go test -race -count=1 -tags integration ./internal/platform/db/...
make migrate-up && cd apps/api && go run ./cmd/lms migrate version        # in version 0 (file 0000)
cd apps/api && go run ./cmd/lms migrate down && go run ./cmd/lms migrate version   # down về rỗng in `no migration`, exit 0  <!-- Red Team: RT-04 - ErrNilVersion không phải lỗi -->
cd apps/api && (unset APP_ENV; go run ./cmd/lms serve; echo exit=$?)             # thiếu APP_ENV → thoát 1, log lỗi cấu hình  <!-- Red Team: RT-11 - APP_ENV bắt buộc -->
cd apps/api && go test -run TestEnvExample ./internal/platform/config/           # .env.example khớp Config  <!-- Red Team: RT-11 -->
make dev-api & sleep 3; curl -sf localhost:8080/healthz; curl -sf localhost:8080/readyz   # cả hai 200
cd apps/web && pnpm lint && pnpm typecheck && pnpm test && pnpm build
make build                            # hai image build xong
docker compose -f infra/docker-compose.yml --profile full up -d --build --wait && curl -sf localhost:8081/api/healthz && docker compose -f infra/docker-compose.yml --profile full ps --status running --services | grep -cE '^(api|worker)$' | grep -q 2 && docker compose -f infra/docker-compose.yml --profile full down   # api + worker đều chạy <!-- Red Team: RT-04 -->
docker compose -f infra/docker-compose.prod.yml --env-file infra/.env.example config --quiet   # cấu hình prod hợp lệ (dùng giá trị mẫu)
make homelab-config
git check-ignore -q infra/.env apps/api/.env && echo ignored
```

Kết quả mong đợi: mọi lệnh exit 0; không có file `.env` nào trong `git status`.

## Security notes

- Secret chỉ qua env; `.env.example` không chứa giá trị thật; `gitleaks` chạy trong CI.
- Distroless `nonroot`, không shell; healthcheck bằng subcommand Go.
- Caddy thêm HSTS, nosniff, DENY frame, CSP, xóa `Server`. Postgres và MinIO prod không publish cổng.
- `APP_ENV` bắt buộc, allowlist `dev|e2e|production`. `APP_ENV=production` từ chối khởi động khi `PUBLIC_BASE_URL` không phải https, `COOKIE_SECURE=false`, `S3_USE_SSL=false` hoặc `SMTP_USER` rỗng. <!-- Red Team: RT-11 - bỏ SMTP_TLS, thêm COOKIE_SECURE/S3_USE_SSL -->
- `TRUSTED_PROXIES` mặc định rỗng: không tin `X-Forwarded-For` nếu vận hành chưa khai báo CIDR của Caddy/Traefik; `OUTBOX_SECRET_KEY` bắt buộc 32 byte, không log. <!-- Red Team: RT-11 - mặc định an toàn -->
- Giá trị `lms/lms`, `lmsminio/lmsminio123` chỉ trong compose dev/e2e, có chú thích "không dùng ở production".

## Risks & Rollback

| Rủi ro | Biện pháp |
|---|---|
| `shadcn init -b radix` với Vite 8 + TS 6 chưa được scaffold thử (ghi chú báo cáo frontend) | Nếu CLI lỗi, tạo `components.json` tay và `shadcn add button` để kiểm; ghi vào README |
| MinIO image ghim tag có thể bị xóa khỏi Docker Hub | Mirror tag sang GHCR của tổ chức hoặc đổi sang `quay.io/minio/minio` cùng tag; `Storage` interface không đổi |
| Cổng 5432/9000 trùng với dịch vụ khác trên máy dev | `check-ports` dừng và chỉ tiến trình, không tự đổi cổng |
| Service `worker` và `api` chạy chung image; lỡ chạy migrate hai lần | `worker` đặt `MIGRATE_ON_START=false`, `depends_on: api: service_healthy`; golang-migrate khóa advisory nên chạy trùng cũng không hỏng <!-- Red Team: RT-04 - worker trong compose --> |
| Rollback | Phase chỉ thêm file, chưa có dữ liệu; `git revert` toàn bộ commit của phase. Từ Phase 02 trở đi rollback prod = hạ tag image với `MIGRATE_ON_START=false` sau `pg_dump -Fc` (xem `docs/runbook.md`) <!-- Red Team: RT-04 - quy ước rollback --> |

## Success Criteria

- [x] `make dev` chạy API (8080) + web (5173) + Postgres + MinIO + Mailpit; Ctrl-C dừng sạch, `lsof` không còn tiến trình của phase.
- [x] `lms --help` liệt kê `serve migrate seed worker healthcheck`; `lms migrate up|down|version` chạy trên file SQL nhúng qua iofs; không có `migrate force`, `routes`. <!-- Updated: Validation Session 1 - cắt lệnh ngoài spec -->
- [x] Thiếu `APP_ENV` hoặc `OUTBOX_SECRET_KEY` sai độ dài → tiến trình thoát mã 1 trước khi mở cổng; `TestEnvExample` xanh. <!-- Red Team: RT-11 -->
- [x] `GET /healthz` 200, `GET /readyz` 200 khi DB sống và 503 khi DB tắt.
- [ ] `make lint`, `make test`, `make build` exit 0; CI `ci.yml` xanh trên branch.
- [x] Profile `full` phục vụ SPA ở 8081 và proxy `/api/*` sang container api; `docker compose ps` thấy cả `api` và `worker` chạy; `docker-compose.prod.yml` cũng có `worker`. <!-- Red Team: RT-04 - worker service -->
- [x] `docs/README.md` và `docs/architecture.md` tồn tại, README gốc liên kết tới chúng. <!-- Updated: Validation Session 1 -->
- [x] `docker-compose.prod.yml` + Caddyfile `config --quiet` hợp lệ; header bảo mật có mặt trong Caddyfile.
- [x] `src/styles/goup-tokens.css` giống hệt dòng 4–51 của `prototype/goup.css` (`diff` rỗng sau khi bỏ comment đầu file).
- [x] Không có `.env` trong git; `gitleaks` không báo.
