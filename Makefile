COMPOSE = docker compose -f infra/docker-compose.yml
# File env của prod/homelab; kiểm cấu hình bằng giá trị mẫu: make homelab-config ENV_FILE=infra/.env.example
ENV_FILE ?= infra/.env
COMPOSE_PROD = docker compose -f infra/docker-compose.prod.yml --env-file $(ENV_FILE)
COMPOSE_HOMELAB = docker compose -f infra/docker-compose.prod.yml -f infra/docker-compose.homelab.yml --env-file $(ENV_FILE)
# pnpm cài toàn cục hoặc qua corepack (Node >= 22 có sẵn corepack).
PNPM ?= $(shell command -v pnpm >/dev/null 2>&1 && echo pnpm || echo corepack pnpm)

.PHONY: help check-ports db-up dev dev-api dev-web dev-down down migrate-up migrate-down migrate-version seed seed-reset worker test test-api test-web lint check build e2e e2e-up e2e-seed-reset e2e-down e2e-ui render-check prod-up prod-down homelab-config homelab-up homelab-down

help:
	@echo "make dev              Postgres + MinIO + Mailpit (docker) + migrate + API (go run) + web (vite); Ctrl-C dừng API/web"
	@echo "make check-ports      kiểm cổng 8080 5173 5432 9000 9001 1025 8025 (POSTGRES_HOST_PORT đổi cổng Postgres)"
	@echo "make db-up            chỉ dựng Postgres + MinIO (tạo bucket) + Mailpit"
	@echo "make dev-api | dev-web   chạy riêng API (không hot reload) hoặc web"
	@echo "make dev-down (down)  dừng stack docker dev"
	@echo "make migrate-up | migrate-down | migrate-version"
	@echo "make seed | seed-reset (chỉ APP_ENV=dev|e2e) | worker"
	@echo "make test | test-api | test-web | lint | check | build"
	@echo "make e2e | e2e-ui | render-check   (profile full trên :8081; e2e-up | e2e-seed-reset | e2e-down)"
	@echo "make prod-up | prod-down   (infra/docker-compose.prod.yml + ENV_FILE=$(ENV_FILE))"
	@echo "make homelab-config | homelab-up | homelab-down   (prod + overlay Traefik)"

