# Workload image egress proof

The policy calls `getImageData` on Pod container, initContainer, and ephemeralContainer
image values. Metadata and loopback references must fail with
`connection blocked to network host`, while unmatched Pods remain admissible.
The ephemeral case updates the `ephemeralcontainers` subresource of an existing Pod.
The init/ephemeral cases also resolve a public pause image and need registry access.

Run against a cluster using this patch:

```sh
chainsaw test --test-dir test/conformance/chainsaw/image-validating-policies/standard/registry-egress-hard-block
```

Policy literals, signature repository overrides, and trust/key URLs remain native.
Generic CEL `image.getMetadata` calls are guarded only for known workload image values.
Focused Go
regressions exercise native policy access, resource preloads and verification caches,
credential ordering, and token/redirect destinations using local registry fixtures.
