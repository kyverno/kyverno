# Controllers Agent Guide

This file provides guidance for coding agents working in `pkg/controllers/`.
The repository-level `AGENTS.md` also applies.

## Scope

`pkg/controllers` holds Kyverno's Kubernetes controllers: the long-running
reconcile loops that watch API objects and converge cluster state. They are
what keep the webhook configurations, TLS certificates, in-memory policy
cache, policy reports, policy statuses, generated
ValidatingAdmissionPolicies/MutatingAdmissionPolicies, and the
cleanup/TTL/deletion machinery up to date.

These controllers are **not** admission handlers. The synchronous admission
path lives in `pkg/webhooks` (which has its own `AGENTS.md`), and the
`UpdateRequest`-driven generate/mutate-existing reconcilers live in
`pkg/background`. This tree is the "watch and converge" half of Kyverno.

Controller *construction and wiring* lives outside this tree, in `cmd/` —
each of the four Kyverno binaries builds a different subset of these
controllers. See "Which Binary Runs What" below; it is the single most
important thing to check before changing a controller's constructor
signature.

## The Shared Controller Pattern

Kyverno does **not** use `sigs.k8s.io/controller-runtime` anywhere in
`pkg/controllers` (verified: no file in this tree imports it). Every
controller is hand-rolled on top of client-go: **SharedIndexInformers +
listers + a typed rate-limiting workqueue**, driven by a small set of
helpers in `pkg/utils/controller` (imported almost everywhere as
`controllerutils`).

### The `Controller` interface

`pkg/controllers/controller.go` defines the entire contract:

```go
type Controller interface {
	// Run starts the controller
	Run(context.Context, int)
}
```

That's it — `Run(ctx, workers)`. Every controller in this tree implements
it (`webhook`, `certmanager`, `policycache`, `policystatus`, `cleanup`,
`deleting`, `exceptions`, `globalcontext`, `admissionpolicygenerator`,
`ttl` (as `manager`), `report/{resource,aggregate,background}`,
`generic/configmap`, `generic/webhook`). Two packages widen it:

- `policycache.Controller` = `controllers.Controller` + `WarmUp() error`
- `generic/configmap.Controller` = `controllers.Controller` +
  `WarmUp(context.Context) error`
- `report/resource.Controller` = `controllers.Controller` + `MetadataCache`
  (which itself has a `Warmup(ctx) error`)

`Run` blocks until the context is cancelled. `cmd/internal/controller.go`
wraps a `controllers.Controller` into an `internal.Controller` whose
`Run(ctx, logger, *wait.Group)` starts it on a wait group — that's the type
the `cmd/` binaries actually manipulate.

### `controllerutils.Run` — the worker loop

Nearly every `Run` implementation is a one-liner delegating to
`controllerutils.Run` (`pkg/utils/controller/run.go`):

```go
controllerutils.Run(ctx, logger, ControllerName, time.Second, c.queue,
	workers, maxRetries, c.reconcile, optionalRoutines...)
```

It starts `workers` goroutines, each looping `queue.Get()` →
`reconcile(ctx, logger, key, namespace, name)` → error handling, plus any
number of extra long-lived "routines" (`func(context.Context, logr.Logger)`)
that share the controller's lifetime. Error handling in `handleErr` is
uniform and worth knowing:

- `err == nil` → `queue.Forget(obj)`
- `apierrors.IsNotFound(err)` → **dropped and forgotten**, not retried
- otherwise, retried with `queue.AddRateLimited` until
  `queue.NumRequeues(obj) >= maxRetries` (10 in most packages, 5 in
  `report/resource`, 3 in `policystatus`), then logged and forgotten
- reconcile counts, requeues, and queue drops are reported to
  `metrics.GetControllerMetrics()` keyed by `controllerName`

`reconcile` in `run.go` splits the queue item before calling the
reconcile func: a `cache.ExplicitKey` is passed through verbatim as `key`
with empty `namespace`/`name`; any other string goes through
`cache.SplitMetaNamespaceKey`, so `key` is `"<ns>/<name>"` or `"<name>"`.
**This is why a controller that wants a non-`namespace/name` key format
must enqueue a `cache.ExplicitKey`** (see `policystatus` and
`admissionpolicygenerator` below).

### `controllerutils` event-handler helpers

`pkg/utils/controller/handlers.go` provides the enqueue plumbing:

- `AddDefaultEventHandlers(logger, informer, queue)` — add/update/delete all
  enqueue `cache.MetaNamespaceKeyFunc(obj)`. Update is filtered on
  `ResourceVersion` inequality.
