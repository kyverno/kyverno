# AGENTS.md — pkg/image

Image signature/attestation verification (cosign + notary). `pkg/cosign` and `pkg/notary` **no longer exist** — if
you find a reference to them (in old docs, comments, or CODEOWNERS), it's stale; the real paths are
`pkg/image/verifiers/{cpol,ivpol}/{cosign,notary}`. No package doc comments exist in this tree except
`pkg/sigstoretuf` (see below).

## cpol and ivpol verification are genuinely duplicated, not layered over a shared core

Legacy `ClusterPolicy` (`cpol`) and CEL `ImageValidatingPolicy` (`ivpol`) each have their own independent cosign
verifier, both wrapping the same underlying `sigstore/cosign` library but building their own `cosign.CheckOpts` from
scratch — there is no shared "core verifier" one delegates to. Same for notary. **The practical consequence**: new
verification features tend to land in cpol first and ivpol catches up later (`a425f4f37 fix: close image
verification parity gaps`, `f0c5d57c5 fix: IVPOL required has no effect`, `17fbbf48b fix: IVPOL mutateDigest
unimplemented stub`). If you're adding a verification feature, check whether it needs implementing in **both**
places, and expect ivpol to be the one that's behind.

## `pkg/sigstoretuf` is a hand-rolled mutex working around a real upstream data race

Its package doc explains why: the upstream `sigstore/pkg/tuf` singleton's metadata-refresh mutates internal maps
without holding its own initialization lock, while read paths do hold it — a genuine
`fatal error: concurrent map writes` (https://github.com/kyverno/kyverno/issues/15983). `pkg/sigstoretuf` adds its
own mutex around every touch point (`Initialize`, `TrustedRoot`, `RekorPublicKeys`, `CTLogPublicKeys`,
`FulcioRoots`), shared by **both** cpol and ivpol verifiers plus CLI/controller bootstrap. **`WithLock`'s callback
must never call another `sigstoretuf` exported function inside it** — the mutex is not re-entrant; ivpol's
`initTUFAndFetch` documents this explicitly and calls `tuf`/`cosign` functions directly inside the closure instead.

## Verification fails closed, deliberately, with one narrow exception

If TUF initialization or trusted-root fetch fails, verification errors out — there is no "skip verification and
allow" fallback anywhere in this path. Rekor/CTLog pubkey and Fulcio-root fetch failures fall back only to
already-fetched trusted-root data, never to skipping verification; if that also fails, the whole verification
errors. The **only** exception is `skipSigstoreInfra`: a key/certificate attestor with `CTLog.InsecureIgnoreTlog`
set explicitly skips TUF by the policy author's own declared config — not an error-handling fallback. Keep this
fail-closed property if you touch this code; it's a deliberate security posture, not an oversight.

## Image-verify cache is TTL + resourceVersion keyed, not explicitly invalidated

`pkg/image/verification/cache` (ristretto-backed, 1h default TTL, 1000 default max entries): the cache key is
`policy UID + policy resourceVersion + rule name + image ref` — so **any edit to the policy object automatically
invalidates its cached verification results** without any explicit invalidation logic. Entries with cached
attestation payload bytes cost their real size against the max-cost bound; presence-only entries cost `1`. The CLI
(`kubectl-kyverno apply`/`test`) and one IVPOL dry-run path explicitly use a disabled (no-op) cache — don't assume
CLI-observed caching behavior matches the admission-controller's long-lived cache.

## Recent test worth reading before touching cert-chain validation

`ivpol/cosign/cert_limit_test.go` tests the CVE-2026-32280 mitigation (a max-intermediate-certs limit) by building
oversized certificate chains and asserting rejection — read it before changing certificate-chain handling in the
ivpol cosign verifier.
