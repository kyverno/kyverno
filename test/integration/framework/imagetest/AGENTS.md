# AGENTS.md — test/integration/framework/imagetest

Shared in-process OCI 1.1 registry and ephemeral virtual Sigstore helpers for offline image verification tests.

## Package constraints

- **No `//go:build integration` tag.** Unit tests under `pkg/image/verifiers/ivpol/cosign` run without
  `-tags=integration` and import this package directly alongside `test/integration/ivpol`. Adding an
  `integration` build tag here will break `make test-unit`.
- **No `github.com/kyverno/kyverno/...` imports.** Keep this a leaf package depending only on Go stdlib and
  external OCI/Sigstore/test libraries. Importing any Kyverno `pkg/` package creates an import cycle with
  `pkg/image/verifiers/ivpol/cosign` tests.
- **Lives under `test/integration/framework/`.** `check-framework.yaml` skips `framework` when discovering
  `test/integration/*/` envtest packages, so this package is not misclassified as an envtest suite, while
  changes to it still trigger `check-framework-registry.yaml`.

## Hermetic trust contract

- `NewRegistry` serves an in-process OCI 1.1 registry (`WithReferrersSupport(true)`) on loopback HTTP and
  registers `t.Cleanup(srv.Close)`. `BlobGets` counts `GET .../blobs/sha256:` requests so tests can assert
  bounded registry reads.
- `NewVirtualSigstore` wraps `sigstore-go/pkg/testing/ca.NewVirtualSigstore()` (in-memory Fulcio CA + RFC 3161
  TSA). `TrustedRootJSON` and `ComposeTrustedRootJSON` intentionally omit Rekor and CTLog entries so callers
  verify offline with `insecureIgnoreTlog: true` and `insecureIgnoreSCT: true` without TUF egress.
- `SignKeylessBundle` produces a Sigstore v0.3 bundle (DSSE envelope + Fulcio leaf cert + RFC 3161 timestamp)
  and `SignLegacyKeyless` attaches a legacy `.sig` tag signature.
- Every helper accepting `*testing.T` must call `t.Helper()`.