- `AddDefaultEventHandlersT[K]` / `AddEventHandlersT[T]` — typed variants;
  handlers receive the concrete type instead of `interface{}`.
- `AddDelayedDefaultEventHandlers(..., delay)` — enqueues via
  `queue.AddAfter` (used by `report/aggregate` with a 10s delay).
- `AddExplicitEventHandlers(logger, informer, queue, parseKey)` — enqueues a
  `cache.ExplicitKey` produced by `parseKey`, for custom key formats.
- Deletes go through `kubeutils.GetObjectWithTombstone` in
  `AddEventHandlers`, so `DeletedFinalStateUnknown` tombstones are
  unwrapped before the delete callback runs. The typed `AddEventHandlersT`
  wraps its callbacks and then delegates to `AddEventHandlers`, so typed
  delete handlers get an already-unwrapped object too and do **not** need
  to call `GetObjectWithTombstone` themselves. Only code that registers
  handlers directly on an informer, bypassing these helpers, has to handle
  tombstones itself.

### `controllerutils` write helpers

`pkg/utils/controller/utils.go` provides generic, client-agnostic
create/update helpers used throughout: `GetOrNew`, `CreateOrUpdate`,
`Update`, `UpdateStatus`, `Cleanup`, and the `ObjectClient[T]` /
`ObjectStatusClient[T]` interfaces the controllers accept instead of
concrete typed clients (which is what makes them fake-able in tests). All
of them **skip the API call when the built object deep-equals the observed
one** — preserve that when adding fields, or you create hot update loops.
`pkg/utils/controller/metadata.go` has the label/annotation/owner helpers,
including `SetManagedByKyvernoLabel` / `IsManagedByKyverno`
(`app.kubernetes.io/managed-by: kyverno`).

## Important Packages

- `controller.go` — the whole shared abstraction: the one-method
  `controllers.Controller` interface (`Run(ctx, workers)`).
- `generic/configmap/` — reusable controller that watches a single named
  ConfigMap in a single namespace and invokes a `func(ctx, *corev1.ConfigMap) error`
  callback on change; adds `WarmUp(ctx)` which runs the informer, waits for
  cache sync and does one synchronous reconcile. Backs the `config-controller`
  and `metrics-config-controller` in `cmd/internal/configmap.go`.
- `generic/logging/` — not a controller at all despite the name: it
  registers add/update/delete handlers on an informer that only log the
  object name at `V(2)`, gated by predicates (`CheckVersion`,
  `CheckGeneration`). `NewController` returns nothing.
- `generic/webhook/` — reusable controller that owns exactly *one*
  `ValidatingWebhookConfiguration` built from a fixed rule set,
  failurePolicy, sideEffects, path and label selector; re-reconciles on CA
  secret change and on `config.Configuration.OnChanged`. Used for the
  policy-exception, CEL-exception, globalcontext, cleanup-policy and TTL
  webhooks.
- `webhook/` — the big one: dynamically builds the resource
  Mutating/ValidatingWebhookConfigurations from the full set of policies,
  plus the policy-CRD and verify webhooks, and updates legacy
  `ClusterPolicy`/`Policy` `.status`. See "The Webhook Controller" below.
- `policycache/` — keeps `pkg/policycache.Cache` (read by the admission
  handlers in `pkg/webhooks/resource`) in sync with `ClusterPolicy`/`Policy`.
  Only caches policies that are `IsReady()`, have
  `AdmissionProcessingEnabled()`, and do **not** use
  `CustomWebhookMatchConditions()` (those get fine-grained webhooks
  instead); everything else is `Unset`. `WarmUp()` populates it from the
  listers before the webhook server starts serving.
- `certmanager/` — drives `tls.CertRenewer.RenewCA` / `RenewTLS` for the CA
  and TLS secrets. Reconciles on secret events, self-enqueues both secret
  names on `Run` (so they get created if absent), and runs an extra routine
  ticking on `tls.CertRenewalInterval` that re-enqueues every secret.
- `policystatus/` — maintains `.status` (readiness conditions) for the CEL
  policy types (`ValidatingPolicy`, `NamespacedValidatingPolicy`,
  `ImageValidatingPolicy`, `NamespacedImageValidatingPolicy`,
  `MutatingPolicy`, `NamespacedMutatingPolicy`, `GeneratingPolicy`,
  `NamespacedGeneratingPolicy`). Sets the `WebhookConfigured` condition from
  the `webhook.StateRecorder` and the `RBACPermissionsGranted` condition by
  running SubjectAccessReviews for the reports service account. Per-type
  status update logic lives in `vpol.go`, `nivpol.go`, `mpol.go`, `gpol.go`,
  `ngpol.go`, `ivpol.go`.
