# AGENTS.md — pkg/webhooks

The actual `AdmissionReview` HTTP handlers. No package doc comments exist here — this file is the closest thing to
one. See [ARCHITECTURE.md](../../ARCHITECTURE.md) for how this fits between `pkg/engine`/`pkg/cel` and
`pkg/background`.

## What actually makes a rule synchronous vs. queued as an UpdateRequest

It's **rule type**, not enforce/audit mode:

- `generate` and `mutateExisting` rules are **always** routed to `handleBackgroundApplies` →
  `pkg/webhooks/updaterequest.go`, regardless of enforce/audit. See [pkg/background/AGENTS.md](../background/AGENTS.md)
  for what happens after the `UpdateRequest` is created.
- Plain `validate` rules are evaluated inline. Enforce-mode and audit-mode-with-`emitWarning` (`ValidateAuditWarn`)
  run synchronously inside the same `wait.Group` that produces the admission decision — a resource can be
  `Allowed: true` and still carry a `Warning` from a mixed policy (see `handlers_test.go`'s
  `Test_ValidateAuditWarn_MixedAuditEnforcePolicy`). Plain audit-mode validate (no warning) runs on a **separate,
  bounded worker pool** (`auditPool`, a `pond.Pool`) purely to produce Events/PolicyReport data — this never affects
  the already-returned admission response.
- Background applies for generate/mutateExisting are fired in parallel with the enforce-validation wait group but
  **not awaited** — the admission response returns as soon as enforce validation finishes, without waiting for
  background work.
- Mutate rules (non-`mutateExisting`) apply inline; if image-verify rules also match, the request is re-patched and
  the `PolicyContext` rebuilt *before* image verification runs — image-verify always sees the post-mutation
  resource, never the original.

## `updaterequest.go` only ever creates a marker CR — it never performs the mutation itself

`handleBackgroundApplies` deliberately uses `context.Background()` + a 30s timeout instead of the inbound HTTP
request context, because the request context is cancelled the moment the webhook handler returns but background
work must outlive it. `Apply` (in `pkg/webhooks/updaterequest/generator.go`) silently no-ops for a `Generate`-type
request with empty `RuleContext`, and otherwise spawns a detached goroutine (`go g.applyResource(context.TODO(), ur)`)
that retries creating the `UpdateRequest` object with exponential backoff, then flips its status to `Pending`. The
real generate/mutate-existing side effects happen later, in `pkg/background`, which watches these objects — this
package's job ends at creating the CR.

## Two separate PolicyException admission handlers exist, on purpose

`pkg/webhooks/exception` (legacy `kyverno.io` `PolicyException`) and `pkg/webhooks/celexception` (CEL
`policies.kyverno.io` `PolicyException`) are wired to two different webhook service paths because they're different
CRDs. Only the CEL handler compiles `matchConditions` as CEL expressions — and it grandfathers already-stored
expressions under a looser validation environment than brand-new ones (see
[pkg/cel/AGENTS.md](../cel/AGENTS.md)'s PolicyException section for the mechanism).

## Recent bug pattern: namespace-scoping/routing for CEL policy variants

Several recent fixes are specifically about namespaced CEL policy kinds being served at the wrong admission path or
evaluated with the wrong scope (`f68ce3153`, `57f69eb73`, `b3629e853`). If you're adding or touching a
`Namespaced*Policy` admission path, check these first — this is a recurring class of bug here, not a one-off.
