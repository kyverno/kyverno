# AGENTS.md — pkg/controllers

This directory contains all Kubernetes controller implementations.

## Structure

```
pkg/controllers/
├── admissionpolicygenerator/  # Generates policies from admission data
├── certmanager/              # Certificate management for webhooks
├── cleanup/                  # CleanupPolicy/ClusterCleanupPolicy controller
├── deleting/                 # Deletion handling utilities
├── exceptions/               # PolicyException controller
├── generic/                  # Generic controller base utilities
├── globalcontext/            # Global context controller
├── metrics/                  # Controller metrics
├── policycache/              # Policy caching and indexing
├── policystatus/             # Policy status reporting
├── report/                   # PolicyReport/ClusterPolicyReport controller
├── ttl/                      # TTL controller for ephemeral resources
├── webhook/                  # Webhook configuration controller
├── controller.go             # Base controller interface
└── *_test.go                 # Controller tests
```

## Key Controllers

| Controller | Purpose | Entry Point |
|------------|---------|-------------|
| **Background** | Generate/mutate existing resources via UpdateRequests | `cmd/background-controller/` |
| **Reports** | Create PolicyReports from admission/background scans | `cmd/reports-controller/` |
| **Cleanup** | Execute CleanupPolicy via CronJobs | `cmd/cleanup-controller/` |
| **PolicyCache** | Cache and index policies for fast lookup | `pkg/controllers/policycache/` |
| **PolicyStatus** | Report policy status conditions | `pkg/controllers/policystatus/` |
| **Webhook** | Configure validating/mutating webhook configurations | `pkg/controllers/webhook/` |

## Conventions

- **Controller pattern**: Each controller implements `Runnable` interface (Start/Stop)
- **Reconciliation**: Use controller-runtime `Reconciler` with `Reconcile()` method
- **Leader election**: Controllers use leader election for HA (see `cmd/*/main.go`)
- **Metrics**: Expose Prometheus metrics via `pkg/metrics/`
- **Feature flags**: Check `pkg/toggle` for experimental features

## Testing

```bash
# All controller tests
go test -race ./pkg/controllers/...

# Specific controller
go test -race ./pkg/controllers/policycache/...
go test -race ./pkg/controllers/webhook/...
go test -race ./pkg/controllers/policystatus/...
```

## For AI Agents

- **Base patterns**: See `controller.go` for common interfaces
- **Reconcile loop**: Standard pattern - fetch, check conditions, act, update status
- **Caching**: Use `policycache` for policy lookups; never query API server directly in hot paths
- **Generated code**: Don't edit `pkg/client/`, `zz_generated.*`
- **Codegen**: After API changes, run `make codegen-all-code && make verify-codegen`
- **Leader election**: Managed by controller-runtime; see `cmd/*/main.go` for setup