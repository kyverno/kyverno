# Registry egress hard blocks

This test verifies that image signature verification rejects cloud metadata and
loopback registry addresses in the default audit mode. It also runs unchanged
with `privateRegistryEgressMode=enforce`. The assertion requires the egress guard's
specific error, so authentication failures, timeouts, and connection refusals
cannot make the test pass.

The policy reuses the public key from the keyed verification tests. Transparency
log and SCT checks are disabled because this proof only exercises registry
network destinations. No signing key, registry, cloud endpoint, or pull secret is
needed. The Pod selector limits the policy to the proof resources. The init
container fixture verifies an init image is guarded independently of the regular
container. Unit coverage in `pkg/registryclient/scope_test.go` also extracts and
blocks regular, init, and ephemeral container images in both modes.

Policy-authored image registry context, signature repository, key, certificate,
Rekor, Fulcio, CT, TSA, TUF, and `validate.manifests` endpoints are outside this
guard, even in enforce mode, and retain upstream native behavior. Workload-image
Sigstore bundle requests remain guarded. Positive reachability regressions for
policy-selected context and signature repositories, public keys, Rekor, and
manifest bundles use
local HTTP test servers rather than blocked-endpoint proof fixtures.

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
kyverno apply policy.yaml --resource init-pod.yaml --registry
```

All commands must report a failed verification. Unit tests in
`pkg/utils/egress` separately prove that blocked destinations receive no dial or
HTTP request, including redirects and attempts to reuse existing connections.
