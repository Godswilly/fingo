# FinGo: Distributed Ledger Engine

## Quality Commands

Run all commands from repo root.

- `make test`: run unit/package tests
- `make test-race`: run tests with the Go race detector
- `make fmt-check`: fail if any Go file is not `gofmt`-formatted
- `make lint`: run `go vet` and `golangci-lint` (if installed)
- `make build`: compile all service entrypoints into `bin/`
- `make ci`: run lint + test + race + build in one command

Notes:

- The Makefile sets `GOCACHE=/tmp/gocache` and `GOMODCACHE=/tmp/gomodcache` by default for sandbox-friendly builds.
- If `golangci-lint` is not installed, `make lint` still runs `go vet` and prints a skip message.
