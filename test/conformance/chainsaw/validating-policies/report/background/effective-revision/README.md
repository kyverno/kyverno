# Background CEL policy revision

Run against a test cluster with this branch deployed and background reporting
enabled. The background scan interval must exceed the ten-second observation
window; the default interval is sufficient.

```sh
chainsaw test --test-dir test/conformance/chainsaw/validating-policies/report/background/effective-revision
```

The test creates a ConfigMap before an admission-disabled ValidatingPolicy and
waits for its background finding. After the aggregate controller consumes the
initial ephemeral report, it records the list resourceVersion and applies an
unrelated annotation. A watch from that revision detects even quickly created
and deleted background reports for the target. No report events should occur
during the observation window. Finally, a validation-expression change must
replace the fail result with one pass result.

Controller unit tests cover status-only updates, legacy-label migration,
namespaced and cluster report persistence, exception edits and removal, and
scanner failures after partial progress. This conformance test covers the API
event path; it is not a cluster-scale performance benchmark.
