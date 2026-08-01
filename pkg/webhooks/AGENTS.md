# Webhooks Agent Guide

This file provides guidance for coding agents working in `pkg/webhooks/`.
The repository-level `AGENTS.md` also applies.

## Scope

`pkg/webhooks` implements Kyverno's admission webhook HTTP server: it
receives `AdmissionReview` requests from the Kubernetes API server, runs them
through a middleware chain, and dispatches them to the handler responsible
for the webhook type (resource validate/mutate, `ValidatingPolicy`,
`MutatingPolicy`, `GeneratingPolicy`, `ImageValidatingPolicy`, policy
resource validation, `PolicyException` validation, `GlobalContextEntry`
validation, and a liveness/readiness "verify" webhook).

The server itself is built by `NewServer` in `server.go`, which registers
every route on an `httprouter.Router` and wires each one through the
`handlers.AdmissionHandler`/`handlers.HttpHandler` middleware chain defined
in `pkg/webhooks/handlers/`.

`pkg/webhooks/handlers` is generic HTTP/admission plumbing shared by every
route. It does not know about policies, engines, or resources — it only
knows about `AdmissionRequest`/`AdmissionResponse` and how to wrap a
handler function with cross-cutting behavior (filtering, tracing, metrics,
etc). The packages under `pkg/webhooks/resource/`, `pkg/webhooks/policy/`,
`pkg/webhooks/exception/`, `pkg/webhooks/celexception/`, and
`pkg/webhooks/globalcontext/` implement the `Handler` interface
(`types.go`) that the middleware chain ultimately calls.

## Request Flow

1. `net/http` hands the request to the `httprouter.Router` built in
   `server.go`. Route registration determines which `Handler`
   (`ResourceHandlers`, `PolicyHandlers`, `ExceptionHandlers`,
   `CELExceptionHandlers`, `GlobalContextHandlers`, or the static
   `handlers.Verify` func) backs the route, and which `failurePolicy`
   string is baked into the closure via `handlerFunc`. The legacy resource
   webhooks use `"ignore"`, `"fail"` and `"all"` (see
   `registerWebhookHandlers` / `registerWebhookHandlersWithAll`); every
   other route passes `""`, because it does not filter policies by
   failure policy at all.
2. `handlers.WithAdmission` (the outermost, always-last-attached wrapper)
   reads and JSON-decodes the HTTP body into an `admissionv1.AdmissionReview`,
   builds the internal `handlers.AdmissionRequest` (embedding the raw
   `AdmissionRequest` plus `URLParams` taken from the httprouter path
   parameter), and calls the inner `AdmissionHandler` chain.
3. The `AdmissionHandler` middlewares then run. **Each `With*` wraps the
   handler it is called on, so a request executes them in the reverse of
   the order they are chained in `server.go`.** For a typical resource
   route chained as
   `.WithFilter().WithProtection().WithDump().WithRoles()
   [.WithOperationFilter()].WithMetrics().WithTopLevelGVK().WithAdmission()`,
   the actual execution order is:

   `WithTopLevelGVK` → `WithMetrics` →
   [`WithOperationFilter` / `WithSubResourceFilter`, only on some routes] →
   `WithRoles` → `WithDump` → `WithProtection` → `WithFilter` →
   `Handler.Execute`.

   See "Middleware Chain" below for what each step does.
4. The innermost function (built by `handlerFunc` in `server.go`) calls
   `Handler.Execute(ctx, logger, request, failurePolicy, startTime)`.
