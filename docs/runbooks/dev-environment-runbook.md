# FinGo Dev Environment Runbook (Week 1 Baseline)

This runbook covers local setup, core commands, and common failure fixes.

## Prerequisites

- Go 1.24+
- Git
- Make
- Docker (for later DB/integration workflows)
- Optional: `golangci-lint` for full local lint checks

## Project Setup

```bash
git clone <repo-url>
cd fingo
go mod download
```

## Core Commands

```bash
make fmt-check
make lint
make test
make test-race
make build
make ci
```

Outputs:

- binaries are produced in `bin/`
- tests run across all packages
- `make ci` runs full local quality gates

## Recommended Environment Variables (Current Baseline)

```bash
export FINGO_APP_ENV=local
export FINGO_LOG_LEVEL=info
export FINGO_DATABASE_URL='postgres://user:pass@localhost:5432/fingo?sslmode=disable'
export FINGO_NATS_URL='nats://localhost:4222'
export FINGO_API_ADDR=':8080'
```

## Common Failures and Fixes

### 1) `go: ... read-only file system` while running tests/builds

Cause:

- default Go cache paths are not writable in restricted environments

Fix:

```bash
GOCACHE=/tmp/gocache GOMODCACHE=/tmp/gomodcache go test ./...
```

or rely on `make` targets which already set cache variables.

### 2) CI lint failure due Go/linter mismatch

Cause:

- `golangci-lint` built with older Go than the module target version

Fix:

- align `go.mod` Go version with supported linter version
- keep `golangci-lint` pinned in CI workflow

### 3) `make lint` skips `golangci-lint` locally

Cause:

- `golangci-lint` is not installed on your machine

Fix:

- install `golangci-lint`, then re-run `make lint`
- `go vet` still runs even when linter is missing

### 4) Branch protection blocks merge despite green local checks

Cause:

- required GitHub checks did not run for latest commit/branch state

Fix:

- push latest commit
- verify required checks (`Quality Gates`, etc.) are green in PR
- ensure branch is up to date with target branch

### 5) Formatting check fails in CI

Cause:

- one or more files are not `gofmt`-formatted

Fix:

```bash
gofmt -w .
make fmt-check
```

## Week 1 Operating Routine

- Before push: `make ci`
- Before opening PR: ensure branch is rebased/updated
- After merge: pull latest `develop` before starting next task branch