- `report/resource/` — the "what resources exist" cache for the reports
  pipeline. Computes the set of GVRs any reporting-relevant policy matches,
  starts a dynamic retry-watcher per GVR, and keeps a `uid → (namespace,
  name, hash)` map. Exposes `MetadataCache` (`GetResourceHash`,
  `GetAllResourceKeys`, `UpdateResourceHash`, `AddEventHandler`) that
  `report/background` consumes. Its own workqueue only ever triggers
  `updateDynamicWatchers`.
- `report/background/` — the background scanner: for each resource UID in
  the `report/resource` metadata cache, runs the engine over all
  background-enabled policies and writes an `EphemeralReport` /
  `ClusterEphemeralReport`. Supports partial reconciles (only re-evaluating
  policies whose resourceVersion changed) and a forced full rescan on an
  interval.
- `report/aggregate/` — merges `EphemeralReport`s into the durable
  `PolicyReport`/`ClusterPolicyReport` (or openreports `Report`/
  `ClusterReport`) and deletes the ephemerals it consumed. Two queues; see
  "The Reports Pipeline" below.
- `report/utils/` — shared reports helpers: `scanner.go` (`Scanner.ScanResource`,
  which dispatches a resource to the Kyverno engine and the CEL/VAP/MAP
  evaluators and returns `EngineResponse`s), `events.go` (`GenerateEvents`),
  and `utils.go` (the `Fetch*` lister wrappers, `BuildKindSet`,
  `RemoveNonBackgroundPolicies`, `RemoveNonValidationPolicies`,
  `ReportsAreIdentical`).
- `cleanup/` — executes legacy `CleanupPolicy`/`ClusterCleanupPolicy`: on
  each reconcile, if the policy's cron execution time has passed, it lists
  every matched kind, applies match/exclude/conditions, deletes what
  matches, updates `.status.lastExecutionTime`, then re-enqueues itself with
  `queue.AddAfter(key, timeUntilNextExecution)`.
- `deleting/` — the CEL-based successor to `cleanup`, for
  `DeletingPolicy`/`NamespacedDeletingPolicy`. Same cron + `AddAfter`
  self-scheduling shape, but resolves GVRs via a RESTMapper from
  `spec.matchConstraints`, lists with the policy's `objectSelector`, and
  evaluates via the CEL deleting engine (`pkg/cel/policies/dpol/engine`).
- `ttl/` — two-level: `manager.go` (`NewManager`, `ControllerName =
  "ttl-controller-manager"`) periodically discovers cluster GVRs, filters to
  those the service account can list/watch/delete, and starts/stops one
  `controller.go` instance per GVR; each per-GVR controller runs a metadata
  informer filtered to the `cleanup.kyverno.io/ttl` label and deletes objects
  whose TTL has elapsed. `utils.go` holds `discoverResources`,
  `HasResourcePermissions` (also called from the cleanup-controller's TTL
  admission handler) and `parseDeletionTime`.
- `globalcontext/` — reconciles `GlobalContextEntry` CRs into
  `pkg/globalcontext/store` entries (either a `k8sresource` informer-backed
  entry or an `externalapi` polling entry). Deleting the CR removes the
  store entry; a full store re-enqueues the key after 1 minute
  (`storeFullRequeueDelay`) instead of erroring.
- `exceptions/` — not a reconciler that writes anything: it maintains an
  in-memory `policyName → ruleName → []*PolicyException` index and
  implements `Find(policyName, ruleName)`, which is handed to the engine as
  the `PolicyExceptionSelector`. Wired up by
  `cmd/internal/engine.go:NewExceptionSelector`.
- `admissionpolicygenerator/` — generates native Kubernetes
  `ValidatingAdmissionPolicy`(+Binding) from Kyverno `ClusterPolicy` and
  `ValidatingPolicy`, and `MutatingAdmissionPolicy`(+Binding) from
  `MutatingPolicy`. `controller.go` dispatches on the queue key prefix;
  `generate-vap.go` / `generate-map.go` do the build/create/update/delete;
  `cpol.go`, `vpol.go`, `mpol.go`, `polex.go`, `vap.go`, `map.go` hold the
  informer event handlers; `utils.go` holds the lister lookups and
  `constructBindingName`.
