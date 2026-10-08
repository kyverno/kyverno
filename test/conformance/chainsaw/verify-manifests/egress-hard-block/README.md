# Manifest verification egress hard blocks

This test exercises the real `ClusterPolicy` `validate.manifests` admission path.
It checks that the default audit mode rejects:

- A public-key URL on loopback (`127.0.0.1`).
- A public-key URL on cloud metadata (`169.254.169.254`).
- A loopback image bundle selected by the resource's
  `cosign.sigstore.dev/resourceBundleRef` annotation.
- A metadata image bundle selected by the policy's `manifests.repository`.

The same test must pass with `privateRegistryEgressMode=enforce`. An allowlist
cannot bypass these hard blocks. Each denial must contain the matching rule name,
the egress guard's `connection blocked to network host` error, and the requested
host. An unrelated matching, signature, authentication, timeout, or connection
error cannot satisfy the assertion. No endpoint, registry, or signing key is needed.

Each Service copies the message and signature from
[`single-signature/resource-one-signature.yaml`](../single-signature/resource-one-signature.yaml)
without changing either value. The `app.kubernetes.io/instance` label selects one
proof rule and is already excluded from signed-manifest comparison by the default
configuration. The annotation-selected bundle case adds only the bundle reference.
All resources keep the original signed name, `test-service2`; every creation must
be denied, so they do not conflict with one another.

Run against a cluster with the changed Kyverno controller installed:

```sh
chainsaw test --test-dir test/conformance/chainsaw/verify-manifests/egress-hard-block
```

With a CLI built from the changed code, run from this directory:

```sh
kyverno apply policy.yaml --resource key-loopback-service.yaml --registry --table --detailed-results
kyverno apply policy.yaml --resource key-metadata-service.yaml --registry --table --detailed-results
kyverno apply policy.yaml --resource bundle-loopback-service.yaml --registry --table --detailed-results
kyverno apply policy.yaml --resource bundle-metadata-service.yaml --registry --table --detailed-results
```

Each command must exit nonzero and show `connection blocked to network host`
and the corresponding address in the detailed results. The table flags expose
manifest verification errors that the default CLI summary does not print. Unit tests in
`pkg/engine/handlers/validation/manifest` separately check valid and tampered signed
resources, multiple signatures, and zero HTTP requests to blocked local servers.
See the [egress configuration guide](../../../../../docs/user/legacy-registry-egress.md)
for controller settings and proxy behavior.
