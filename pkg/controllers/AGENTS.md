# Controller guidance

`pkg/controllers` contains Kubernetes reconciliation logic used by Kyverno's
controller binaries. Controllers observe Kubernetes resources and drive the
cluster toward the desired state; unlike admission webhooks, this work is
asynchronous and must tolerate retries, duplicate events, and leader changes.

## Entry points

- Most controller packages expose a `NewController` constructor. The binaries
  under `cmd/` assemble and start them.
- `webhook/` configures Kyverno webhook resources; `policycache/` maintains the
  policy cache; `report/` creates and aggregates reports.
- `cleanup/`, `deleting/`, and `ttl/` handle deletion and cleanup workflows.
- `admissionpolicygenerator/`, `exceptions/`, `globalcontext/`, and
  `policystatus/` implement specialized reconciliation responsibilities.
- `generic/` holds shared controller infrastructure. Prefer using it over
  duplicating queue, logging, or reconciliation plumbing.

See `docs/dev/controllers/README.md` for the controller-to-binary architecture
and leader-election responsibilities.

## Testing

- Add or update focused tests in the changed controller package and run
  `go test ./pkg/controllers/...` while iterating.
- Run `make test-unit` when shared infrastructure or multiple controllers are
  affected.
- Add a Chainsaw conformance scenario for changes observable in a running
  cluster, especially generate, cleanup, reporting, webhook-configuration, or
  policy-status behavior.

## Change safety

- Reconciliation must be idempotent and safe to retry. Do not assume a single
  event delivery or a stable resource version.
- Preserve ownership, leader-election, and finalizer behavior when working on
  resources managed by more than one controller.
- Check interactions with the admission controller and background controller;
  an update request or report often crosses component boundaries.

## Automation boundary

Automation may summarize the controllers touched and select candidate tests,
but must not auto-merge changes to reconciliation, deletion, cleanup, or
webhook-configuration behavior. Escalate when a change crosses controller
boundaries or changes resource lifecycle semantics.
