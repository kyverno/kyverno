# GlobalContextEntry storage migration

GlobalContextEntry uses `kyverno.io/v2` for runtime clients and storage.
The `v2beta1` and `v2alpha1` APIs remain served. Existing manifests can use
any of these versions. The deprecated `status.ready` field is retained for
round trips; readiness still comes from `status.conditions`.

## Upgrade

Install the updated CRD before switching the controllers. Changing its
storage version affects new writes, but does not rewrite existing objects.
The Helm post-upgrade migration job already includes GlobalContextEntry.
For a manual installation, run the matching CLI after updating the CRD:

```sh
kyverno migrate --resource globalcontextentries.kyverno.io
kubectl get crd globalcontextentries.kyverno.io \
  -o jsonpath='{.status.storedVersions}'
```

The migrator updates each object through the configured storage version
before narrowing `status.storedVersions`. If an object update or CRD status
update fails, correct the failure and rerun the command. Do not clear old
stored-version markers by hand. A GET through v2 does not prove that an
object has been rewritten in storage.

## Rollback

Keep all three schemas served. To reverse the storage migration, first set
`v2beta1` as the sole storage version while retaining the v2 schema, then
run the same migration command. Confirm that every object has been
rewritten and that `status.storedVersions` contains only `v2beta1` before
replacing CRD manifests. A Helm rollback alone does not perform these steps.

This change does not retire an API version. Any later retirement needs the
notice period and migration evidence in [API versioning](../../context/shared/api-versioning.md):
one minor release for alpha, two for beta and three for stable APIs, with
a release-note announcement and planned removal version.
Release scheduling and the minimum supported upgrade baseline require
maintainer agreement.
