# AGENTS.md — pkg/controllers/webhook

This directory contains the webhook configuration controller that manages ValidatingWebhookConfiguration and MutatingWebhookConfiguration resources.

## Structure

```
pkg/controllers/webhook/
├── controller.go          # Main controller logic
├── reconciler.go          # Reconciliation logic
├── handlers.go            # Event handlers
├── builder.go             # Webhook configuration builders
├── certs.go               # Certificate management
├── metrics.go             # Prometheus metrics
└── *_test.go              # Unit tests
```

## Responsibilities

- **Creates/updates** webhook configurations for all policy types (ValidatingPolicy, MutatingPolicy, GeneratingPolicy, ImageValidatingPolicy, DeletingPolicy)
- **Manages** TLS certificates for webhook servers (rotation, renewal)
- **Handles** policy changes (add/remove/update) and updates webhook configs accordingly
- **Implements** failure policy (Ignore/Fail) based on policy configuration
- **Coordinates** with policy cache for efficient webhook registration

## Key Types

| Type | Purpose |
|------|---------|
| `WebhookController` | Main controller struct |
| `WebhookReconciler` | Reconciles webhook configurations |
| `WebhookBuilder` | Builds ValidatingWebhookConfiguration/MutatingWebhookConfiguration |
| `CertManager` | Manages webhook server certificates |

## Conventions

- **Reconcile pattern**: Fetch → Compute desired state → Apply diff → Update status
- **Leader election**: Only leader reconciles (see `cmd/kyverno/main.go`)
- **Certificate rotation**: Automatic via cert-manager or self-signed rotation
- **Metrics**: Exposed via `pkg/metrics/` (webhook_* prefix)

## Testing

```bash
# Unit tests
go test -race ./pkg/controllers/webhook/...

# Integration: deploy Kyverno in Kind, create policies, verify webhook configs
make kind-deploy-kyverno && kubectl get validatingwebhookconfigurations,mutatingwebhookconfigurations
```

## For AI Agents

- **Entry**: `controller.go` - `Start()` registers reconciler
- **Reconciliation**: `reconciler.go` - `Reconcile()` is main loop
- **Builders**: `builder.go` - constructs webhook configs from policies
- **Certificates**: `certs.go` - handles cert generation/rotation
- **Generated code**: Don't edit `pkg/client/`, `zz_generated.*`
- **Codegen**: After API changes, run `make codegen-all-code && make verify-codegen`