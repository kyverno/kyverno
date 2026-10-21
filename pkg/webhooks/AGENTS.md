# AGENTS.md — pkg/webhooks

The actual `AdmissionReview` HTTP handlers. No package doc comments exist here — this file is the closest thing to
one. See [ARCHITECTURE.md](../../ARCHITECTURE.md) for how this fits between `pkg/engine`/`pkg/cel` and
`pkg/background`.

## Routing is per CEL policy kind, each with its own webhook handler

Since 1.20 removed legacy `kyverno.io` execution (#17710), there is no single shared admission
dispatcher for resource requests. Each CEL policy kind is served from its own route in `server.go`
by its own handler package under `pkg/webhooks/resource/`:

- `pkg/webhooks/resource/vpol` — `ValidatingPolicy`/`NamespacedValidatingPolicy`, served at
  `/vpol/*policies` / `/nvpol/*policies`, evaluated synchronously inside a `wait.Group`.
- `pkg/webhooks/resource/ivpol` — `ImageValidatingPolicy`/`NamespacedImageValidatingPolicy`,
  served at `/ivpol/{validate,mutate}/*policies` / `/nivpol/{validate,mutate}/*policies`.
- `pkg/webhooks/resource/mpol` — `MutatingPolicy`/`NamespacedMutatingPolicy`, served at
  `/mpol/*policies` / `/nmpol/*policies`.
- `pkg/webhooks/resource/gpol` — `GeneratingPolicy`/`NamespacedGeneratingPolicy`, served at
  `/gpol/*policies` / `/ngpol/*policies`. For non-dry-run requests it calls `h.urGenerator.Apply`
  (`pkg/webhooks/updaterequest/generator.go`) to queue background generation as an `UpdateRequest`;
  a dry-run request returns immediately with no `UpdateRequest` created
  (`admissionutils.IsDryRun(request.AdmissionRequest)` guard). See
  [pkg/background/AGENTS.md](../background/AGENTS.md) for what happens after the `UpdateRequest` is
  created.

`pkg/webhooks/resource/validation/utils.go`'s `NeedsReports` is the one piece of the former shared
resource-admission package kept across the deletion; it's still imported by the surviving CEL
handlers.

## `updaterequest.go` only ever creates a marker CR — it never performs the mutation itself

`handleBackgroundApplies` deliberately uses `context.Background()` + a 30s timeout instead of the inbound HTTP
request context, because the request context is cancelled the moment the webhook handler returns but background
work must outlive it. `Apply` (in `pkg/webhooks/updaterequest/generator.go`) silently no-ops for a `Generate`-type
request with empty `RuleContext`, and otherwise spawns a detached goroutine (`go g.applyResource(context.TODO(), ur)`)
that retries creating the `UpdateRequest` object with exponential backoff, then flips its status to `Pending`. The
real generate/mutate-existing side effects happen later, in `pkg/background`, which watches these objects — this
package's job ends at creating the CR.

## Legacy `kyverno.io` policy writes are denied at the route level, not inside the handlers

Starting in 1.20, `handlers.AdmissionHandler.WithLegacyPolicyDenial()`
(`pkg/webhooks/handlers/legacy.go`) denies create/update requests for the legacy `kyverno.io`
policy kinds (`ClusterPolicy`, `Policy`, `CleanupPolicy`, `ClusterCleanupPolicy`, legacy
`PolicyException`) before they reach `pkg/webhooks/policy`, `pkg/webhooks/exception`, or the
cleanup admission handler. It is wired on the policy and exception routes in `server.go`, and on
the cleanup-controller's server, *outside* `WithSubResourceFilter()` — that filter returns success
for every subresource request before the inner handler runs, so a gate placed inside it would
never see a subresource write and would reintroduce the blanket subresource bypass the 1.20 change
removes. The decision function is `deprecations.DenyLegacyWrite` (`pkg/deprecations/block.go`);
the webhook rule tables it depends on for registration live in `pkg/deprecations/webhookrules.go`,
the single source of truth consumed by `pkg/controllers/webhook`, `cmd/kyverno/main.go`, and
`cmd/cleanup-controller/main.go`. The typed `Validate` methods in this package no longer contain
the block themselves; they only run once the gate has already let a request through.

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