- `metrics/policy/` — no queue, no reconcile: registers informer handlers
  that bump `kyverno_policy_changes_total` on a wait group, and an
  OpenTelemetry observable callback that reports `kyverno_policy_rule_info`
  by listing all policies. Its own comment calls this out: "this is a
  strange controller, it only processes events, this should be changed to a
  real controller."
- `metrics/updaterequest/` — same shape, even smaller: registers an
  OpenTelemetry callback reporting the number of `UpdateRequest`s in the
  Kyverno namespace. `NewController` returns nothing.

## Which Binary Runs What

Kyverno ships four controller-hosting binaries under `cmd/`, each with its
own leader election lease. Controllers created inside the leader-election
callback run **only on the elected leader**; controllers started outside it
run on **every replica**. This table is derived from the `cmd/*/main.go`
wiring, not from the package names.

| Binary | Leader lease | Leader-only controllers | Every-replica controllers |
| --- | --- | --- | --- |
| `cmd/kyverno` (admission controller) | `kyverno` | `webhook`, `certmanager` (unless `--disableCertManagerController`), three `generic/webhook` instances (policy-exception, CEL-exception, globalcontext), `policystatus`, `admissionpolicygenerator` (only if VAP and/or MAP APIs are registered) | `policycache`, `globalcontext`, `exceptions` (if PolicyExceptions enabled), `generic/configmap` ×2, `generic/logging` ×2, `metrics/policy`, `metrics/updaterequest` |
| `cmd/reports-controller` | `kyverno-reports-controller` | `report/resource`, `report/aggregate` (if `aggregateReports`), `report/background` (if `backgroundScan`) | `globalcontext`, `exceptions`, `generic/configmap` ×2 |
| `cmd/background-controller` | `kyverno-background-controller` | `pkg/policy` and `pkg/background` controllers (outside this tree) | `globalcontext`, `exceptions`, `metrics/policy`, `metrics/updaterequest`, `generic/configmap` ×2 |
| `cmd/cleanup-controller` | `kyverno-cleanup-controller` | `certmanager`, two `generic/webhook` instances (cleanup-policy, TTL-label), `cleanup`, `deleting`, `ttl` manager | `globalcontext`, `generic/logging` ×2, `generic/configmap` ×2 |

Consequences worth internalizing:

- **`report/aggregate` and `report/background` only ever run in the reports
  controller**, and only on its leader — so they are single-writer by
  construction. Don't add cross-replica coordination for them; add it to the
  leader election wiring in `cmd/reports-controller/main.go` instead.
- **`certmanager` appears in two binaries** (`kyverno` and
  `cleanup-controller`), each renewing its own CA/TLS secret pair for its
  own webhook server. Changing its constructor means touching both.
- **`globalcontext` runs in all four binaries**, but only `cmd/kyverno`
  passes `shouldUpdateStatus=true` — the other binaries pass `false` so they
  don't fight over `GlobalContextEntry.status`.
- `cmd/kyverno` deliberately reuses the *outer-scope* informer factories
  inside the leader callback (there is a comment saying so) so the webhook
  handlers and the webhook controller share informer caches. Informers keep
  running after leadership is lost; only the controllers stop, because they
  are bound to the leader context.

## The Webhook Controller

`pkg/controllers/webhook` is what `pkg/webhooks/AGENTS.md` refers to as
"managed elsewhere". It owns five webhook configuration objects, keyed in
its queue by their names (from `pkg/config`):

| Queue key | Object | Built by |
| --- | --- | --- |
| `kyverno-resource-mutating-webhook-cfg` | MutatingWebhookConfiguration | `buildResourceMutatingWebhookConfiguration` (or `buildDefaultResourceMutatingWebhookConfiguration` when `--autoUpdateWebhooks=false`) |
| `kyverno-resource-validating-webhook-cfg` | ValidatingWebhookConfiguration | `buildResourceValidatingWebhookConfiguration` / `buildDefault…` |
| `kyverno-policy-mutating-webhook-cfg` | MutatingWebhookConfiguration | `buildPolicyMutatingWebhookConfiguration` |
| `kyverno-policy-validating-webhook-cfg` | ValidatingWebhookConfiguration | `buildPolicyValidatingWebhookConfiguration` |
| `kyverno-verify-mutating-webhook-cfg` | MutatingWebhookConfiguration | `buildVerifyMutatingWebhookConfiguration` |

