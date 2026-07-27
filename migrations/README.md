# Database Migrations

This directory contains FinGo database migrations.

Migrations are embedded into the `migrations` Go package and applied by
`cmd/fingo-migrate`.

Common commands:

```bash
make postgres-up
make migrate
make test-integration
make postgres-down
```

When a migration changes tables used by sqlc, update `sql/schema` to reflect
the resulting schema and run `make sqlc-generate`.