5. `Execute` is implemented per handler package:
   - `pkg/webhooks/resource` (`resourceHandlers.Validate`/`Mutate`) for the
     legacy per-`ClusterPolicy`/`Policy` admission path (`/validate`,
     `/mutate`, and their `/ignore`, `/fail`, and fine-grained
     `/finegrained/*policy` variants).
   - `pkg/webhooks/resource/vpol`, `mpol`, `gpol`, `ivpol` for the newer CEL
     policy-type-specific routes (`/vpol`, `/nvpol`, `/mpol`, `/nmpol`,
     `/gpol`, `/ngpol`, `/ivpol/validate`, `/ivpol/mutate`,
     `/nivpol/validate`, `/nivpol/mutate`).
   - `pkg/webhooks/policy`, `pkg/webhooks/exception`,
     `pkg/webhooks/celexception`, `pkg/webhooks/globalcontext` for
     validating the Kyverno CRDs themselves (see "Policy-Resource
     Validation vs. Resource Validation" below).
   - `handlers.Verify` for the `/verifymutate` liveness-probe webhook.
6. Handlers that evaluate policy against a resource hand off to
   `pkg/engine` (the classic `engineapi.Engine` for `ClusterPolicy`/`Policy`
   validate/mutate/generate/image-verify, or the CEL engines under
   `pkg/cel/policies/{vpol,mpol,gpol,ivpol}/engine` for the newer policy
   types) to actually run rules/expressions against the admission object.
   `pkg/engine` has its own `AGENTS.md`; this document does not duplicate
   its evaluation flow.
7. The handler converts the engine result into an `admissionv1.
   AdmissionResponse` (allow/deny, patches, warnings), builds
   `PolicyReport`/`AdmissionReport` entries and Kubernetes events as a side
   effect (usually on a background goroutine so the admission response
   isn't delayed), and — for generate / mutate-existing rules — enqueues an
   `UpdateRequest` via `pkg/webhooks/updaterequest` so the background
   controller (outside this tree, in `pkg/background`) performs the actual
   create/update of downstream resources.
8. `WithAdmission` marshals the `AdmissionResponse` back into the
   `AdmissionReview` and writes the JSON HTTP response.

## Important Packages

- `handlers/` — generic, policy-agnostic admission HTTP plumbing: request
  parsing (`admission.go`), the middleware chain (`filter.go`,
  `protect.go`, `dump.go`, `metrics.go`, `trace.go`), role resolution and
  top-level GVK resolution (`enrich.go`), the liveness/readiness probe
  (`probe.go`), and the health-check mutating webhook (`verify.go`).
- `resource/` — dispatch layer (`handlers.go`) for the legacy
  per-`ClusterPolicy`/`Policy` validate/mutate webhooks: retrieves and
  categorizes matching policies from the policy cache, builds the policy
  context, and delegates to `validation/`, `mutation/`, `generation/`, and
  `imageverification/`. Also implements `handleBackgroundApplies`
  (`updaterequest.go`) for mutate-existing and generate side effects.
- `resource/validation/` — `ValidationHandler`: runs `engine.Validate` for
  enforce and audit-warn policies, decides whether to block the request
  (`webhookutils.BlockRequest`), and creates `AdmissionReport`s.
- `resource/mutation/` — `MutationHandler`: runs `engine.Mutate` per policy
  in sequence, threading the patched resource's JSON context into the next
  policy, and accumulates JSON patches.
- `resource/generation/` — `GenerationHandler`: evaluates generate rules
  via `engine.ApplyBackgroundChecks` and creates `UpdateRequest`s for the
  background controller; also handles non-trigger resource changes (e.g.
  clone-source edits) that must resync downstream generated resources.
- `resource/imageverification/` — `ImageVerificationHandler`: runs
  `engine.VerifyAndPatchImages` for legacy `verifyImages` rules
  (`ClusterPolicy`/`Policy`), invoked from the mutating webhook path after
  mutation patches are applied.
- `resource/vpol/` — handler for `ValidatingPolicy`/`NamespacedValidatingPolicy`
  (CEL-based validation), backed by `pkg/cel/policies/vpol/engine`.
- `resource/mpol/` — handler for `MutatingPolicy`/`NamespacedMutatingPolicy`
  (CEL-based mutation, including mutate-existing via `UpdateRequest`s of
  type `kyvernov2.CELMutate`), backed by `pkg/cel/policies/mpol/engine`.
- `resource/gpol/` — handler for `GeneratingPolicy`/`NamespacedGeneratingPolicy`.
  Unlike vpol/mpol/ivpol it does not call a CEL engine directly in the
  webhook path; it only builds and enqueues `UpdateRequest`s
  (`kyvernov2.CELGenerate`) asynchronously — actual generation happens in
  the background controller.
- `resource/ivpol/` — handler for `ImageValidatingPolicy`/
  `NamespacedImageValidatingPolicy`, serving both the mutate phase (digest
  pinning, `/ivpol/mutate`, `/nivpol/mutate`) and the validate phase
  (`/ivpol/validate`, `/nivpol/validate`), backed by
  `pkg/cel/policies/ivpol/engine` and `pkg/image/verification/evaluator`.
- `resource/utils.go`, `resource/fake.go`, `resource/merge_responses_test.go`
  — shared helpers for the `resource` package (patch application, UR
  transforms, response merging, and a fake `Handler` for tests).
- `policy/` — `policyHandlers.Validate`/`Mutate`: validates the Kyverno
  policy CRDs themselves (`ClusterPolicy`, `Policy`, `ValidatingPolicy`,
  `ImageValidatingPolicy`, `MutatingPolicy`, `GeneratingPolicy`,
  `DeletingPolicy`) when they are created/updated, dispatching by type via
  `policy.AsXxxLike()` to the corresponding `pkg/validation/policy` or
  `pkg/cel/policies/{vpol,gpol,mpol}` / `pkg/image/verification/evaluator`
  validators. This is distinct from `resource/`, which validates arbitrary
  Kubernetes resources *against* policies.
- `exception/` — validates legacy `PolicyException` resources
  (`pkg/validation/exception`).
- `celexception/` — validates CEL-based `PolicyException` (`CELPolicyException`)
  resources; shares the same `pkg/validation/exception` package but a
  separate webhook route/handler.
- `globalcontext/` — validates `GlobalContextEntry` resources
  (`pkg/validation/globalcontext`).
- `updaterequest/` — `Generator` interface (`Apply`) used by every
  generate/mutate-existing/CEL-generate/CEL-mutate-existing path in this
  tree to asynchronously create `UpdateRequest` CRs (with retry/backoff)
  that the out-of-tree background controller (`pkg/background`) later
  reconciles. This is under `pkg/webhooks` because it's the fan-out point
  from the synchronous admission path into asynchronous background
  processing, not because it does admission itself.
- `utils/` — shared helpers used across handler packages: request blocking
  logic (`block.go`), Kyverno-managed-resource/report-kind exclusion
  (`exclude.go`), event generation from engine responses (`event.go`),
  delete-operation rule matching (`match.go`), `PolicyContextBuilder`
  (`policy_context_builder.go`), and warning/error message formatting
  (`warning.go`, `error.go`).

## Middleware Chain

All middleware lives in `handlers/` and is attached via `With*` methods
that both apply the behavior and wrap it in `WithTrace` for an OpenTelemetry
span.

**Each `With*` returns a handler that runs its own logic and then calls the
handler it wrapped, so requests execute the middlewares in the reverse of
the order they are chained in `server.go`.** The list below is in
*execution* order for a typical resource webhook route; read the chain in
`server.go` bottom-up to match it.

1. **`WithAdmission`** (`admission.go`) — the outermost wrapper and the
   only `HttpHandler` in the chain. Reads and JSON-decodes the HTTP body
   into an `admissionv1.AdmissionReview`, builds the internal
   `handlers.AdmissionRequest` (the raw `AdmissionRequest` plus
   `URLParams` from the httprouter path parameter), invokes the
   `AdmissionHandler` chain, and writes the JSON response. It applies its
   own `WithMetrics`/`WithTrace` at the HTTP-handler level, distinct from
   the `AdmissionHandler`-level `WithMetrics` below.
2. **`WithTopLevelGVK`** (`enrich.go`) — resolves the *top-level* GVK for
   the request's GVR via discovery (`dclient.IDiscovery.GetGVKFromGVR`) and
   attaches it to `request.GroupVersionKind`. This matters because
   `request.Kind` from the raw `AdmissionRequest` can be a sub-resource or
   custom-resource-specific kind, while policy matching and the policy
   cache key off the canonical top-level kind; several handlers
   (`resource`, `vpol`, `mpol`, `gpol`, `ivpol`) rely on
   `request.GroupVersionKind` rather than `request.Kind` for cache lookups.
3. **`WithMetrics`** (`metrics.go`) — records admission metrics
   (allowed/denied, namespace, operation, kind, latency) via
   `metrics.GetAdmissionMetrics()`. No-ops if metrics aren't configured.
4. **`WithOperationFilter`** / **`WithSubResourceFilter`** (`filter.go`,
   only on specific routes — e.g. mutating webhooks are filtered to
   Create/Update/Connect only, policy/exception/globalcontext webhooks
   filter to empty sub-resource) — an early-exit filter independent of
   `WithFilter`.
5. **`WithRoles`** (`enrich.go`) — resolves the requesting user's Roles and
   ClusterRoles via `userinfo.GetRoleRef` and sets `request.Roles` /
   `request.ClusterRoles`.
6. **`WithDump`** (`dump.go`, only when `DebugModeOptions.DumpPayload` is
   set) — logs the full (secret-redacted) request/response payload at
   `V(4)` after the inner chain returns. Debug-only; never enabled by
   default.
7. **`WithProtection`** (`protect.go`, only enabled when the
   `ProtectManagedResources` toggle is on) — rejects mutations to
   resources labeled `app.kubernetes.io/managed-by: kyverno` unless the
   requesting user is a `system:serviceaccount:<kyverno-namespace>:*`
   service account (namespace-controller deletions are always allowed
   through, to permit namespace cleanup).
8. **`WithFilter`** (`filter.go`) — the last gate before the handler.
   Short-circuits to an allowed response (`admissionutils.ResponseSuccess`)
   if the request's user/group/role is in the configured exclusions
   (`config.IsExcluded`), the resource/namespace matches configmap-based
   resource filters (`config.ToFilter`), or the resource `Kind` is a
   Kyverno-internal reporting kind (`AdmissionReport`,
   `ClusterAdmissionReport`, `BackgroundScanReport`,
   `ClusterBackgroundScanReport`, `UpdateRequest` — see
   `utils.ExcludeKyvernoResources`). This prevents Kyverno's own reporting
   CRs from recursively triggering policy evaluation.
9. `Handler.Execute` — the actual admission logic.

**Why this order:** `withFilter` reads `request.Roles`/`request.ClusterRoles`
(via `config.IsExcluded`) and `request.GroupVersionKind` (via
`config.ToFilter`), so `WithRoles` and `WithTopLevelGVK` **must** run
before it — which is exactly what the chaining produces. When adding a
middleware that populates a field on `AdmissionRequest`, chain it *after*
(i.e. outside) every middleware that reads that field.

Policy, exception, CEL-exception and globalcontext webhooks — which admit
Kyverno's own CRDs rather than arbitrary resources — skip `WithFilter`,
`WithRoles`, `WithTopLevelGVK` and `WithProtection` entirely; they chain
only `WithDump`, `WithSubResourceFilter` (validating routes),
`WithMetrics` and `WithAdmission`, because role- and GVK-based policy
matching doesn't apply when the object being admitted is itself a policy.

## FailurePolicy Ignore/Fail Routing

Kyverno registers separate webhook paths per `failurePolicy` value
(`registerWebhookHandlers` in `server.go`): `<base>/ignore`, `<base>/fail`,
plus fine-grained `<base>/ignore/finegrained/*policy` and
`<base>/fail/finegrained/*policy` variants, and an unqualified `<base>`
("all") variant for the legacy resource mutate/validate webhooks. The
`failurePolicy` string — `"ignore"`, `"fail"` or `"all"` on these legacy
routes, and `""` on every other route, which does no such filtering — is
baked into the
`Handler.Execute` call via `handlerFunc`'s closure and passed through to
`resourceHandlers.retrieveAndCategorizePolicies` → `filterPolicies`, which
filters the policy-cache result set to only policies whose own
`spec.failurePolicy` matches. This lets the webhook *configuration*
(managed elsewhere, in `pkg/controllers/webhook`) register one
MutatingWebhookConfiguration/ValidatingWebhookConfiguration rule per
failurePolicy setting while a single server process serves all of them —
the split happens by URL path, not by separate listeners. The
policy-type-specific routes (`vpol`, `mpol`, `gpol`, `ivpol`) do not use
this ignore/fail path split; CEL policies get one route each
(`/vpol/*policies`, etc.) because failure policy for CEL policies is
resolved inside the CEL engine, not via cache filtering.

## Fine-Grained Webhook Paths

The `*policy` (legacy) and `*policies` (CEL) URL wildcards
(`httprouter`'s catch-all syntax) let a single webhook route encode one or
more specific policy names/namespaces in the request path — this is how
Kyverno supports "one webhook rule per policy" ("fine-grained" webhooks,
see `config.FineGrainedWebhookPath = "/finegrained"`) generated by the
webhook controller in `pkg/controllers/webhook`, as opposed to a single
catch-all webhook that evaluates every cached policy on every request. For
the legacy `resource` handlers, `request.URLParams` (set from the
httprouter `policy` param in `admission.go`) is parsed as
`<namespace>/<name>` or `<name>` in `retrieveAndCategorizePolicies` to look
up exactly one policy via the cluster/namespaced policy lister instead of
querying the policy cache by GVR. For `vpol`/`mpol`/`gpol`/`ivpol`, the
`policies` wildcard param is split on `/` into a list of policy names and
turned into an `engine.MatchNames(...)` predicate passed to the CEL engine.

## Making Changes

- Changes to `handlers/` middleware affect every route registered in
  `server.go` (resource, CEL-policy-type, policy-CRD, exception,
  globalcontext, and verify webhooks alike). Check all call sites in
  `server.go` before changing a middleware's signature or behavior, and
  keep in mind some routes deliberately omit certain middlewares (see
  "Middleware Chain" above) — don't assume every route needs every step.
- When adding a new webhook route, mirror an existing one of the same
  shape in `server.go` as closely as possible (legacy resource-style vs.
  CEL policy-type-style vs. CRD-validation-style have different middleware
  subsets), and remember to register both the plain and
  `/ignore`/`/fail`/fine-grained variants if the new handler filters by
  `failurePolicy`.
- Prefer the smallest package that owns the behavior: business logic for
  evaluating a specific policy type belongs in that type's subpackage
  (`resource/validation`, `resource/mutation`, `resource/vpol`, etc.), not
  in `resource/handlers.go`, which should stay a thin dispatch/aggregation
  layer.
- `resource/handlers.go`'s `retrieveAndCategorizePolicies` and the CEL
  handlers' predicate-building are the places where policy selection
  happens; changes to matching semantics should be verified against both
  the catch-all path (URLParams empty / no policy names in URL) and the
  fine-grained path (a specific policy/policies named in the URL) since
  they use different code paths to select policies.
- Report/event generation is largely fire-and-forget on background
  goroutines (`go h.auditPool.Submit(...)`, bare `go func(){...}()`) so the
  admission response isn't delayed by report writes. When touching this
  code, preserve that a blocked (denied) request must still get a
  best-effort event, and an allowed request's audit-mode/report path must
  not affect `AdmissionResponse.Allowed`.
- `UpdateRequest` creation (`pkg/webhooks/updaterequest`) is the boundary
  between the synchronous admission path and Kyverno's asynchronous
  background reconciliation (`pkg/background`, outside this tree). Do not
  perform generate/mutate-existing side effects synchronously in the
  webhook handler — always go through `urGenerator.Apply`.
- Dry-run admission requests (`admissionutils.IsDryRun`) must not create
  `UpdateRequest`s or other persistent side effects; several handlers
  (`resourceHandlers.Validate`, `mpol.mutate`, `gpol.Generate`) explicitly
  check for this to honor `SideEffects: NoneOnDryRun` in the webhook
  configuration — preserve that check when refactoring.
- `policy/handlers.go`'s `Validate` dispatches on the *new* CRD type first
  (`AsValidatingPolicyLike`, `AsImageValidatingPolicyLike`, etc.) and falls
  back to legacy `AsKyvernoPolicy`. When adding a new policy CRD type that
  needs admission-time validation, add a case here and implement the
  validator in the appropriate `pkg/cel/policies/<type>` or
  `pkg/validation/*` package rather than inlining validation logic in this
  file.

## Testing

Run tests for the package you changed first:

```sh
go test ./pkg/webhooks/<package>
```

For changes to shared middleware or server wiring, run the full webhooks
package tree:

```sh
go test ./pkg/webhooks/...
```

`resource/handlers_test.go`, `resource/validation_test.go`,
`resource/updaterequest_test.go`, and `resource/merge_responses_test.go`
cover the legacy dispatch layer; each policy-type subpackage
(`vpol`, `mpol`, `gpol`, `ivpol`, `generation`, `validation`, `mutation`,
`imageverification`) has its own `*_test.go` alongside the implementation.
`server_test.go` and `types_test.go` cover route registration and the
`Handler`/`HandlerFunc` plumbing.

Before submitting a code change, also follow the repository-level
formatting, linting, code-generation, and testing requirements in
`/AGENTS.md`.
