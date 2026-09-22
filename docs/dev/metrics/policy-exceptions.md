# PolicyException inventory metric

Proposed public metric contract for issue #17387, pending maintainer confirmation.

The admission controller exposes `kyverno_policy_exception_info`, an observable
gauge with value `1`, for existing PolicyException resources from `kyverno.io/v2`
and `policies.kyverno.io/v1beta1`.

| Label | Meaning |
| --- | --- |
| `exception_api_group` | `kyverno.io` or `policies.kyverno.io` |
| `exception_namespace` | Exception resource namespace |
| `exception_name` | Exception resource name |
| `policy_kind` | Declared CEL kind; legacy `Policy` for references containing `/`, otherwise `ClusterPolicy` |
| `policy_name` | Reference exactly as declared; legacy namespaced references may contain `namespace/name` |

One series is emitted for each unique five-label tuple, plus at most one series
with empty policy labels for an exception without declared references. Duplicate
references do not increase the value. Deleted resources and removed references
stop being observed after the informer cache receives the change.

This is resource inventory: expired CEL exceptions and references to nonexistent
policies are included. Exception enablement and the configured exception namespace
do not restrict collection. Metrics namespace include/exclude configuration applies
to the **exception namespace**. This metric does not measure effective exceptions,
exempted workloads, or bypass frequency.

Identity labels create cardinality proportional to exceptions and their distinct
policy references on each replica. No rule, workload, UID, annotation, or expression
labels are added. Keep all five labels when configuring metric label filtering;
dropping them merges distinct observations and loses inventory information.

Count distinct exceptions, deduplicating both references and replicas:

```promql
count(
  max by (exception_api_group, exception_namespace, exception_name) (
    kyverno_policy_exception_info
  )
) or vector(0)
```

Count distinct exception/policy-reference tuples across replicas:

```promql
count(
  max by (exception_api_group, exception_namespace, exception_name, policy_kind, policy_name) (
    kyverno_policy_exception_info
  )
) or vector(0)
```

Scope these queries to one cluster and one Kyverno installation using your
Prometheus target labels, or retain the cluster/installation labels in the grouping.
Otherwise identically named exceptions from different clusters are merged.
The zero fallback also produces zero when targets are unavailable or the metric
is disabled; use `up` and scrape-health alerts to distinguish those conditions.

Every admission-controller replica collects from both informer caches, outside
leader election. If a cache was not already needed on a replica, this adds a
LIST/WATCH and an in-memory copy of that API's exception resources. Scrapes only
read listers and never request Kubernetes resources directly. Unsynchronized
sources are skipped; failures in one source do not suppress the other source.
Allow for watch propagation and the Prometheus scrape interval when observing
changes. The metric can be disabled through the existing metrics configuration.

Proof manifests and an endpoint lifecycle check are in
`test/conformance/chainsaw/metrics/policy-exception-inventory`.
