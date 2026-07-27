# FinGo: Distributed Ledger Engine

## Project Structure

FinGo is a backend-only Go service. Its layout follows the Builder backend
shape while keeping FinGo's hexagonal boundaries:

```text
cmd/                  # process entrypoints
internal/
  app/                # use-case orchestration
  config/             # configuration loading
  database/
    db/               # generated sqlc package
    postgres/         # Postgres adapters implementing app ports
  domain/             # business invariants
  errs/               # typed internal errors
migrations/           # executable migration history
sql/
  schema/             # current schema snapshot for sqlc
  queries/            # sqlc query files
scripts/              # local checks and maintenance scripts
```

## Quality Commands

Run all commands from repo root.

- `make test`: run unit/package tests
- `make test-race`: run tests with the Go race detector
- `make fmt-check`: fail if any Go file is not `gofmt`-formatted
- `make lint`: run `go vet` and `golangci-lint` (if installed)
- `make build`: compile all service entrypoints into `bin/`
- `make ci`: run lint + test + race + build in one command
- `make sqlc-generate`: regenerate `internal/database/db` from `sql/`
- `make sqlc-verify`: compile sqlc schema/query configuration

Notes:

- The Makefile sets `GOCACHE=/tmp/gocache` and `GOMODCACHE=/tmp/gomodcache` by default for sandbox-friendly builds.
- If `golangci-lint` is not installed, `make lint` still runs `go vet` and prints a skip message.
