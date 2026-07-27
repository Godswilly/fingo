# Database Layer

`internal/database` contains outbound database code.

```text
internal/database/
├── db/        # generated sqlc package
└── postgres/  # hand-written Postgres adapters for app-layer ports
```

Application code should depend on ports in `internal/app`, not on generated
sqlc types directly. The generated package is an implementation detail of the
database adapters.
