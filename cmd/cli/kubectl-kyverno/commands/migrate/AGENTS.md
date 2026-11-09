# AGENTS.md — cmd/cli/kubectl-kyverno/commands/migrate

This package implements the `kyverno migrate` CLI command used in post-upgrade Helm hooks (`charts/kyverno/templates/hooks/post-upgrade-migrate-resources.yaml`) and by cluster operators to transition Custom Resource Definitions and existing custom resource instances to current storage versions in etcd.

## The Invariants

1. **Missing CRDs must never fail the migration hook**:
   When Kyverno is installed with `crds.install=false` (e.g., GitOps, Argo CD, Flux, Helmfile) or when optional CRD suites are omitted, `migrate` must treat absent CRDs (`apierrors.IsNotFound`) as non-fatal and skip them cleanly with an informational log. Similarly, if a CRD or resource disappears between lookup, listing, or status update, `NotFound` is treated as a clean skip and does not fail the post-upgrade hook Job.

2. **Legacy policy kinds are skipped and never updated**:
   In Kyverno 1.20+, execution support for legacy policy kinds (`ClusterPolicy`, `Policy`, `CleanupPolicy`, `ClusterCleanupPolicy`, and `PolicyException` in `kyverno.io`) is removed, and admission writes to these kinds are rejected by admission webhooks. Attempting to update their stored version in-place would fail admission. `migrate` uses `deprecations.IsLegacyPolicyKind` to detect these legacy kinds and skips updating their instances or claiming single-storage status.

3. **status.storedVersions must remain unchanged if migration is incomplete**:
   Per Kubernetes CRD storage version requirements, `status.storedVersions` tracks all versions that may still exist in etcd. If an object update fails with an unexpected error, `migrate` returns the error and leaves `status.storedVersions` untouched. Only when all existing instances have been rewritten (or were deleted in-flight) is `status.storedVersions` pruned to `[storedVersion.Name]`.

4. **Status updates must retry on conflict and verify CRD identity**:
   Updating CRD status can encounter optimistic concurrency conflicts if controllers or operators modify the CRD while resources are being iterated. All status writes must be wrapped in `retry.RetryOnConflict`. Before updating `status.storedVersions`, the command verifies that the refetched CRD's UID and generation match the initial state. If the generation or UID changed, migration aborts with an error directing operators to rerun migration so un-migrated versions are not prematurely pruned.
