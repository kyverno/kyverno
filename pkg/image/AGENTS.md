# AGENTS.md — pkg/image

Image signature/attestation verification (cosign + notary). `pkg/cosign` and `pkg/notary` **no longer exist** — if
you find a reference to them (in old docs, comments, or CODEOWNERS), it's stale; the real paths are
`pkg/image/verifiers/{cpol,ivpol}/{cosign,notary}`. Shared trust helpers live in
`pkg/sigstoreguard` and `pkg/sigstoretuf` (see below).

## cpol and ivpol verification are genuinely duplicated, not layered over a shared core

Legacy `ClusterPolicy` (`cpol`) and CEL `ImageValidatingPolicy` (`ivpol`) each have their own independent cosign
verifier, both wrapping the same underlying `sigstore/cosign` library but building their own `cosign.CheckOpts` from
scratch — there is no shared "core verifier" one delegates to. Same for notary. **The practical consequence**: new
verification features tend to land in cpol first and ivpol catches up later (`a425f4f37 fix: close image
verification parity gaps`, `f0c5d57c5 fix: IVPOL required has no effect`, `17fbbf48b fix: IVPOL mutateDigest
unimplemented stub`). If you're adding a verification feature, check whether it needs implementing in **both**
places, and expect ivpol to be the one that's behind.

## `pkg/sigstoreguard` owns guarded trust loading and shared cache synchronization

TUF access uses `sigstore-go` with Kyverno's guarded HTTP client. `pkg/sigstoreguard` serializes access to the
shared operator cache and isolates policy-supplied repositories so they cannot replace the controller's default
trust configuration. `pkg/sigstoretuf` preserves the existing `Initialize`, `TrustedRoot`, `RekorPublicKeys`,
`CTLogPublicKeys`, and `FulcioRoots` helper API by delegating to this implementation; it no longer wraps the
legacy Sigstore singleton. IVPOL's `initTUFAndFetch` obtains its trust material together through
`sigstoreguard.VerificationMaterialFor`. **A `WithLock` callback must not call another helper that acquires the
same non-reentrant cache lock.** IVPOL's Rekor and HTTP key/root loaders also use `sigstoreguard`; legacy
verifiers retain their existing non-TUF HTTP paths in this CEL change.

## Verification fails closed, deliberately, with one narrow exception

If TUF initialization or trusted-root fetch fails, verification errors out — there is no "skip verification and
allow" fallback anywhere in this path. Rekor/CTLog pubkey and Fulcio-root fetch failures fall back only to
already-fetched trusted-root data, never to skipping verification; if that also fails, the whole verification
errors. The **only** exception is `skipSigstoreInfra`: a key/certificate attestor with `CTLog.InsecureIgnoreTlog`
set explicitly skips TUF by the policy author's own declared config — not an error-handling fallback. Keep this
fail-closed property if you touch this code; it's a deliberate security posture, not an oversight.

## Image-verify cache is TTL + resourceVersion keyed, not explicitly invalidated

`pkg/image/verification/cache` (ristretto-backed, 1h default TTL, `DefaultMaxSize` = 10 MiB as the default `MaxCost`
byte budget, tunable via `--imageVerifyCacheMaxSize`): the cache key is
`policy UID + policy resourceVersion + rule name + image ref` — so **any edit to the policy object automatically
invalidates its cached verification results** without any explicit invalidation logic. Entries cost against that
budget by size: presence-only entries cost `1`, entries carrying attestation payload bytes cost roughly their real
size, and ristretto adds its own per-item cost (about 56 bytes) to both, so the number of entries that fit varies
with payload size. `NumCounters` is sized from an expected ~1 KiB entry, not from the byte budget, because ristretto
allocates its counters up front. ristretto can drop a write after `SetWithTTL` returned true, so `SetWithPayload`
reports whether the entry is actually in the cache afterwards.
The CLI
(`kubectl-kyverno apply`/`test`) and one IVPOL dry-run path explicitly use a disabled (no-op) cache — don't assume
CLI-observed caching behavior matches the admission-controller's long-lived cache.

## Recent test worth reading before touching cert-chain validation

`ivpol/cosign/cert_limit_test.go` tests the CVE-2026-32280 mitigation (a max-intermediate-certs limit) by building
oversized certificate chains and asserting rejection — read it before changing certificate-chain handling in the
ivpol cosign verifier.
