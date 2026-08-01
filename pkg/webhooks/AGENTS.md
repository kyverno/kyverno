# AGENTS.md — pkg/webhooks

This directory contains admission webhook handlers and related utilities.

## Structure

```
pkg/webhooks/
├── celexception/       # CEL exception handlers
├── exception/          # Policy exception handlers
├── globalcontext/      # Global context for webhooks
├── handlers/           # Main webhook handler entry points
├── policy/             # Policy-specific webhook logic
├── resource/           # Resource webhook handlers (vpol, ivpol, mpol, gpol, dpol)
│   ├── vpol/           # ValidatingPolicy handlers
│   ├── ivpol/          # ImageValidatingPolicy handlers
│   ├── mpol/           # MutatingPolicy handlers
│   ├── gpol/           # GeneratingPolicy handlers
│   └── dpol/           # DeletionPolicy handlers
├── updaterequest/      # UpdateRequest handlers (background mutations)
├── utils/              # Shared webhook utilities
├── server.go           # Webhook server setup
├── server_test.go      # Server tests
├── types.go            # Webhook types
└── log.go              # Logging setup
```

## Key Components

| Component | Purpose |
|-----------|---------|
| `handlers/` | Main admission handler - routes requests to policy-specific handlers |
| `resource/vpol/` | ValidatingPolicy admission + audit handlers |
| `resource/ivpol/` | ImageValidatingPolicy admission + audit handlers |
| `resource/mpol/` | MutatingPolicy handlers |
| `resource/gpol/` | GeneratingPolicy handlers |
| `resource/dpol/` | DeletionPolicy handlers |
| `updaterequest/` | Background controller UpdateRequest processing |

## Conventions

- **Admission handlers** return `*admissionv1.AdmissionResponse`
- **Audit handlers** create `PolicyReport` resources via reports controller
- **Error handling**: Use structured logging (L0-L4), never expose internal errors to clients
- **Feature flags**: Check `pkg/toggle` before enabling experimental handlers

## Testing

```bash
# All webhook tests
go test -race ./pkg/webhooks/...

# Specific policy type
go test -race ./pkg/webhooks/resource/vpol/...
go test -race ./pkg/webhooks/resource/ivpol/...

# Handler tests
go test -race ./pkg/webhooks/handlers/...
```

## For AI Agents

- **Entry point**: `handlers/AdmissionHandler` - main handler registration
- **Policy handlers**: Each policy type has `Admit()` and `Audit()` methods
- **Validation**: Use `pkg/validation/` for policy validation before admission
- **Generated code**: Don't edit `pkg/client/`, `zz_generated.*`
- **Codegen**: After API changes, run `make codegen-all-code && make verify-codegen`
- **Webhook config**: Managed in `pkg/webhooks/server.go` - registers handlers and sets up TLS