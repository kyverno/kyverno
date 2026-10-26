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
`PolicyReport`/`ClusterPolicyReport` is re-queued once, which prunes any legacy-policy-sourced result
from it during the normal reconcile (results are only ever rebuilt from current `EphemeralReport`s,
and nothing produces one for a legacy policy anymore). If no legacy policy objects exist at startup,
the sweep is skipped entirely as unnecessary work. The check fails closed: if the presence check
itself errors, the sweep is skipped for that start rather than risking a cluster-wide read burst
against an already-unhealthy API server, and the next leader election retries it.

Practically: after upgrading to 1.20, existing reports clear their legacy-policy results on the first
reports-controller restart that sees the upgrade, with no manual cleanup step. A report that never
referenced a legacy policy is untouched.

## Non-Helm / GitOps preflight

The automatic "don't let an operator silently lose enforcement" gate is two Helm-specific mechanisms,
both in `charts/kyverno/templates/`:

- `validate-legacy-policies.yaml` — a chart-render-time `lookup` against the live cluster (requires
  `helm template`/`install`/`upgrade` to be run with real cluster access) that `fail`s the render if
  any of the five legacy kinds still have instances, unless
  `upgrade.acknowledgeLegacyPoliciesNotEnforced=true` is set.
- `templates/hooks/pre-install-check-legacy-policies.yaml` — a `pre-install,pre-upgrade` Helm hook
  `Job` running `kyverno check-legacy-policies` (the same check, as a cluster-side binary instead of
  Helm template logic) for installers that skip client-side `lookup`. Suppressed when
  `global.templating.enabled=true`, which is how this repo's own `config/install.yaml`/
  `config/install-latest-testing.yaml` are generated — there is no live cluster to look up against
  while building a static manifest for publication, and no hook execution model for a plain manifest
  either.

**Neither mechanism runs for an install or upgrade that doesn't invoke live Helm templating or hook
execution against the target cluster.** Concretely, this gate does not protect:

- Applying the published static manifest (`config/install.yaml` or `config/install-latest-testing.yaml`)
  directly, e.g. `kubectl apply -f`.
- A GitOps pipeline that renders the chart once (`helm template`) and commits or applies the resulting
  plain YAML — common when a team wants Kustomize post-processing, or wants `kubectl diff`/drift
  detection on plain manifests instead of relying on Helm's own diffing.

It does run under GitOps controllers that genuinely invoke Helm against the live cluster at sync time
(Flux's `helm-controller`, ArgoCD's native Helm source type), since those execute the same `lookup`
and hook-equivalent logic Helm itself would.

**For any of the unprotected paths**, run the same check Helm would have run, before upgrading:

```
kyverno check-legacy-policies --kubeconfig <path> [--context <name>]
```

This is the exact binary the Helm hook `Job` runs (`cmd/cli/kubectl-kyverno/commands/legacypolicies`);
it is registered on the `kyverno` CLI but marked `Hidden: true` (not shown in `--help`), since it's
meant to be invoked by the hook rather than discovered interactively — it still works when run by
name. A non-zero exit means legacy policies were found and are about to lose enforcement; there's no
machine-checkable equivalent of `acknowledgeLegacyPoliciesNotEnforced` outside Helm's own values —
the operator has to decide to proceed anyway, same as setting that value would mean.

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
