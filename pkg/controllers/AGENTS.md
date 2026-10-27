# AGENTS.md — pkg/controllers

Reconciler layer wiring the engine/webhooks/background packages into controller-runtime-style loops. See
[docs/dev/controllers/README.md](../../docs/dev/controllers/README.md) for the original controller list (has two
confirmed-stale rows, noted below).

## Shared pattern every controller follows

Most controllers implement the one-method `Controller` interface (`Run(context.Context, int)`), built via
`internal.NewController(name, ctrl, workers)` in the `cmd/*/main.go` that owns it. Internally: a named
`workqueue.TypedRateLimitingInterface`, informer→queue wiring through `pkg/utils/controller` helpers
(imported as `controllerutils`), and a single shared `controllerutils.Run(ctx, logger, ControllerName, period,
queue, workers, maxRetries, reconcileFunc, routines...)` call — identical across nearly every *queue-backed*
controller in this package. This is not a universal pattern, though — several subdirectories opt out:
`pkg/controllers/generic/logging` doesn't use a workqueue or implement `Controller.Run` at all — it's a
fire-and-forget event logger wired directly to informer callbacks. #17710 PR 4 deleted its last caller
(`cmd/cleanup-controller/main.go`'s CleanupPolicy/ClusterCleanupPolicy change logging); nothing in the tree
constructs it any more, so treat it as dead code pending removal, not a live pattern to copy.
`pkg/controllers/metrics/updaterequest` is the same shape: a callback-only constructor
subscribed to an OTel scrape callback and backed by an informer lister, with no `Controller.Run`, no queue, and
no `controllerutils.Run` call. `pkg/controllers/metrics/legacypolicy` is a third shape, not either of the above:
it does implement `Controller.Run`, wired through `internal.NewController` like a queue-backed controller, but
`Run` drives a plain `wait.UntilWithContext` timer loop (`refreshInterval`, 30s) instead of a workqueue. There
is no informer behind it because #17710 PR 4 deleted the legacy engine's typed `kyverno.io` listers, which
forced the gauge onto metadata-only `list` calls instead of a watch — and that, in turn, is what let the chart
narrow the admission/cleanup controllers' RBAC for these kinds down to `list` only (no `watch`). Don't assume
the queue pattern applies to any of these three.

## Subdirectory → responsibility → leader election

Leader-elected = created inside a `leaderelection.New(...)` callback in its owning `cmd/*/main.go`; not
leader-elected = created before/outside that block, so it runs on every replica.

| Subdirectory | Reconciles | Leader-elected? |
|---|---|---|
| `certmanager` | Kyverno's own webhook TLS certificate Secrets | Yes |
| `deleting` | CEL `DeletingPolicy`/`NamespacedDeletingPolicy` — see [pkg/background/AGENTS.md](../background/AGENTS.md) for why this is not UpdateRequest-driven | Yes |
| `admissionpolicygenerator` | Generates native `ValidatingAdmissionPolicy`/`MutatingAdmissionPolicy` from Kyverno CEL policies, gated by the `GenerateValidatingAdmissionPolicy`/`GenerateMutatingAdmissionPolicy` toggles | Yes |
| `globalcontext` | `GlobalContextEntry`/`ClusterGlobalContextEntry` → in-memory cache store for CEL/JMESPath external lookups | **No** — created/run before/outside the leader-election block in all four binaries that use it (`kyverno`, background, cleanup, reports controllers; in `cmd/kyverno/main.go` it's explicitly under the "start non leader controllers" comment) |
| `metrics/legacypolicy` | Reports the `kyverno_legacy_policies_total` gauge at scrape time, from counts refreshed every 30s via metadata-only `list` calls for the legacy kinds the admission and cleanup controllers each cover | **No** — created/run before/outside the leader-election block, same as `globalcontext` above |
| `metrics/updaterequest` | Updates `UpdateRequest` count/status metrics from an informer lister, read at scrape time | No (scrape-time callback, no queue) |
| `policystatus` | Aggregates and writes `.status` for CEL policy kinds | Yes |
| `report/aggregate` | Aggregates intermediary Admission/BackgroundScan reports into final `PolicyReport`/`ClusterPolicyReport`, dropping results whose policy no longer exists and every legacy `source: kyverno` result; uses two separate queues (front/back) and, while legacy `ClusterPolicy`/`Policy` are still present, runs a one-time startup sweep over every existing report | Yes |
| `report/background` | Periodic background policy scans producing `BackgroundScanReport`/`ClusterBackgroundScanReport` | Yes |
| `report/resource` | Tracks which cluster resources participate in reports | Yes |
| `report/utils` | Shared helpers for the three `report/*` controllers, not a controller itself | n/a |
| `ttl` | Deletes TTL-annotated expired resources via a metadata-only client | Yes |
| `webhook` | Creates/reconciles Kyverno's dynamic `ValidatingWebhookConfiguration`/`MutatingWebhookConfiguration`, plus a watchdog routine | Yes |
| `generic/configmap` | Watches the Kyverno config ConfigMap, reloads `config.Configuration` in-memory | No (runs everywhere) |
| `generic/logging` | Generic CRD-change logger — unused since #17710 PR 4 deleted its last caller | n/a (no caller) |
| `generic/webhook` | Generic webhook-configuration reconciler, reused for cleanup/ttl webhook configs outside `cmd/kyverno` | Yes (from within leader callbacks) |

`generic/webhook.NewController` takes a trailing `...Option`. The only option, `WithoutConfigMatchConditions()`,
stops `build()` from stamping the ConfigMap's `config.matchConditions` onto the webhook configuration; the zero
value keeps stamping them, so every existing caller is unaffected. Only the two 1.20 legacy-policy-denial
configurations (the legacy `PolicyException` webhook in `cmd/kyverno/main.go` and the `CleanupPolicy`/
`ClusterCleanupPolicy` webhook in `cmd/cleanup-controller/main.go`) pass it, because a documented
`config.matchConditions` value (for example "skip this service account") would otherwise exempt a principal from
the legacy-write denial for those two configurations. Do not pass it to a resource-facing webhook.

## Admission-policy generation status

Generation and policy-status controllers both update CEL policy status. The admission-policy generator must
return status-write errors through reconciliation so the workqueue retries resource-version conflicts using a
fresh informer object. Logging and swallowing a conflict can leave `status.generated` false indefinitely even
after the native admission policy and binding were created.

## Two confirmed-stale rows in `docs/dev/controllers/README.md`

That table lists `admission-report-controller` and `update-request-controller` as if they were `pkg/controllers/*`
entries. Neither exists as a `ControllerName` string in the current codebase:
- The `admission-report-controller` responsibility appears folded into `report/aggregate`'s two-queue design rather
  than being its own controller.
- `update-request-controller`'s actual code lives **outside `pkg/controllers` entirely**, in the
  `pkg/background` package (`pkg/background/update_request_controller.go`), registered under the name
  `"background-controller"` in `cmd/background-controller/main.go` — not `update-request-controller`. Despite its
  name, `pkg/background` itself only processes the CEL `gpol`/`mpol` request types now (#17710 PR 2); it is not a
  "legacy controller" in the sense the README's disclaimer means. This matches that README's own disclaimer
  ("most controllers... except for some legacy controllers") but the specific name has drifted from the code. If
  you're touching that doc, fix these two rows; don't assume the table is fully current.