# Không tự đổi cổng khi bị chiếm: báo tiến trình/container đang giữ cổng để dừng đúng nó.
# Cổng của Postgres/MinIO/Mailpit được bỏ qua khi chính service đó của stack lms đang publish đúng cổng ấy.
# Dùng `ss` để thấy cả cổng do docker-proxy (root) giữ; `lsof` không thấy socket của user khác.
check-ports:
	@pg_port=$${POSTGRES_HOST_PORT:-5432}; status=0; \
	for spec in 8080:: 5173:: $$pg_port:postgres:5432 9000:minio:9000 9001:minio:9001 1025:mailpit:1025 8025:mailpit:8025; do \
		p=$${spec%%:*}; rest=$${spec#*:}; svc=$${rest%%:*}; cport=$${rest#*:}; \
		if command -v ss >/dev/null 2>&1; then \
			busy=$$(ss -Hltn "sport = :$$p" 2>/dev/null); \
		else \
			busy=$$(lsof -nP -iTCP:$$p -sTCP:LISTEN 2>/dev/null); \
		fi; \
		[ -z "$$busy" ] && continue; \
		if [ -n "$$svc" ] && $(COMPOSE) port "$$svc" "$$cport" 2>/dev/null | grep -qx ".*:$$p"; then continue; fi; \
		echo "Cổng $$p đang bị chiếm:"; \
		lsof -nP -iTCP:$$p -sTCP:LISTEN 2>/dev/null || echo "$$busy"; \
		docker ps --filter "publish=$$p" --format '  container {{.Names}} ({{.Ports}})' 2>/dev/null; \
		status=1; \
	done; \
	if [ $$status -ne 0 ]; then echo "Dừng tiến trình trên (hoặc đặt POSTGRES_HOST_PORT cho Postgres) rồi chạy lại."; fi; \
	exit $$status

# `up --wait` báo lỗi khi một service chạy-một-lần thoát (kể cả mã 0), nên minio-init chạy riêng
# và trả đúng mã thoát của nó.
db-up:
	$(COMPOSE) up -d --wait postgres minio mailpit
	$(COMPOSE) up --no-log-prefix --exit-code-from minio-init minio-init

dev: check-ports db-up migrate-up
	@trap 'kill 0' INT TERM EXIT; \
	 ( cd apps/api && $(MAKE) --no-print-directory run ) & \
	 ( cd apps/web && $(PNPM) dev ) & \
	 wait

dev-api:
	cd apps/api && $(MAKE) --no-print-directory run

dev-web:
	cd apps/web && $(PNPM) dev

dev-down:
	$(COMPOSE) down

down: dev-down

migrate-up:
	cd apps/api && $(MAKE) --no-print-directory migrate-up

migrate-down:
	cd apps/api && $(MAKE) --no-print-directory migrate-down

migrate-version:
	cd apps/api && $(MAKE) --no-print-directory migrate-version

seed:
	cd apps/api && $(MAKE) --no-print-directory seed

seed-reset:
	cd apps/api && $(MAKE) --no-print-directory seed-reset

worker:
	cd apps/api && $(MAKE) --no-print-directory worker

test: test-api test-web

test-api:
	cd apps/api && $(MAKE) --no-print-directory test

test-web:
	cd apps/web && $(PNPM) test

lint:
	cd apps/api && $(MAKE) --no-print-directory lint
	cd apps/web && $(PNPM) lint && $(PNPM) typecheck

check: lint test

build:
	docker build -t lms-api apps/api
	docker build -t lms-web apps/web

# E2E trên stack container (postgres + minio + mailpit + api + worker + web, profile "full"); cần Docker và Node.
# Postgres của máy dev đã đổi cổng thì giữ nguyên POSTGRES_HOST_PORT khi gọi, nếu không compose sẽ dựng lại ở 5432.
e2e-up:
	$(COMPOSE) --profile full up -d --build --wait

# Seed lại bằng chính container api của profile "full": cùng APP_ENV=e2e và SEED_PASSWORD với stack đang chạy.
# Tải video mẫu cho mọi khóa video của seed để trình phát chạy được. globalSetup của Playwright gọi target
# này đúng một lần; spec @serial gọi lại ở beforeAll.
e2e-seed-reset:
	$(COMPOSE) --profile full exec -T api /lms seed --reset --upload-sample

# Dừng và xóa riêng container của profile "full"; postgres/minio/mailpit dùng chung với `make dev` được giữ lại.
e2e-down:
	$(COMPOSE) --profile full rm -sf api worker web

# Chạy chromium e2e + a11y rồi dừng container của profile "full", trả đúng mã thoát của Playwright.
e2e: e2e-up
	cd apps/web && $(PNPM) test:e2e; status=$$?; cd ../.. && $(MAKE) --no-print-directory e2e-down; exit $$status

# Playwright UI mode trên stack đang chạy (không tự dừng stack).
e2e-ui: e2e-up
	cd apps/web && $(PNPM) exec playwright test -c e2e/playwright.config.ts --ui

# So computed style và ảnh vùng của web (:8081) với prototype tĩnh (:8090, Playwright tự dựng bằng python3).
render-check: e2e-up
	cd apps/web && $(PNPM) render-check

prod-up:
	$(COMPOSE_PROD) up -d --build

prod-down:
	$(COMPOSE_PROD) down

homelab-config:
	$(COMPOSE_HOMELAB) config --quiet

homelab-up:
	$(COMPOSE_HOMELAB) up -d --build

homelab-down:
	$(COMPOSE_HOMELAB) down
