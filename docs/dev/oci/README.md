# OCI policy bundles

A Kyverno CEL policy bundle is an OCI artifact that carries `policies.kyverno.io` `ValidatingPolicy`,
`MutatingPolicy`, `GeneratingPolicy`, `DeletingPolicy`, `ImageValidatingPolicy`, their `Namespaced*` variants, and
`PolicyException` resources, preserving their original file layout. `kyverno oci push` builds one from a directory
or a single file; `kyverno oci pull` and the `oci://` scheme accepted by `kyverno test` and `kyverno apply`
consume one; `kyverno oci validate` checks a directory, file, or pushed image against the same rules `push`
enforces, without pushing anything; `kyverno oci inspect` shows a pushed bundle's metadata and resource index;
`kyverno oci resolve` pins a tag to its digest. An optional `kyverno-bundle.yaml` at the bundle root names the
bundle and assigns resources to named sets; see [bundle-spec.md section 7](./bundle-spec.md#7-bundle-descriptor).

**Current format version: `1.0.0`.**

The normative specification is [bundle-spec.md](./bundle-spec.md): what bytes a writer produces, what a reader
must check, the media types, the config blob and index schema, and the compatibility rules across format
versions. If you're implementing a writer or a reader, start there.

For a CLI-oriented walkthrough of pushing and pulling, see the
[Kyverno CLI reference](https://kyverno.io/docs/kyverno-cli/#oci). [flux-example.md](./flux-example.md) is a
worked, digest-pinned example of consuming a bundle with Flux's `OCIRepository`. A user-facing GitOps guide
based on it is in progress on [kyverno.io](https://kyverno.io) as part of
[issue #17667](https://github.com/kyverno/kyverno/issues/17667).

This format is tracked by [epic #17645](https://github.com/kyverno/kyverno/issues/17645) and specified by
[issue #17660](https://github.com/kyverno/kyverno/issues/17660).