`reconcile` switches on `name` (the key is a bare name, no namespace, since
these objects are cluster-scoped) and ignores anything else.

### Failure-policy split and fine-grained paths

Legacy `ClusterPolicy`/`Policy` are aggregated by
`buildForPolicies{Mutation,Validation}` into at most two shared `webhook`
structs — `ignoreWebhook` and `failWebhook` — plus one *per-policy*
"fine-grained" webhook for every policy whose spec sets
`CustomWebhookMatchConditions()`. `mergeWebhook` folds each policy's matched
kinds into the target webhook's rule set, resolving GVK→GVR through
discovery (`discoveryClient.FindResources`) and returning `ready=false` if a
resource can't be resolved. `webhookNameAndPath` (in `utils.go`) then derives
the URL the API server will call:

```
<base>/ignore                       or  <base>/fail
<base>/ignore/finegrained/<ns>/<name>  (config.FineGrainedWebhookPath = "/finegrained")
```

which is exactly the route set `pkg/webhooks/server.go` registers. The CEL
policy types take a different route: `buildForJSONPolicies{Mutation,Validation}`
calls `buildWebhookRules` (`validating.go`), which splits policies into
"fine-grained" (those with valid `matchConditions`, `matchPolicy: Exact`, or
a custom `timeoutSeconds`) and "basic" ones. Basic policies are grouped by
their resolved (namespaceSelector, objectSelector) pair — `groupBySelectors`,
keyed on a full SHA-256 of the serialized selectors — and each group gets one
`-ignore` and one `-fail` webhook whose path is `/vpol/<name1>/<name2>/...`
(the sorted, `/`-joined policy names). Fine-grained CEL webhooks get
`path.Join(queryPath, policyName)` and a per-policy name.

### Watchdog / health lease

`Run` starts a `watchdog` routine alongside the workers. Every
`tickerInterval` (10s) it upserts the `kyverno-health` Lease in the Kyverno
namespace with a `kyverno.io/last-request-time` annotation and re-enqueues
the two resource webhook keys. `watchdogCheck()` returns true only if that
annotation is younger than `IdleDeadline` (100s). Two invariants hang off
this and **must not be broken**:

1. If `watchdogCheck()` is false, `reconcile` requeues the resource webhook
   keys after 1s and **does not rebuild** the configuration — rebuilding
   with unconfirmed health would publish an empty webhook configuration and
   drop every rule (startup, leader change, cluster resumed after outage).
2. `updatePolicyStatuses` returns early when `watchdogCheck()` is false, so
   policies are not downgraded to `NotReady`. The in-code comment cites
   #11560 / #16281: marking them NotReady evicts them from the policy cache,
   and the handler then admits requests unvalidated, silently skipping
   `failurePolicy: Fail` rules.

`runtime.IsRollingUpdate()` short-circuits the same way (requeue, don't
rebuild).

### State recorder

`recorder.go` holds a tiny `StateRecorder` (`Record`, `Ready`, `Reset`,
`NotifyChannel`) shared between this controller and `policystatus`. Keys are
built by `BuildRecorderKey(policyType, name, namespace)` in the format
`Kind/name` for cluster-scoped and `Kind/name+namespace` for namespaced
policies, and parsed back by `ParseRecorderKey`. The webhook controller
calls `Record` for every CEL policy it successfully wrote into a webhook
configuration, and `Reset` when a reconcile fails; `policystatus`'s
`watchdog` routine drains `NotifyChannel()` into its own queue as
`cache.ExplicitKey`s. This is the only cross-controller channel in the tree
— and it means the two controllers must run in the same process
(`cmd/kyverno`, both leader-only).

### Other webhook-controller details

- `objectMeta` always stamps `webhook.kyverno.io/managed-by: kyverno`
  (`kyverno.LabelWebhookManagedBy`) on the configurations it creates, and
  `buildOwner` sets an ownerReference to the Kyverno ClusterRole matching
  `*:webhook` so the configurations are garbage-collected with the release.
- `bootstrap_exclusion.go` appends a `kyverno-exclude-bootstrap-resources`
  matchCondition to every **Fail** webhook when `--excludeBootstrapResources`
  is set, excluding `nodes` and `certificatesigningrequests` so a cold
  cluster can't deadlock on an unavailable Kyverno. It is idempotent
  (`hasBootstrapExclusion`) because the API server rejects duplicate
  matchCondition names.
- `expression_cache.go` caches CEL matchCondition compilation results by
  `sha256(name:expression)`; `validConditions` drops a policy's
  matchConditions wholesale if *any* of them fails to compile against the
  Kubernetes CEL environment, which is what pushes that policy onto the
  fine-grained path.
