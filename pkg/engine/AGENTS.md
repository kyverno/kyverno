# AGENTS.md — pkg/engine

This directory contains the core policy engine logic.

## Structure

```
pkg/engine/
├── adapters/         # Adapters for different policy types
├── anchor/           # Anchor handling for strategic merge patches
├── api/              # Engine API types and interfaces
├── apicall/          # External API call handlers
├── context/          # Policy evaluation context building
├── factories/        # Factory functions for engine components
├── handlers/         # Rule handlers (validate, mutate, generate, verify)
├── internal/         # Internal engine utilities
├── jmespath/         # JMESPath expression evaluation
├── jsonutils/        # JSON manipulation utilities
├── mutate/           # Mutation logic (patch, strategic merge, JSON patches)
├── operator/         # Operator pattern matching
├── pattern/          # Pattern matching utilities
├── policycontext/    # Policy context types
├── resources/        # Resource handling
├── utils/            # Shared engine utilities
├── validate/         # Validation logic
├── variables/        # Variable substitution and handling
├── wildcards/        # Wildcard matching
├── *.go              # Core engine entry points
└── *_test.go         # Unit tests
```

## Key Files

| File | Purpose |
|------|---------|
| `engine.go` | Main engine entry point, coordinates rule evaluation |
| `validation.go` / `validation_test.go` | Validate rule evaluation |
| `mutation.go` / `mutation_test.go` | Mutate rule evaluation |
| `generation.go` | Generate rule evaluation |
| `background.go` | Background scan processing |
| `image_verify.go` | Image verification coordination |

## Conventions

- **No CGO** - CGO_ENABLED=0
- **Generated code** - Never edit `zz_generated.*` files manually; run `make codegen-all`
- **Import aliases** - Enforced by `importas` linter (see `.golangci.yml`)
- **Logging** - Use `logr` with zerologr backend; levels L0-L4 (see `docs/dev/logging/logging.md`)
- **Feature flags** - Managed via `pkg/toggle` package

## Testing

```bash
# Unit tests for engine
go test -race -covermode=atomic ./pkg/engine/...

# Specific sub-package
go test -race ./pkg/engine/validate/...
go test -race ./pkg/engine/mutate/...
```

## For AI Agents

- **Entry points**: `Engine` struct in `engine.go`, `Validate()`, `Mutate()`, `Generate()` methods
- **Context building**: `pkg/engine/context/` - builds evaluation context from admission request
- **Rule matching**: `pkg/engine/validate/`, `pkg/engine/mutate/` - rule evaluation logic
- **Don't modify**: `pkg/client/`, `zz_generated.*` files
- **Codegen required**: After any API type changes in `api/`, run `make codegen-all-code && make verify-codegen`