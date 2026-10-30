# AGENTS.md — pkg/image

Image signature/attestation verification (cosign + notary). `pkg/cosign` and `pkg/notary` **no longer exist** — if
you find a reference to them (in old docs, comments, or CODEOWNERS), it's stale; the real path is
`pkg/image/verifiers/ivpol/{cosign,notary}`. The legacy `ClusterPolicy` (`cpol`) verifier under
`pkg/image/verifiers/cpol` was deleted in #17710 PR 4 along with the rest of the legacy engine; `ivpol` (CEL
`ImageValidatingPolicy`) is the only verifier left. No package doc comments exist in this tree except
`pkg/sigstoretuf` (see below).

## `pkg/sigstoretuf` is a hand-rolled mutex working around a real upstream data race

Its package doc explains why: the upstream `sigstore/pkg/tuf` singleton's metadata-refresh mutates internal maps
without holding its own initialization lock, while read paths do hold it — a genuine
`fatal error: concurrent map writes` (https://github.com/kyverno/kyverno/issues/15983). `pkg/sigstoretuf` adds its
own mutex around every touch point (`Initialize`, `TrustedRoot`, `RekorPublicKeys`, `CTLogPublicKeys`,
`FulcioRoots`), shared by the ivpol verifier plus CLI/controller bootstrap. **`WithLock`'s callback must never
call another `sigstoretuf` exported function inside it** — the mutex is not re-entrant; ivpol's
`initTUFAndFetch` documents this explicitly and calls `tuf`/`cosign` functions directly inside the closure instead.

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