- `utils_webhook.go`'s `webhook.buildRulesWithOperations` deduplicates rules
  subsumed by wildcards (`hasRule`) and always adds
  `pods/ephemeralcontainers` alongside `pods`.
- `restrictMatchConditionsToCreate` (in `validating.go`) rewrites a
  synchronize-enabled GeneratingPolicy's matchConditions to
  `request.operation != 'CREATE' || (<original>)` so UPDATE/DELETE of a
  trigger still reaches Kyverno and downstream resources can be cleaned up.

## The Reports Pipeline

Three controllers, all leader-only in `cmd/reports-controller`, plus the
admission path in `pkg/webhooks`:

1. **Producers.** Admission handlers (`pkg/webhooks/resource/**`) and the
   background reconcilers (`pkg/background/**`) call
   `reportutils.CreateEphemeralReport`, writing an `EphemeralReport` /
   `ClusterEphemeralReport` labelled `app.kubernetes.io/managed-by: kyverno`
   with `source: admission`. Admission-sourced reports carry the resource
   UID in a label but **no ownerReference**.
2. **`report/resource`** discovers which GVRs matter (union of kinds matched
   by ClusterPolicy/Policy/vpol/nvpol/mpol/nmpol/ivpol/nivpol/VAP/MAP,
   filtered to kinds supporting list+watch and passing
   `reportutils.IsGvkSupported`), starts a `watchTools.RetryWatcher` per GVR,
   and maintains `uid → Resource{Namespace, Name, Hash}`. Watchers that die
   with HTTP 410 Gone are restarted through `watchDeathChan`. It notifies
   registered `EventHandler`s on Added/Modified/Deleted/Stopped.
3. **`report/background`** subscribes to that handler; every non-delete
   event enqueues `"<namespace>/<uid>"` (or just `"<uid>"` for
   cluster-scoped) after a 30s delay. `reconcile` looks the UID up in the
   metadata cache — if it's gone, the ephemeral report is deleted.
   `needsReconcile` decides between: no work, a **partial** reconcile (some
   policy/exception/binding resourceVersion changed — existing results for
   unchanged policies are kept), and a **full** reconcile (no report yet,
   resource hash changed, or `audit.kyverno.io/last-scan-time` older than
   `--backgroundScanInterval`). Results are written via
   `breaker.GetReportsBreaker()`, and every reconcile self-requeues with
   `AddAfter(key, forceDelay)`.
4. **`report/aggregate`** runs two queues:
   - **frontQueue** (fed by delayed ephemeral-report informer events, 10s
     delay) does *adoption*: for an ephemeral report with no ownerReference,
     it finds the target resource, sets the ownerReference and the resource
     UID, and re-saves. If the resource can't be fetched due to RBAC, or the
     report is older than `deletionGrace` (2 min) and the resource is gone,
     the report is deleted. If the report is already owned, it pushes
     `"<namespace>/<ownerUID>"` onto the backQueue.
   - **backQueue** does the actual aggregation, keyed by
     `"<namespace>/<resourceUID>"`: fetch the existing
     PolicyReport/ClusterPolicyReport (name == resource UID), fetch all
     ephemeral reports labelled with that resource UID, merge them with
     `MergeReports` (dropping results whose policy no longer exists, keeping
     the newest by `Timestamp.Seconds` on key collision), write the durable
     report, and — only if aggregation succeeded — delete the ephemeral
     reports it consumed. Empty result set ⇒ the durable report is deleted.
   - A background goroutine started in `NewController` prunes the
     `reportUUIDToPolicyCache` (report UID → set of policy keys) every 10s;
     that cache is what lets a single policy change enqueue only the reports
     that actually reference it instead of all of them. When the cache is
     empty (cold start) it falls back to enqueuing everything.

## Invariants and Gotchas

- **Queue key formats are not uniform.** Most controllers use
  `cache.MetaNamespaceKeyFunc` (`namespace/name`). Exceptions:
  - `webhook` enqueues bare webhook-configuration names.
  - `policystatus` enqueues `cache.ExplicitKey`s in `Kind/name` or
    `Kind/name+namespace` form (`webhook.BuildRecorderKey`), so its
    `reconcile` receives them as `key` with empty `namespace`/`name` and
    must call `webhook.ParseRecorderKey`.
  - `admissionpolicygenerator` enqueues `"<PolicyKind>/<metaNamespaceKey>"`
    (e.g. `ClusterPolicy/foo`, `ValidatingPolicy/foo`, `MutatingPolicy/foo`)
    and dispatches on `strings.Split(key, "/")[0]`.
  - `report/background` and `report/aggregate`'s backQueue key by
    **resource UID**, not resource name.
  - `exceptions` enqueues *policy* names derived from the
    `PolicyException`'s `spec.exceptions[].policyName`, not the exception's
    own key.
