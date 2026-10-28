# Legacy policy removal (1.20): stale reports, non-Helm/GitOps preflight, and rollback limits

Companion to [removed-legacy-go-api.md](./removed-legacy-go-api.md) (the Go-side removal) and
[`config/crds/kyverno/README.md`](../../../config/crds/kyverno/README.md) (the frozen CRD schemas).
This page covers three operational questions #17707 asks to be documented separately: what happens
to existing `PolicyReport`/`ClusterPolicyReport` entries, what protects an install or upgrade that
doesn't go through `helm install`/`helm upgrade`, and what a rollback to a pre-1.20 release does and
doesn't restore.

## Stale reports

Before 1.20, legacy `kyverno.io` policies produced ordinary report results like any other policy
type. After 1.20 removes their execution, no new result referencing a legacy policy is ever produced
again — but results already written under 1.19 don't disappear on their own.

`pkg/controllers/report/aggregate/controller.go` handles this with a one-time startup sweep
(`legacyPoliciesPresent` / `enqueueAllReports`, internally called "decision C2"): on controller start,
if any `kyverno.io/v1` `ClusterPolicy` or `Policy` object still exists on the cluster, every managed
permanent report is re-queued once. The pruning itself happens on that reconcile's normal path
(`backReconcile`), not as a special case: it feeds the report's existing results back into
`MergeReports` alongside the current `EphemeralReport`s. `MergeReports` keeps a result only if its
`Source` is one of the live CEL, VAP, or MAP sources *and* the originating policy is still found in
the matching policy map. Everything else falls through to the default arm, which consults a legacy
policy map that `backReconcile` never populates — so every legacy (`SourceKyverno`) result is dropped
on that pass, as is any result carrying an unknown or empty source.

**A report left with no results at all is deleted, not emptied.** When the merge yields zero results,
`backReconcile` calls `deleteReport` and the `PolicyReport`/`ClusterPolicyReport` object (or its
`Report`/`ClusterReport` equivalent under OpenReports) is removed. On a cluster whose results came
only from legacy policies, that means the reports themselves
disappear on the sweep rather than remaining as empty shells. This is expected, not data loss: the
objects held nothing but results that no longer have a producing policy.

If no legacy policy objects exist at startup, the sweep itself is skipped as unnecessary work. A
report that already held stale `SourceKyverno` results *before* the legacy policy that produced them
was deleted isn't swept by this path. The controller's ordinary re-queue trigger covers part of that
gap but is not a substitute for the sweep, and the difference matters:

- **The startup sweep reaches both scopes.** `enqueueAllReports` lists namespaced reports and
  cluster-scoped reports separately, queueing every one.
- **The ordinary trigger reaches namespaced reports only, while the cache is cold.** Adding,
  updating, or deleting any `policies.kyverno.io`, VAP, or MAP policy calls
  `enqueueReportsForPolicy`. That function checks the namespaced reports first, and when the
  report-to-policy cache is empty — which it always is immediately after start, while the policy
  informers replay their initial Add events — it queues those and **returns**, before it ever reaches
  its cluster-scoped branch. So a cold-cache policy event never queues a
  `ClusterPolicyReport`/`ClusterReport`. On a cluster with no namespaced reports at all, the cache
  stays empty through this path and that early return queues nothing whatsoever.
- Once the cache has been populated (by reports that did get reconciled), both branches run, so
  cluster-scoped reports become reachable on a *later* policy event. On a cluster that only ever has
  cluster-scoped reports, nothing populates the cache through this path, and they stay stale.

The practical consequence: for a `ClusterPolicyReport`/`ClusterReport` carrying stale legacy results,
the startup sweep is effectively the only reliable cleanup path. If the legacy policies were deleted
before the restart into 1.20, the sweep is skipped and those cluster-scoped stale results can persist
indefinitely. Namespaced reports are much more likely to be cleaned up incidentally.

