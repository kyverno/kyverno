# AGENTS.md — pkg/utils/kube

Small Kubernetes helpers shared across binaries. Most files here are unremarkable; `secret.go` and
`secret_client.go` are not, because they sit on an authorization boundary. Read this before changing
either of them.

## Secret access is cache-only by default; the live GET is opt-in and namespace-confined

Kyverno is installed with RBAC for Secrets in its own namespace plus any namespace named in
`--imagePullSecrets`. It deliberately does **not** hold cluster-wide Secret read access. The
registry-credential lister built in `cmd/internal/registry.go` is a `multiLister`
(`cmd/internal/lister.go`): one informer per configured namespace, and `emptySecretNamespaceLister`
— which returns `NotFound` — for every other namespace. That NotFound is the designed answer, not a
cache miss to be papered over.

`WithSecretClient` wraps that lister so namespaced ImageValidatingPolicies can read registry
credentials from their **own** namespace, which has no informer. The invariants that make this safe:

- **`WithSecretClient` does not change default behavior.** `secretClientLister` embeds the cached
  lister and adds only `ForNamespace`/`ForNamespaceWithContext`. It does **not** override `Secrets()`,
  so an ordinary `lister.Secrets(ns).Get(name)` still resolves through `multiLister` and still returns
  `NotFound` for unconfigured namespaces. The legacy image-verification engine, the shared CEL context
  provider and resource `imagePullSecrets` keep their cache-only behavior precisely because they never
  call `ForNamespace`. **Do not promote the live GET onto `Secrets()`.**
- **The live GET is reachable only through `ScopeSecretLister`/`ScopeSecretListerWithContext`.** Both
  return the lister untouched when the namespace is empty, so cluster-scoped `ImageValidatingPolicy`
  is unaffected. The only sanctioned call sites are the namespaced-ivpol credential paths —
  `pkg/image/verification/evaluator/compiler.go` and `pkg/cel/libs/imageverify/impl.go`. Adding a
  caller elsewhere widens Secret access; treat it as a design change, not a refactor.
- **The scope is enforced, not advisory.** `scopedSecretNamespaceLister` returns a `Forbidden` error
  for any namespace other than the policy's own, including Kyverno's installation namespace. Forbidden
  must stay authoritative — never fall back to the cached lister or to another namespace when a scoped
  read fails. Only `NotFound` from the cache escalates to a live GET.
- **The live GET stays bounded.** It runs under the caller's context with a 5s timeout, and
  `GetWithContext` checks `ctx.Err()` first so a cancelled admission request does not issue an API
  call. Keep both; `Get` exists only so non-context-aware callers still get the bounded request.

## `ScopeSecretReferences` is validation, not normalization

`ScopeSecretReferences` (`secret.go`) rejects a reference to any namespace other than the policy's,
rather than silently rewriting it — a policy author who typed another namespace gets an error naming
the field, reference and namespace. It also rejects an empty policy namespace outright, so it can
never be called in a way that degrades into "match anything". `ParseSecretReference` defaults a bare
name to the namespace the caller supplies, which is why the caller must pass the *policy* namespace
and not `config.KyvernoNamespace()` on namespaced paths.

Callers must run these checks before compilation and again before runtime dynamic-attestor
verification, without mutating the cached policy object — compiled policies are immutable and shared
across admission requests.