- **`IsNotFound` errors are swallowed by the shared worker loop.** Returning
  a NotFound error from `reconcile` silently forgets the key instead of
  retrying. Handle deletion explicitly (as `policycache`, `globalcontext`
  and `report/background` do) rather than relying on the retry path.
- **Cron-style controllers self-schedule.** `cleanup` and `deleting` end
  every successful reconcile with `queue.AddAfter(key, delay)` and clamp
  `delay` to `minRequeueDelay` (1s) when the next execution time is in the
  past — removing that clamp reintroduces a hot loop. Both also filter
  informer updates on `GetGeneration()` so status writes don't re-trigger
  them.
- **Deep-equal guards prevent write storms.** `controllerutils.Update` /
  `UpdateStatus` / `CreateOrUpdate` skip the API call when nothing changed,
  and `generic/configmap` additionally short-circuits on unchanged
  `resourceVersion`. `report/background` has `utils.ReportsAreIdentical`.
  Adding a timestamp or a randomized field to any built object defeats all
  of these.
- **`WarmUp` must run before the thing that reads the data.** `policycache`'s
  `WarmUp()` is called by `cmd/kyverno`'s non-leader bootstrap before the
  webhook server starts; `report/resource`'s `Warmup(ctx)` is called by the
  reports-controller's leader warmup before the scanners start;
  `generic/configmap`'s `WarmUp(ctx)` is called synchronously (and fatally
  checked) before `Run`.
- **Generated / managed-resource labelling.** Objects Kyverno creates carry
  `app.kubernetes.io/managed-by: kyverno`
  (`controllerutils.SetManagedByKyvernoLabel`); the `cleanup` controller
  refuses to delete such objects when the `ProtectManagedResources` toggle
  is on, and `report/aggregate` selects ephemeral/durable reports by that
  label. Webhook configurations use the *separate*
  `webhook.kyverno.io/managed-by` label instead.
- **`ttl` starts and stops informers at runtime.** The manager holds a lock
  around `resController` and cancels a per-GVR context to stop both the
  informer and the controller; the per-GVR controller's `Stop()` also
  deregisters its event handler. It does a `preflightCheck` (a limit-1 List)
  before starting an informer so a 403 doesn't produce a hot-retrying
  informer.
- **Concurrency assumptions.** Controllers that mutate shared in-memory
  state guard it explicitly: `webhook` (`policyState` under `sync.Mutex`),
  `exceptions` (`index` under `sync.RWMutex`), `report/resource`
  (`dynamicWatchers`/`eventHandlers` under `sync.RWMutex`),
  `report/aggregate` (`reportUUIDToPolicyCache` under a `*sync.Mutex`),
  `webhook.Recorder`, `ttl.manager`. Worker counts are per-controller
  constants (`Workers`) — e.g. `certmanager` and `globalcontext` use 1,
  `webhook` and `admissionpolicygenerator` use 2, `policycache`/`cleanup`/
  `deleting`/`exceptions`/`policystatus`/`ttl` use 3 — and `report/aggregate`
  / `report/background` take theirs from CLI flags rather than the package
  constant. If you raise a worker count, re-check what that controller
  mutates.
- **`metrics/policy`, `metrics/updaterequest` and `generic/logging` are not
  reconcilers.** They have no queue and no `Run`; their constructors return
  nothing (or nothing useful) and just register informer handlers and/or
  OpenTelemetry observable callbacks. Don't try to wire them through
  `internal.NewController`.

## Making Changes

- **Adding a controller**: implement `controllers.Controller`
  (`Run(ctx, workers)`), build a
  `workqueue.NewTypedRateLimitingQueueWithConfig(...)` named after the
  controller, register handlers with `controllerutils.AddDefaultEventHandlers`
  (or a typed/explicit variant), and delegate `Run` to `controllerutils.Run`.
  Export `ControllerName` and `Workers` constants and a package-level
  `logger` in `log.go` (`logging.ControllerLogger(ControllerName)`), matching
  every existing package. Then wire it in the right `cmd/*/main.go` — and be
  deliberate about whether it goes in `createNonLeaderControllers` or
  `createrLeaderControllers`.
