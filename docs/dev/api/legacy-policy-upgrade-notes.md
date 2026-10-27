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
`MergeReports` alongside the current `EphemeralReport`s, and `MergeReports` only keeps a legacy
(`SourceKyverno`) result if it finds the originating policy in a live policy map — which, now that
legacy execution is gone, is never populated, so every `SourceKyverno` result is dropped on that pass
regardless of which report it came from. If no legacy policy objects exist at startup, the sweep
itself is skipped as unnecessary work — but note this means a report that already held stale
`SourceKyverno` results *before* the legacy policy that produced them was deleted won't get swept by
this path; it's only cleared once something else re-queues that specific report (e.g. a write to the
resource it covers). The presence check fails closed: if it errors, the sweep is skipped for that
start rather than risking a cluster-wide read burst against an already-unhealthy API server, and the
next leader election retries it.

This covers both report backends: when OpenReports is enabled (`orClient != nil`), the permanent
objects being read, merged, and written back are `Report`/`ClusterReport`, not
`PolicyReport`/`ClusterPolicyReport` — same mechanism, different CRD kind.

Practically: if legacy policies are still present when a cluster restarts into 1.20, every report
gets its legacy-sourced results pruned on that one pass, with no manual cleanup step. If every legacy
policy was already deleted *before* that restart, the startup sweep has nothing to trigger it, and any
report still carrying stale results from before the deletion keeps them until it's next reconciled for
an unrelated reason.

## Non-Helm / GitOps preflight

The automatic "don't let an operator silently lose enforcement" gate is two independent mechanisms,
both in `charts/kyverno/templates/`, that run on different conditions:

- `validate-legacy-policies.yaml` — a chart-render-time `lookup` against the live cluster that `fail`s
  the render if any of the five legacy kinds still have instances, unless
  `upgrade.acknowledgeLegacyPoliciesNotEnforced=true` is set. **`lookup` is not populated by an
  ordinary `helm template`, even one run with full cluster access** — confirmed directly (a minimal
  test chart's `lookup` call returns empty under plain `helm template --kube-context <ctx>`, and under
  `helm install --dry-run`/`--dry-run=client`, but returns real data under `helm template
  --dry-run=server` (the current name for the deprecated `--validate` flag) and under a real `helm
  install`/`helm upgrade`). This is Helm's own behavior, not specific to this chart.
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
| Plain `helm template` (even with `--kube-context` set, or `--dry-run=server`), or any pipeline that renders the chart once and applies/commits the resulting plain YAML | **Does not run** | **Does not run** — no hook execution model outside a real Helm install/upgrade |
| The published release `install.yaml` asset, or `config/install-latest-testing.yaml` | **Does not run** | **Does not run** |

¹ "Runs" here means the chart renders the Job and Helm executes a real install/upgrade — not that
nothing can skip it. Three independent, generic bypasses still apply regardless of install method:
this chart's own `upgrade.legacyPolicyCheck.enabled=false` (omits the Job from the render entirely —
`templates/hooks/pre-install-check-legacy-policies.yaml`), Helm's own `--no-hooks` flag (skips every
hook on that invocation, chart-agnostic), and Flux's `HelmRelease.spec.install.disableHooks` /
`spec.upgrade.disableHooks` (same, for a Flux-managed release). Any of these means the preflight did
not actually run even though the install/upgrade itself was real — treat "Runs" above as the default,
not a guarantee, and run the manual check below if any of these are in play.

**For any of the paths marked "does not run" above**, run the same check the hook `Job` would have run,
manually, before upgrading:

```
kyverno check-legacy-policies --kubeconfig <path> [--context <name>]
```

This is the exact binary the hook `Job` runs (`cmd/cli/kubectl-kyverno/commands/legacypolicies`); it is
registered on the `kyverno` CLI but marked `Hidden: true` (not shown in `--help`), since it's meant to
be invoked by the hook rather than discovered interactively — it still works when run by name.

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
  rollback also reverts any non-frozen CRD back to the shape it had in the rolled-back-to revision.
  If a 1.20-only schema addition to a `policies.kyverno.io` kind was in use, rolling back can affect
  objects that depended on it. This is a general Helm-plus-CRD rollback property, not something
  specific to legacy-policy removal, and the legacy-migration verification script doesn't attempt to
  cover it.
