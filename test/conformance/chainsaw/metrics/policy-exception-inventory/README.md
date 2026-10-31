# PolicyException inventory proof

Install the modified admission controller with exactly two replicas, policy
exceptions enabled, and the exception namespace set to `"*"`. The runner needs
`kubectl` and permission to proxy admission-controller pods. Set
`KYVERNO_NAMESPACE` if Kyverno is installed outside `kyverno`.

```sh
.tools/chainsaw test --test-dir test/conformance/chainsaw/metrics/policy-exception-inventory
bash test/conformance/chainsaw/metrics/policy-exception-inventory/check_test.sh
```

The lifecycle proof uses CEL exceptions. It verifies gauge type, exact
identity/reference labels and value, duplicate reference collapse, reference
replacement, deletion, and preservation of an unrelated exception through the
actual `/metrics` endpoint on both replicas, including the nonleader. Missing or
extra replicas cause retries and eventually failure. The shell regression tests
cover replica counts, API failures, stale metrics, and retry exhaustion without a
cluster. No referenced policies exist.

Kyverno 1.20 denies creating or editing legacy exceptions. To test their startup
inventory, install the legacy PolicyException CRD and create legacy resources
**before** installing Kyverno, then verify their `exception_api_group="kyverno.io"`
series on both admission replicas. Delete a resource and verify its series goes
away while the other resources remain. This does not require bypassing the legacy
admission denial. For example, from the repository root before installing Kyverno:

```sh
kubectl apply -f config/crds/kyverno/kyverno.io_policyexceptions.yaml
kubectl create namespace legacy-inventory-proof
kubectl -n legacy-inventory-proof apply -f test/conformance/chainsaw/metrics/policy-exception-inventory/legacy-exceptions.yaml
```

Install the modified controller with the settings above and
`upgrade.acknowledgeLegacyPoliciesNotEnforced=true`, then run:

```sh
bash test/conformance/chainsaw/metrics/policy-exception-inventory/check.sh legacy-created legacy-inventory-proof
kubectl -n legacy-inventory-proof delete policyexceptions.kyverno.io legacy-inventory
bash test/conformance/chainsaw/metrics/policy-exception-inventory/check.sh legacy-deleted legacy-inventory-proof
```

Unit tests cover both API caches, empty references, expired
resources, optional APIs, and independent source failures.
