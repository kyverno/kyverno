# Registry egress denylist proof

Proves that denied registry destinations are rejected wherever the image reference comes from.

`policy.yaml` calls `getImageData` on Pod container, initContainer, and ephemeralContainer
values. Metadata and loopback references must fail with `connection blocked to network host`,
while Pods that do not match the policy stay admissible. The ephemeral case patches the
`ephemeralcontainers` subresource of an existing Pod. The init and ephemeral cases also resolve a
public pause image, so the cluster needs registry access.

`literal-policy.yaml` covers the complementary case: the denied reference is written into the
policy rather than read from the Pod. Because the destination is validated where the image is
fetched, it is rejected on the same terms. This is the behavior that distinguishes the fetch-site
guard from an approach based on extracting workload images — there is no reference provenance to
get wrong.

The destinations used here, `169.254.169.254` and `127.0.0.1`, are covered by the shipped
`registryEgressDeniedIPs` list (`169.254.0.0/16` and `127.0.0.0/8`), so the test needs no denylist
configuration of its own. Nothing in that list is privileged: it is an ordinary chart value, and an
operator who removes those entries makes these same references reachable. The first step does
switch the admission controller to `--privateRegistryEgressMode=enforce`, restoring its original
arguments on cleanup, because the shipped mode is `audit` — which logs a denied destination and
connects anyway rather than rejecting it.

```sh
chainsaw test --test-dir test/conformance/chainsaw/image-validating-policies/standard/registry-egress-denylist
```

Sigstore (Rekor, Fulcio, TSA, TUF) and KMS endpoints do not use the registry client and are out of
scope here. Unit coverage for the destination policy, pinned dialing, redirect and token handling,
and connection pooling lives in `pkg/utils/egress`.
