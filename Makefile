GOCACHE ?= /tmp/gocache
GOMODCACHE ?= /tmp/gomodcache
GOLANGCI_LINT_CACHE ?= /tmp/golangci-lint
GO ?= go

.PHONY: fmt-check test test-race lint build quality-gates ci install-hooks uninstall-hooks check-secrets boundary-check

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

quality-gates: check-secrets boundary-check fmt-check lint test test-race build

ci: quality-gates

check-secrets:
	CHECK_SECRETS_SCOPE=all scripts/check-secrets.sh

boundary-check:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) scripts/check-boundaries.sh

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
