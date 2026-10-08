# Nạp apps/api/.env (nếu có) cho các lệnh cần DATABASE_URL, OUTBOX_SECRET_KEY...
WITH_ENV = set -a; [ -f .env ] && . ./.env; set +a;
# Bản cài sẵn phải được build bằng Go >= 1.27; nếu không, chạy đúng bản của CI:
# make lint GOLANGCI_LINT="go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0"
GOLANGCI_LINT ?= golangci-lint

.PHONY: run migrate-up migrate-down migrate-version seed seed-reset worker test test-unit test-integration lint fmt bin

run:
	@$(WITH_ENV) go run ./cmd/lms serve

migrate-up:
	@$(WITH_ENV) go run ./cmd/lms migrate up

migrate-down:
	@$(WITH_ENV) go run ./cmd/lms migrate down

migrate-version:
	@$(WITH_ENV) go run ./cmd/lms migrate version

seed:
	@$(WITH_ENV) go run ./cmd/lms seed

# Xóa dữ liệu rồi seed lại; lệnh tự từ chối khi APP_ENV không phải dev|e2e.
seed-reset:
	@$(WITH_ENV) go run ./cmd/lms seed --reset

worker:
	@$(WITH_ENV) go run ./cmd/lms worker

# Unit test luôn chạy; integration test chạy khi có TEST_DATABASE_URL (trong .env hoặc môi trường).
# Integration dùng -p 1: mọi package chung database lms_test nên chạy lần lượt.
test:
	@$(WITH_ENV) if [ -n "$$TEST_DATABASE_URL" ]; then \
		go test -race -count=1 -p 1 -tags integration ./...; \
	else \
		echo "TEST_DATABASE_URL chưa đặt: chỉ chạy unit test"; go test -race -count=1 ./...; \
	fi

test-unit:
	go test -race -count=1 ./...

test-integration:
	@$(WITH_ENV) test -n "$$TEST_DATABASE_URL" || { echo "Cần TEST_DATABASE_URL"; exit 1; }; \
	go test -race -count=1 -p 1 -tags integration ./...

lint:
	go vet ./...
	go vet -tags integration ./...
	$(GOLANGCI_LINT) run ./...

fmt:
	gofmt -w .

bin:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/lms ./cmd/lms