If the presence check itself errors, the sweep is skipped for that start rather than risking a
cluster-wide read burst against an already-unhealthy API server. Nothing retries it inside the
running process — the next attempt comes with the next controller start, so a new leader. For
cleanup purposes that is fail-open, not fail-closed: the stale results stay until something else
re-queues those reports. Look for the controller's Error log from the presence check if reports
aren't being swept when you expect them to be.

This covers both report backends: when OpenReports is enabled (`orClient != nil`), the permanent
objects being read, merged, and written back are `Report`/`ClusterReport`, not
`PolicyReport`/`ClusterPolicyReport` — same mechanism, different CRD kind.

Practically: if legacy policies are still present when a cluster restarts into 1.20, every report in
both scopes gets its legacy-sourced results pruned on that one pass, with no manual cleanup step, and
any report left holding nothing else is deleted. If every legacy policy was already deleted *before*
that restart, the startup sweep has nothing to trigger it. Namespaced reports still carrying stale
results are then likely to be cleared incidentally by the next policy event; cluster-scoped ones
quite possibly are not, and may need the operator to delete or re-trigger them by hand.

## Non-Helm / GitOps preflight

The automatic "don't let an operator silently lose enforcement" gate is two independent mechanisms,
both in `charts/kyverno/templates/`, that run on different conditions:

- `validate-legacy-policies.yaml` — a chart-render-time `lookup` against the live cluster that `fail`s
  the render if any of the five legacy kinds still have instances, unless
  `upgrade.acknowledgeLegacyPoliciesNotEnforced=true` is set. **`lookup` is not populated by an
  ordinary `helm template`, even one run with full cluster access** — checked against Helm v4.2.3
  with a minimal chart whose only template calls `lookup`: empty under plain
  `helm template --kube-context <ctx>` and under `helm install --dry-run=client`, populated under
  `helm template --dry-run=server` (the current name for the deprecated `--validate` flag), under
  `helm install --dry-run=server`, and under a real `helm install`/`helm upgrade`. This is Helm's own
  behavior, not specific to this chart. It is also version-sensitive — `--dry-run=server` has not
  always existed on `helm template` — so confirm it against your own Helm version before relying on
  a render-time gate.
- `templates/hooks/pre-install-check-legacy-policies.yaml` — a `pre-install,pre-upgrade` Helm hook
  `Job` running `kyverno check-legacy-policies` (the same check, as a cluster-side binary instead of
  Helm template logic). Suppressed when `global.templating.enabled=true`, which is how this repo's own
  `config/install-latest-testing.yaml` is generated for CI and how the release workflow builds the
  `install.yaml` asset it attaches to each GitHub release (there is no release-tracked
  `config/install.yaml` path in this tree) — there is no live cluster to look up against while
  building a manifest for publication, and no hook execution model for a plain manifest either.

**Concretely, by install method:**

| Method | `lookup` gate | Hook `Job` |
|---|---|---|
| Real `helm install`/`helm upgrade` | Runs | Runs¹ |
| `helm install --dry-run=server`/`helm upgrade --dry-run=server` | Runs | **Does not run** — a dry run never creates the Job, server-side validation notwithstanding |
| Flux's `helm-controller` (`HelmRelease`) | Runs — it performs a genuine Helm install/upgrade, not just templating | Runs¹ |
| ArgoCD's native Helm source type | **Does not run** — ArgoCD renders with `helm template`, it does not install/upgrade a Helm release | Runs, via ArgoCD's own Helm-hook-to-sync-wave translation, unless hooks are skipped for that app/resource |
| `helm template --dry-run=server` | Runs — server-side rendering populates `lookup` | **Does not run** — no hook execution model outside a real Helm install/upgrade |
| Plain `helm template` (even with `--kube-context` set), or any pipeline that renders the chart once and applies/commits the resulting plain YAML | **Does not run** | **Does not run** — no hook execution model outside a real Helm install/upgrade |
| The published release `install.yaml` asset, or `config/install-latest-testing.yaml` | **Does not run** | **Does not run** |

