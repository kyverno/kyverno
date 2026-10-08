# CEL image metadata egress proof

The policy obtains image metadata using `image.GetMetadata`. Pods referencing
cloud metadata and loopback addresses must be denied with
`connection blocked to network host`, before a credential helper or
network connection is attempted. An unmatched Pod remains admissible.

On a cluster running this patch:

```sh
chainsaw test --test-dir test/conformance/chainsaw/validating-policies/context/registry-egress-hard-block
```

Standalone CLI ValidatingPolicy evaluation uses supplied image context data.
Use the [ImageValidatingPolicy CLI proof](../../../image-validating-policies/standard/registry-egress-hard-block/README.md)
to exercise actual registry requests without a cluster. Unit tests also verify
private-address audit compatibility, explicit private allowlists, redirects,
proxy handling, and Sigstore key, Rekor, TUF, and signature-source requests.
