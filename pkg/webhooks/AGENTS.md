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
   string (`"ignore"`, `"fail"`, or `""`/`"all"`) is baked into the closure
   via `handlerFunc`.
2. `handlers.WithAdmission` (the outermost, always-last-attached wrapper)
   reads and JSON-decodes the HTTP body into an `admissionv1.AdmissionReview`,
   builds the internal `handlers.AdmissionRequest` (embedding the raw
   `AdmissionRequest` plus `URLParams` taken from the httprouter path
   parameter), and calls the inner `AdmissionHandler` chain.
3. The chain built in `server.go` runs, in this order, before
   `WithAdmission` is reached (i.e. this is the order requests actually
   flow through, since each `With*` wraps the previous handler):
   `WithFilter` → `WithProtection` → `WithDump` → `WithRoles` →
   [`WithOperationFilter` / `WithSubResourceFilter`, only on some routes] →
   `WithMetrics` → `WithTopLevelGVK` → `WithAdmission`.
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
span. Requests flow through them in the order they are chained in
`server.go`, which for a typical resource webhook route is:

1. **`WithFilter`** (`filter.go`) — short-circuits to an allowed response
   (`admissionutils.ResponseSuccess`) before any policy work happens, if the
   request's user/group/role is in the configured exclusions, the
   resource/namespace matches configmap-based resource filters, or the
   resource `Kind` is a Kyverno-internal reporting kind (`AdmissionReport`,
   `ClusterAdmissionReport`, `BackgroundScanReport`,
   `ClusterBackgroundScanReport`, `UpdateRequest` — see
   `utils.ExcludeKyvernoResources`). This prevents Kyverno's own reporting
   CRs from recursively triggering policy evaluation.
2. **`WithProtection`** (`protect.go`, only enabled when the
   `ProtectManagedResources` toggle is on) — rejects mutations to
   resources labeled `app.kubernetes.io/managed-by: kyverno` unless the
   requesting user is a `system:serviceaccount:<kyverno-namespace>:*`
   service account (namespace-controller deletions are always allowed
   through, to permit namespace cleanup).
3. **`WithDump`** (`dump.go`, only when `DebugModeOptions.DumpPayload` is
   set) — logs the full (secret-redacted) request/response payload at
   `V(4)` after the inner chain runs. Debug-only; never enabled by default.
4. **`WithRoles`** (`enrich.go`) — resolves the requesting user's Roles and
   ClusterRoles via `userinfo.GetRoleRef` and attaches them to the request.
   Policies with role-based match/exclude conditions depend on this having
   already run.
5. **`WithOperationFilter`** / **`WithSubResourceFilter`** (`filter.go`,
   only on specific routes — e.g. mutating webhooks are filtered to
   Create/Update/Connect only, policy/exception/globalcontext webhooks
   filter to empty sub-resource) — an additional early-exit filter beyond
   `WithFilter`.
6. **`WithMetrics`** (`metrics.go`) — records admission metrics
   (allowed/denied, namespace, operation, kind, latency) via
   `metrics.GetAdmissionMetrics()`. No-ops if metrics aren't configured.
7. **`WithTopLevelGVK`** (`enrich.go`) — resolves the *top-level* GVK for
   the request's GVR via discovery (`dclient.IDiscovery.GetGVKFromGVR`) and
   attaches it to `request.GroupVersionKind`. This matters because
   `request.Kind` from the raw `AdmissionRequest` can be a sub-resource or
   custom-resource-specific kind, while policy matching and the policy
   cache key off the canonical top-level kind; several handlers
   (`resource`, `vpol`, `mpol`, `gpol`, `ivpol`) rely on
   `request.GroupVersionKind` rather than `request.Kind` for cache lookups.
8. **`WithAdmission`** (`admission.go`) — the outermost `HttpHandler`
   wrapper; not part of the `AdmissionHandler` chain above but the thing
   that produces it. Parses the HTTP body/AdmissionReview, invokes the full
   `AdmissionHandler` chain, and writes the JSON response. It also applies
   its own `WithMetrics`/`WithTrace` at the HTTP-handler level (distinct
   from step 6, which is at the `AdmissionHandler` level).

Order matters: `WithFilter` and `WithProtection` must run before expensive
work (role resolution, GVK discovery, policy evaluation) so filtered/
protected requests short-circuit cheaply. `WithRoles` must run before any
handler that needs `request.Roles`/`request.ClusterRoles` (i.e. before
`Execute`). `WithTopLevelGVK` must run before `Execute` for the same
reason. Policy/exception/globalcontext webhooks (which validate Kyverno's
own CRDs, not arbitrary resources) skip `WithRoles`/`WithTopLevelGVK`/
`WithProtection`/`WithOperationFilter` — they only need
`WithDump`/`WithSubResourceFilter`/`WithMetrics`/`WithAdmission` — because
role- and GVK-based policy matching doesn't apply when the "resource" being
admitted is itself a policy object.

## FailurePolicy Ignore/Fail Routing

Kyverno registers separate webhook paths per `failurePolicy` value
(`registerWebhookHandlers` in `server.go`): `<base>/ignore`, `<base>/fail`,
plus fine-grained `<base>/ignore/finegrained/*policy` and
`<base>/fail/finegrained/*policy` variants, and an unqualified `<base>`
("all") variant for the legacy resource mutate/validate webhooks. The
`failurePolicy` string ("ignore", "fail", or "") is baked into the
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
