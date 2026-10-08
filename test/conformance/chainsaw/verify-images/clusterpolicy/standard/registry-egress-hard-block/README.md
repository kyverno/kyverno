# Registry egress hard blocks

This test verifies that image signature verification rejects cloud metadata and
loopback registry addresses in the default audit mode. It also runs unchanged
with `privateRegistryEgressMode=enforce`. The assertion requires the egress guard's
specific error, so authentication failures, timeouts, and connection refusals
cannot make the test pass.

The policy reuses the public key from the keyed verification tests. Transparency
log and SCT checks are disabled because this proof only exercises registry
network destinations. No signing key, registry, cloud endpoint, or pull secret is
needed. The Pod selector limits the policy to the two proof resources.

Run against a cluster with the changed Kyverno controller installed. Helm passes
the registry egress settings to admission, background, and reports controllers;
cleanup does not use these flags. The [configuration guide](../../../../../../../docs/user/legacy-registry-egress.md#reproducing-the-chart-configuration)
also provides a local regression check of rendered arguments against the actual
controller executables.

```sh
chainsaw test --test-dir test/conformance/chainsaw/verify-images/clusterpolicy/standard/registry-egress-hard-block
```

The same manifests can be checked locally with the changed Kyverno CLI. From
this directory, run each command and verify the output contains
`connection blocked to network host` and the corresponding registry address:

```sh
kyverno apply policy.yaml --resource metadata-pod.yaml --registry
kyverno apply policy.yaml --resource loopback-pod.yaml --registry
```

Both commands must report a failed verification. Unit tests in
`pkg/utils/egress` separately prove that blocked destinations receive no dial or
HTTP request, including redirects and attempts to reuse existing connections.
