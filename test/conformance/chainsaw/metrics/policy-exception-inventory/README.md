# PolicyException inventory proof

Run against a disposable cluster with the modified admission controller, metrics
on port 8000, and both exception CRDs. Enable `features.policyExceptions.enabled`
and set its namespace to `"*"`. For Kyverno 1.20, set
`features.blockLegacyPolicyAPIs.enabled=false` to allow creating legacy fixtures.
The runner needs `kubectl` and permission to proxy admission-controller pods.
Set `KYVERNO_NAMESPACE` if Kyverno is installed outside `kyverno`.

```sh
.tools/chainsaw test --test-dir test/conformance/chainsaw/metrics/policy-exception-inventory
```

This verifies gauge type, exact identity/reference labels and value, duplicate
reference collapse, reference replacement, deletion, and preservation of an
unrelated exception through the actual `/metrics` endpoint on every replica.
No referenced policies exist. Use two admission replicas to cover nonleaders.
Empty references and expired resources are covered by unit tests because
admission validation can reject such fixtures.
