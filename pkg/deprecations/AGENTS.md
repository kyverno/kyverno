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
#17708 set out to close.

## The two registries are add-only, on purpose

`RegisterLegacyExecutionEscapeHatch` and `AllowSubresourceForLegacyStatusWriter` have no unregister
API, and must not gain one. An unregister function is the lever that would let a caller widen the
gate at runtime.

The registries are empty by default, so this package on its own denies unconditionally. Each
allowance is registered *by the legacy code that justifies it*, so deleting that code deletes the
allowance in the same commit:

| Allowance | Registered from | Deleted by |
|---|---|---|
| Escape hatch (kyverno) | `pkg/webhooks/resource/legacy_hatch.go`, from `NewHandlers` over `policycache.Cache` | Removing the policy cache or the legacy branch |
| Escape hatch (cleanup) | `pkg/controllers/cleanup/legacy_hatch.go`, from `NewController` | Removing the legacy cleanup scheduler |
| `status` subresource | The three `legacy_status.go` files holding Kyverno's own legacy status writers | Deleting those writers |

**Do not move a registration to a file that is not itself legacy execution.** An earlier design
registered the hatch from the typed validation handlers; those are not execution, so a tree could
compile with execution removed and the hatch still honouring `FLAG_BLOCK_LEGACY_POLICY_APIS=false`.
That is the forbidden state.

When #17710 removes legacy execution, verify the registrations disappear with it rather than
deleting these functions outright.

## Webhook rules

`webhookrules.go` is the single source of truth for which kinds and versions the legacy denial
webhooks match. `webhookrules_test.go` audits it against the served versions in
`config/crds/kyverno/*.yaml`. A served version that no rule matches is a silent-acceptance hole, so
never narrow these values to tidy them up; `pkg/controllers/webhook/legacy_apiversions_test.go`
exists to stop exactly that.

Related: [`pkg/utils/admission`](../utils/admission/AGENTS.md), [`pkg/toggle`](../toggle/AGENTS.md),
[`pkg/webhooks`](../webhooks/AGENTS.md).
