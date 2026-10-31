# Registry egress hard-block proof

Proves that unsafe registry destinations are rejected wherever the image reference comes from.

`policy.yaml` calls `getImageData` on Pod container, initContainer, and ephemeralContainer
values. Metadata and loopback references must fail with `connection blocked to network host`,
while Pods that do not match the policy stay admissible. The ephemeral case patches the
`ephemeralcontainers` subresource of an existing Pod. The init and ephemeral cases also resolve a
public pause image, so the cluster needs registry access.

`literal-policy.yaml` covers the complementary case: the blocked reference is written into the
policy rather than read from the Pod. Because the destination is validated where the image is
fetched, it is rejected on the same terms. This is the behavior that distinguishes the fetch-site
guard from an approach based on extracting workload images — there is no reference provenance to
get wrong.

Hard blocks apply in both `audit` and `enforce` mode and cannot be lifted by an allowlist entry,
so no egress configuration is needed to run this.

```sh
chainsaw test --test-dir test/conformance/chainsaw/image-validating-policies/standard/registry-egress-hard-block
```

Sigstore (Rekor, Fulcio, TSA, TUF) and KMS endpoints do not use the registry client and are out of
scope here. Unit coverage for the destination policy, pinned dialing, redirect and token handling,
and connection pooling lives in `pkg/utils/egress`.
