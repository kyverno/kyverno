# AGENTS.md — pkg/deprecations

This package owns the 1.20 write-time denial of legacy `kyverno.io` policy APIs. It decides
admission outcomes from raw request metadata only and must never import a typed policy API, because
it has to keep working after #17713 deletes `api/kyverno/v1` and the generated clients.

## The invariant

Never ship removed execution with silently accepted writes. A legacy policy must never store
successfully with nothing enforcing it and no error shown. Every rule below exists to protect that.

## What `DenyLegacyWrite` decides

It denies creates and every top-level update on a legacy kind, including unchanged-spec
reapplication, which is why GitOps reconcilers fail until the object is deleted. DELETE and CONNECT
always pass. There are exactly two exceptions, both narrow:

- Finalizer removal on an already-terminating object, via
  `admissionutils.IsFinalizerRemovalOnTerminatingObject`. See that package's `AGENTS.md`.
- Subresources registered through `AllowSubresourceForLegacyStatusWriter`, currently only `status`.

Do not widen either. The blanket "any subresource passes" bypass this replaced is the exact hole
issue #17708 set out to close.

## The two registries are add-only, on purpose

`RegisterLegacyExecutionEscapeHatch` and `AllowSubresourceForLegacyStatusWriter` have no unregister
API, and must not gain one. An unregister function is the lever that would let a caller widen the
gate at runtime.

`ClearExecutionEscapeHatch` is not that lever and must not grow into it. It drops the hatch, which
makes the gate strictly more restrictive, and it exists because the registry is process-global:
`go test -count=N` reuses one process, so a test asserting what registering the hatch changed needs
a known starting point. It deliberately leaves the subresource allowances alone, since those are
registered from `init()` and cannot be re-triggered within a process. Never add a selective remove.

The registries are empty by default, so this package on its own denies unconditionally. Each
allowance is registered *by the legacy code that justifies it*, so deleting that code deletes the
allowance in the same commit:

| Allowance | Registered from | Deleted by |
|---|---|---|
| `status` subresource | The three `legacy_status.go` files holding Kyverno's own legacy status writers | Deleting those writers |

#17710 deleted both escape-hatch registration sites (`pkg/webhooks/resource/legacy_hatch.go`,
registered from `NewHandlers` over the now-deleted `policycache.Cache`, and
`pkg/controllers/cleanup/legacy_hatch.go`, registered from `NewController` in the now-deleted
`pkg/controllers/cleanup`) along with the legacy execution they guarded. `RegisterLegacyExecutionEscapeHatch`
itself stays in `block.go` as a documented, currently-unused hook — nothing in the tree calls it, so
the write-time denial is unconditional. Do not reintroduce a call to it outside genuinely restored
legacy execution code.

## Keep this package's own source type-free

`deprecations.go` must not import a Kyverno policy type. The field-level warning helpers live in
the `policywarnings` subpackage, so that extraction removes their direct type dependency from
`deprecations.go`.

One transitive path remains: `block.go` imports `pkg/utils/admission` for the finalizer check, and
that package still carries typed helpers such as `policy.go`. Those are consumed only by the legacy
handlers, so #17710 deletes them with their callers before #17713 deletes the types. If that ordering
ever changes, split the type-free helpers out rather than reintroducing typed code here.

## Webhook rules

`webhookrules.go` is the single source of truth for which kinds and versions the legacy denial
webhooks match. `webhookrules_test.go` audits it against the served versions in
`config/crds/kyverno/*.yaml`. A served version that no rule matches is a silent-acceptance hole, so
never narrow these values to tidy them up; `pkg/controllers/webhook/legacy_apiversions_test.go`
exists to stop exactly that.

Related: [`pkg/utils/admission`](../utils/admission/AGENTS.md), [`pkg/toggle`](../toggle/AGENTS.md),
[`pkg/webhooks`](../webhooks/AGENTS.md).
