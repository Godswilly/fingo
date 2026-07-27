# SQL Queries and Schema

This directory contains SQL files used by `sqlc`.

```text
sql/
├── schema/   # Current schema snapshot for sqlc
└── queries/  # sqlc query files
```

Migration files in `../migrations` are the executable history. Schema files in
`sql/schema` are the current schema snapshot used for code generation and must
stay aligned with migrations.

Common workflow:

1. Add or update a migration in `migrations/`.
2. Reflect the final schema shape in `sql/schema/`.
3. Add or update queries in `sql/queries/`.
4. Run `make sqlc-generate`.
5. Run `make ci`.

Generated code lives in `internal/database/db`. Hand-written Postgres adapters
that translate between generated query types and app-layer ports live in
`internal/database/postgres`.

`make ci` includes `scripts/check-schema-sync.sh`, a lightweight name-only
guard for tables, indexes, and constraints. It is not a semantic schema diff,
so reviewers still need to check column types and constraint expressions when
migrations change.