¹ "Runs" here means the chart renders the Job and Helm executes a real install/upgrade — not that
nothing can skip it. Three independent, generic bypasses still apply regardless of install method:
this chart's own `upgrade.legacyPolicyCheck.enabled=false` (omits the Job from the render entirely —
`templates/hooks/pre-install-check-legacy-policies.yaml`), Helm's own `--no-hooks` flag (skips every
hook on that invocation, chart-agnostic), and Flux's `HelmRelease.spec.install.disableHooks` /
`spec.upgrade.disableHooks` (same, for a Flux-managed release). Any of these means the preflight did
not actually run even though the install/upgrade itself was real — treat "Runs" above as the default,
not a guarantee, and run the manual check below if any of these are in play.

**For any path where neither column above runs** — plain `helm template`, render-then-apply
pipelines, the published `install.yaml` asset, or any real install/upgrade where one of the three
bypasses in the footnote is in play — run the same check the hook `Job` would have run, manually,
before upgrading:

```
kyverno check-legacy-policies --kubeconfig <path> [--context <name>]
```

If you installed the CLI as a kubectl plugin rather than a standalone binary, the same command is:

```
kubectl kyverno check-legacy-policies --kubeconfig <path> [--context <name>]
```

Both are the exact binary the hook `Job` runs (`cmd/cli/kubectl-kyverno/commands/legacypolicies`); it
is registered on the root `kyverno` command but marked `Hidden: true` (not shown in `--help`), since
it's meant to be invoked by the hook rather than discovered interactively — it still works when run
by name.

**A non-zero exit is not by itself proof that legacy policies were found** — `run()` returns any error
from listing the five kinds (a kubeconfig problem, an RBAC denial, an API timeout) exactly the same
way it returns the "found" case, and a missing CRD is treated as zero instances of that kind, not an
error. Check the actual message: only `"found %d legacy policy resource(s); migrate them to
policies.kyverno.io before continuing..."` means a real finding. Any other non-zero exit is an
indeterminate result — the check itself failed, not a clean bill of health — and needs investigating
before deciding whether it's safe to proceed. There's no machine-checkable equivalent of
`acknowledgeLegacyPoliciesNotEnforced` outside Helm's own values either way — a confirmed finding still
means the operator has to decide to proceed, same as setting that value would.

## Rollback limits

`scripts/verify-legacy-policy-migration.sh`'s scenario A5 (`helm rollback <release> <pre-1.20 revision>`)
is the only rollback path this project verifies, and it passes: CR counts and per-object spec hashes
across all five legacy kinds are unchanged by the upgrade/rollback round trip, CRDs and webhook rules
return to their pre-upgrade shape, enforcement resumes exactly as before 1.20 (a violating pod is
denied again), and the 1.20 write-time block itself is gone (a new legacy policy can be created again),
since 1.19's binaries never had it.

Known limits, not exercised by that verification:

- **Only a same-release `helm rollback` is covered.** Restoring a pre-1.20 state by other means (hand-
  applying an old manifest bundle, restoring a cluster/etcd backup) isn't tested and may behave
  differently depending on what else changed outside Kyverno's own resources in the interim.
- **CEL (`policies.kyverno.io`) objects created or evolved while on 1.20 aren't addressed.** A chart
  rollback also reverts any non-frozen CRD back to the shape it had in the rolled-back-to revision —
  but only because this chart templates its CRDs through the `charts/kyverno/charts/crds` subchart,
  and only when `crds.install=true`. With CRDs managed outside the chart, a rollback leaves them at
  their 1.20 shape.
  If a 1.20-only schema addition to a `policies.kyverno.io` kind was in use, rolling back can affect
  objects that depended on it. This is a general Helm-plus-CRD rollback property, not something
  specific to legacy-policy removal, and the legacy-migration verification script doesn't attempt to
  cover it.
