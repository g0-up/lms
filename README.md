# GoUp LMS

Hệ thống quản lý đào tạo học viên theo [spec-lms-mvp.md](spec-lms-mvp.md). Monorepo gồm `apps/api` (Go: cobra, Gin, sqlx, golang-migrate), `apps/web` (Vite, React, Tailwind v4, shadcn Radix) và `infra/` (Docker Compose, Caddy hoặc Traefik ở homelab).

## Chạy nhanh

Cần Docker (Compose v2), Go 1.27, Node >= 22.22 (pnpm qua corepack: `corepack pnpm ...`, không cần cài pnpm toàn cục).

```sh
cp apps/api/.env.example apps/api/.env
# Điền OUTBOX_SECRET_KEY (bắt buộc, 32 byte base64) và SEED_PASSWORD trong apps/api/.env:
openssl rand -base64 32
(cd apps/web && corepack pnpm install)
make dev      # kiểm cổng, dựng Postgres + MinIO + Mailpit, migrate, chạy API và web; Ctrl-C dừng API/web
make down     # dừng stack docker dev (dữ liệu giữ trong volume)
```

Cổng 5432 đã bị chiếm: đặt `POSTGRES_HOST_PORT=5433` (biến chỉ dùng cho compose) và đổi `localhost:5432` thành `localhost:5433` trong `DATABASE_URL`, `TEST_DATABASE_URL` của `apps/api/.env`.

| Dịch vụ | URL dev |
|---|---|
| Web (Vite, proxy `/api` sang API) | http://localhost:5173 |
| API | http://localhost:8080 (`/healthz`, `/readyz`, `/api/v1/...`) |
| Mailpit (hộp thư dev) | http://localhost:8025 |
| MinIO console (`lmsminio` / `lmsminio123`, chỉ dev) | http://localhost:9001 |
| Stack container `--profile full` (E2E) | http://localhost:8081 |

## Lệnh thường dùng

```sh
make help                 # liệt kê target
make test                 # Go (thêm integration khi có TEST_DATABASE_URL) + Vitest
make lint                 # go vet + golangci-lint, ESLint + tsc
make build                # image lms-api và lms-web
make e2e                  # stack container + seed --reset + Playwright
make prod-up              # infra/docker-compose.prod.yml với infra/.env (mẫu: infra/.env.example)
make homelab-config ENV_FILE=infra/.env.example   # kiểm cấu hình prod + overlay Traefik
```

CLI của API (`go run ./cmd/lms <lệnh>` trong `apps/api`, hoặc `/lms <lệnh>` trong image):

| Lệnh | Ý nghĩa |
|---|---|
| `serve` | Chạy HTTP API; migrate trước nếu `MIGRATE_ON_START=true` |
| `migrate up` / `migrate down [--steps N]` / `migrate version` | Migration SQL nhúng trong binary; chưa có migration nào thì in `no migration` |
| `seed [--reset] [--upload-sample]` | Dữ liệu mẫu; chỉ `APP_ENV=dev` hoặc `e2e`, `--reset` xóa dữ liệu cũ |
| `worker` | Worker nền (outbox email) chạy tới khi nhận SIGTERM |
| `healthcheck` | Gọi `/healthz` của tiến trình `serve` cục bộ, dùng cho healthcheck container |

## Cấu trúc

```text
apps/api/        Go API: cmd/lms (cobra), internal/app (router), internal/platform (config, db), migrations (SQL nhúng)
apps/web/        SPA React: src/app (route), src/shared (lib, ui), src/styles (token GoUp), e2e (Playwright)
infra/           compose dev/prod/homelab, Caddyfile, postgres/init, .env.example (danh sách biến production)
docs/            tài liệu dự án, bắt đầu từ docs/README.md
plans/           kế hoạch triển khai theo phase
prototype/       prototype tĩnh, nguồn chuẩn cho giao diện
```

`apps/web/components.json` được viết tay (style `new-york`, alias `@/shared/...`) vì shadcn CLI 4 `init -b radix` chỉ còn preset nova/vega/...; thêm component bằng `corepack pnpm dlx shadcn@latest add <tên>` (CLI gọi `pnpm`, nên cần `pnpm` trên PATH, ví dụ `corepack enable pnpm`).

## Tài liệu

- [docs/README.md](docs/README.md): điều hướng tài liệu
- [docs/architecture.md](docs/architecture.md): kiến trúc và hạ tầng
- [spec-lms-mvp.md](spec-lms-mvp.md): đặc tả MVP
- [plans/](plans/): kế hoạch triển khai
