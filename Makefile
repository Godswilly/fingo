GOCACHE ?= /tmp/gocache
GOMODCACHE ?= /tmp/gomodcache
GO ?= go

.PHONY: test test-race lint build ci

test:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) test ./...

test-race:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) test -race ./...

lint:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found; skipped (go vet already ran)"; \
	fi

build:
	mkdir -p bin
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) build -o bin/fingo-api ./cmd/fingo-api
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) build -o bin/fingo-worker ./cmd/fingo-worker
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) build -o bin/fingo-migrate ./cmd/fingo-migrate

ci: lint test test-race build
