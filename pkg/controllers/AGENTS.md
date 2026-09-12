# AGENTS.md — pkg/controllers

Reconciler layer wiring the engine/webhooks/background packages into controller-runtime-style loops. See
[docs/dev/controllers/README.md](../../docs/dev/controllers/README.md) for the original controller list (has two
confirmed-stale rows, noted below) and [pkg/controllers/policycache.md](../../docs/dev/controllers/policycache.md)
for a per-controller-doc example.

## Shared pattern every controller follows

Most controllers implement the one-method `Controller` interface (`Run(context.Context, int)`), built via
`internal.NewController(name, ctrl, workers)` in the `cmd/*/main.go` that owns it. Internally: a named
`workqueue.TypedRateLimitingInterface`, informer→queue wiring through `pkg/utils/controller` helpers
(imported as `controllerutils`), and a single shared `controllerutils.Run(ctx, logger, ControllerName, period,
queue, workers, maxRetries, reconcileFunc, routines...)` call — identical across nearly every *queue-backed*
controller in this package. This is not a universal pattern, though — three subdirectories opt out entirely:
`pkg/controllers/generic/logging` doesn't use a workqueue or implement `Controller.Run` at all — it's a
fire-and-forget event logger wired directly to informer callbacks (used for CleanupPolicy/ClusterCleanupPolicy
change logging). `pkg/controllers/metrics/policy` and `pkg/controllers/metrics/updaterequest` are the same shape:
callback-only constructors subscribed directly to informer events (see the table below), with no `Controller.Run`,
no queue, and no `controllerutils.Run` call — don't assume the shared pattern applies when touching these three.

## Subdirectory → responsibility → leader election

Leader-elected = created inside a `leaderelection.New(...)` callback in its owning `cmd/*/main.go`; not
leader-elected = created before/outside that block, so it runs on every replica.

| Subdirectory | Reconciles | Leader-elected? |
|---|---|---|
| `certmanager` | Kyverno's own webhook TLS certificate Secrets | Yes |
| `cleanup` | `CleanupPolicy`/`ClusterCleanupPolicy` and their cron jobs | Yes |
| `deleting` | CEL `DeletingPolicy`/`NamespacedDeletingPolicy` — see [pkg/background/AGENTS.md](../background/AGENTS.md) for why this is not UpdateRequest-driven | Yes |
| `exceptions` | In-memory index of `PolicyException` rule matches, feeding the engine's exception selector | **No** — built alongside the webhook server, outside the leader block |
| `admissionpolicygenerator` | Generates native `ValidatingAdmissionPolicy`/`MutatingAdmissionPolicy` from Kyverno CEL policies, gated by the `GenerateValidatingAdmissionPolicy`/`GenerateMutatingAdmissionPolicy` toggles | Yes |
| `globalcontext` | `GlobalContextEntry`/`ClusterGlobalContextEntry` → in-memory cache store for CEL/JMESPath external lookups | **No** — created/run before/outside the leader-election block in all four binaries that use it (`kyverno`, background, cleanup, reports controllers; in `cmd/kyverno/main.go` it's explicitly under the "start non leader controllers" comment) |
| `metrics/policy` | Updates policy Prometheus metrics from CRD events only — its own code comment calls it "a strange controller, it only processes events" | No (event-driven, no queue) |
| `metrics/updaterequest` | Updates `UpdateRequest` count/status metrics from informer events | No (event-driven, no queue) |
| `policycache` | Keeps an up-to-date in-memory policy cache for webhook-time lookups | **No** — explicitly started before leader election in `cmd/kyverno` |
| `policystatus` | Aggregates and writes `.status` for CEL policy kinds | Yes |
| `report/aggregate` | Aggregates intermediary Admission/BackgroundScan reports into final `PolicyReport`/`ClusterPolicyReport`; uses two separate queues (front/back) | Yes |
| `report/background` | Periodic background policy scans producing `BackgroundScanReport`/`ClusterBackgroundScanReport` | Yes |
| `report/resource` | Tracks which cluster resources participate in reports | Yes |
| `report/utils` | Shared helpers for the three `report/*` controllers, not a controller itself | n/a |
| `ttl` | Deletes TTL-annotated expired resources via a metadata-only client | Yes |
| `webhook` | Creates/reconciles Kyverno's dynamic `ValidatingWebhookConfiguration`/`MutatingWebhookConfiguration`, plus a watchdog routine | Yes |
| `generic/configmap` | Watches the Kyverno config ConfigMap, reloads `config.Configuration` in-memory | No (runs everywhere) |
| `generic/logging` | Generic CRD-change logger, reused for cleanup policy kinds | No (no queue at all) |
| `generic/webhook` | Generic webhook-configuration reconciler, reused for cleanup/ttl webhook configs outside `cmd/kyverno` | Yes (from within leader callbacks) |

## Two confirmed-stale rows in `docs/dev/controllers/README.md`

That table lists `admission-report-controller` and `update-request-controller` as if they were `pkg/controllers/*`
entries. Neither exists as a `ControllerName` string in the current codebase:
- The `admission-report-controller` responsibility appears folded into `report/aggregate`'s two-queue design rather
  than being its own controller.
- `update-request-controller`'s actual code lives **outside `pkg/controllers` entirely**, in the legacy
  `pkg/background` package (`pkg/background/update_request_controller.go`), registered under the name
  `"background-controller"` in `cmd/background-controller/main.go` — not `update-request-controller`. This matches
  that README's own disclaimer ("most controllers... except for some legacy controllers") but the specific name has
  drifted from the code. If you're touching that doc, fix these two rows; don't assume the table is fully current.