- **Anything that writes cluster-wide state must be leader-only.** Anything
  that only fills a per-process in-memory cache read by that process's own
  request path (`policycache`, `exceptions`, `globalcontext`) must run on
  every replica. Getting this backwards produces either write conflicts or
  replicas serving admission from an empty cache.
- **Changing a constructor signature is a `cmd/` change too.** These
  constructors take a dozen-plus informers each and several are called from
  more than one binary (`certmanager`, `globalcontext`, `generic/webhook`,
  `generic/configmap`). Grep `cmd/` for the package before you start.
- **Informers passed as `nil` are load-bearing.** `report/resource`,
  `report/background`, `report/aggregate` and `admissionpolicygenerator` all
  accept optional VAP/MAP informers that are `nil` when the cluster doesn't
  serve those APIs (and the MAP version — v1 / v1beta1 / v1alpha1 — is
  chosen at startup by `admissionpolicy.PreferredMutatingAdmissionPolicyVersion`).
  Every use is nil-guarded; keep new code guarded the same way rather than
  assuming a lister exists.
- **Touching `pkg/controllers/webhook` build functions**: the resulting
  webhook paths are a hard contract with the routes registered in
  `pkg/webhooks/server.go`. Changing a path, a name suffix (`-ignore` /
  `-fail` / `-finegrained-…`), or the fine-grained key format requires the
  matching change there. Also preserve the `watchdogCheck()` guards — see
  above; they are load-bearing correctness, not defensive coding.
- **Touching the reports pipeline**: keep the ephemeral → durable direction
  intact. `report/background` writes ephemeral reports only;
  `report/aggregate` is the only writer of durable
  PolicyReports/ClusterPolicyReports and the only deleter of ephemeral
  reports, and it deletes them only after a successful merge. The openreports
  vs. wgpolicyk8s output is a runtime switch (`orClient != nil`) threaded
  through `getReport` / `NewPolicyReport` / `updateReport` — new code paths
  need both branches.
- **Prefer the generic controllers over a new bespoke one.** If you need to
  manage one more `ValidatingWebhookConfiguration` with a fixed rule set,
  instantiate `generic/webhook` in `cmd/` (as the exception, CEL-exception,
  globalcontext, cleanup and TTL webhooks all do) instead of adding a
  package here. Same for watching a single ConfigMap
  (`generic/configmap`).
- **Don't add resync-driven full reconciles.** The informer factories are
  created with a shared `--resyncPeriod` in `cmd/`; individual controllers
  get periodic behavior from an explicit `AddAfter` (cleanup, deleting,
  background scan) or an explicit ticker routine passed to
  `controllerutils.Run` (certmanager, webhook watchdog). Follow one of those
  two patterns so the cadence is visible in the package.

## Testing

Run tests for the package you changed first:

```sh
go test ./pkg/controllers/<package>
```

For changes to the shared abstraction, the generic controllers, or anything
touching more than one package, run the whole tree:

```sh
go test ./pkg/controllers/...
```

If you changed `pkg/utils/controller` (the shared queue/handler/update
helpers every controller here depends on), run that too:

```sh
go test ./pkg/utils/controller/...
```

Existing test files in this tree:

- `webhook/`: `controller_test.go`, `utils_test.go`, `validating_test.go`,
  `mpol_test.go`, `recorder_test.go`, `bootstrap_exclusion_test.go` — by far
  the densest coverage, and the place to add a case when you change rule
  building, webhook naming, or the fine-grained split.
- `report/aggregate/controller_test.go`, `report/utils/scanner_test.go`,
  `report/utils/utils_test.go`.
- `admissionpolicygenerator/`: `generate_map_test.go`, `mpol_test.go`,
  `polex_test.go`, `utils_test.go`.
- `cleanup/controller_test.go`, `deleting/controller_test.go`,
  `policystatus/controller_test.go`, `ttl/controller_test.go`,
  `ttl/utils_test.go`.

`report/resource` and `report/background` currently have no unit tests;
they are exercised by the chainsaw conformance tests under
`test/conformance/`. `generic/*`, `certmanager`, `policycache`,
`exceptions`, `globalcontext` and `metrics/*` have no unit tests either —
if you touch them, the `pkg/utils/controller` tests plus a conformance run
are the practical safety net.

Before submitting a code change, also follow the repository-level
formatting, linting, code-generation, and testing requirements in
`/AGENTS.md`.
