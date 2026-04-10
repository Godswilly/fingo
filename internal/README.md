# Internal Package Boundaries

`internal/` contains all non-public FinGo code.

- `app`: use-case orchestration and application services
- `domain`: core business rules and invariants (no transport/DB dependencies)
- `errs`: shared internal error primitives
- `config`: environment and runtime configuration loading/validation
- `platform`: process wiring/bootstrap composition root
- `transport`: inbound adapters (`http`, `grpc`)
- `persistence`: outbound storage adapters (`postgres`, `migrations`)
- `messaging`: outbound event adapters (`nats`)
- `observability`: logging, metrics, tracing setup
