GOCACHE ?= /tmp/gocache
GOMODCACHE ?= /tmp/gomodcache
GOLANGCI_LINT_CACHE ?= /tmp/golangci-lint
GO ?= go
SQLC_VERSION ?= v1.27.0
FINGO_DATABASE_URL ?= postgres://fingo:fingo@localhost:5432/fingo?sslmode=disable

.PHONY: fmt-check test test-race lint build quality-gates ci install-hooks uninstall-hooks check-secrets boundary-check schema-sync-check postgres-up postgres-down migrate test-integration sqlc-generate sqlc-verify

fmt-check:
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "The following files are not formatted:"; \
		gofmt -l .; \
		exit 1; \
	fi

test:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) test ./...

test-race:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) test -race ./...

lint:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) golangci-lint run --timeout=5m ./...; \
	else \
		echo "golangci-lint not found; skipped (go vet already ran)"; \
	fi

build:
	mkdir -p bin
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) build -o bin/fingo-api ./cmd/fingo-api
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) build -o bin/fingo-worker ./cmd/fingo-worker
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) build -o bin/fingo-migrate ./cmd/fingo-migrate

quality-gates: check-secrets boundary-check schema-sync-check sqlc-verify fmt-check lint test test-race build

ci: quality-gates

check-secrets:
	CHECK_SECRETS_SCOPE=all scripts/check-secrets.sh

boundary-check:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) scripts/check-boundaries.sh

schema-sync-check:
	scripts/check-schema-sync.sh

postgres-up:
	docker compose up -d postgres
	@echo "waiting for postgres..."
	@until docker compose exec -T postgres pg_isready -U fingo -d fingo >/dev/null 2>&1; do sleep 1; done

postgres-down:
	docker compose down

migrate:
	FINGO_APP_ENV=$${FINGO_APP_ENV:-local} FINGO_LOG_LEVEL=$${FINGO_LOG_LEVEL:-info} FINGO_DATABASE_URL=$(FINGO_DATABASE_URL) \
		GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) run ./cmd/fingo-migrate

test-integration: postgres-up migrate
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) FINGO_DATABASE_URL=$(FINGO_DATABASE_URL) \
		$(GO) test -tags=integration ./internal/database/postgres/...

sqlc-generate:
	@if command -v sqlc >/dev/null 2>&1; then \
		sqlc generate; \
	else \
		GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate; \
	fi

sqlc-verify:
	@if command -v sqlc >/dev/null 2>&1; then \
		sqlc compile; \
	else \
		GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) compile; \
	fi

install-hooks:
	@if ! command -v lefthook >/dev/null 2>&1; then \
		echo "lefthook is not installed."; \
		echo "Install it from https://github.com/evilmartians/lefthook, then run make install-hooks again."; \
		exit 1; \
	fi
	lefthook install

uninstall-hooks:
	@if command -v lefthook >/dev/null 2>&1; then \
		lefthook uninstall; \
	else \
		echo "lefthook is not installed; nothing to uninstall."; \
	fi
