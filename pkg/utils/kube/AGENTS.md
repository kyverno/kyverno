# AGENTS.md — pkg/utils/kube

Small Kubernetes helpers shared across binaries. Most files here are unremarkable; `secret.go` and
`secret_client.go` are not, because they sit on an authorization boundary. Read this before changing
either of them.

## Secret access is cache-only by default; the live GET is opt-in and namespace-confined

Kyverno holds RBAC for Secrets in its own namespace — the chart creates a namespaced `Role` in the
release namespace only (`charts/kyverno/templates/admission-controller/role.yaml`,
`charts/kyverno/templates/reports-controller/role.yaml`). Any extra namespace named in
`--imagePullSecrets` needs a Role the operator supplies themselves; the chart does not generate one,
and `cmd/internal/registry.go` treats a cache-sync failure there as fatal. Kyverno deliberately does
**not** hold cluster-wide Secret read access. The registry-credential lister built in
`cmd/internal/registry.go` is a `multiLister` (`cmd/internal/lister.go`): one informer per configured
namespace, and `emptySecretNamespaceLister` — which returns `NotFound` — for every other namespace.
That NotFound is the designed answer, not a cache miss to be papered over.

`WithSecretClient` wraps that lister so namespaced ImageValidatingPolicies can read registry
credentials from their **own** namespace, which has no informer. The invariants that make this safe:

- **`WithSecretClient` does not change default behavior.** `secretClientLister` embeds the cached
  lister and adds only `ForNamespace`/`ForNamespaceWithContext`. It does **not** override `Secrets()`,
  so an ordinary `lister.Secrets(ns).Get(name)` still resolves through `multiLister` and still returns
  `NotFound` for unconfigured namespaces. The shared CEL context provider (`pkg/cel/libs/context.go`),
  `pkg/cel/policies/ivpol/engine/engine.go`'s `imagedataloader.NewImageContext` — which gets the raw,
  unconfined lister on purpose — and resource `imagePullSecrets` all keep their cache-only behavior
  precisely because they never call `ForNamespace`. **Do not promote the live GET onto `Secrets()`.**
- **The live GET is reachable only through `ScopeSecretLister`.** It returns the lister untouched when
  the namespace is empty, so cluster-scoped `ImageValidatingPolicy` is unaffected. The only sanctioned
  call sites are the namespaced-ivpol credential paths — `pkg/image/verification/evaluator/compiler.go`
  and `pkg/cel/libs/imageverify/impl.go`. Adding a caller elsewhere widens Secret access; treat it as
  a design change, not a refactor. (`ScopeSecretListerWithContext` is the context-binding variant and
  currently has **no non-test callers** — request-context propagation happens instead through
  `secretsKeychain.ResolveContext` calling `GetWithContext`. Don't cite it as a live path.)
- **Scoping fails closed.** With a non-empty namespace, a lister that did *not* come from
  `WithSecretClient` is still confined — cache-only, no live GET, foreign namespaces `Forbidden` —
  rather than returned unscoped. That matters because the CLI (`kubectl-kyverno apply`/`test`) passes
  a plain informer lister: it keeps working for same-namespace reads and loses only the live GET, but
  it cannot silently lose the boundary. Do not reintroduce a path that returns the original lister
  when the type assertion misses. A `nil` lister stays `nil`, since `NewSecretsKeychain` already
  rejects a nil lister when the policy names any reference, and wrapping it would change the error
  callers see.
- **The scope is enforced, not advisory.** `scopedSecretNamespaceLister` returns a `Forbidden` error
  for any namespace other than the policy's own, including Kyverno's installation namespace. Forbidden
  must stay authoritative — never fall back to the cached lister or to another namespace when a scoped
  read fails. A cache `NotFound` escalates to a live GET only when a client was configured; with a nil
  client the NotFound stands.
- **The live GET stays bounded.** It runs under the caller's context with a 5s timeout, and
  `GetWithContext` checks `ctx.Err()` first so a cancelled admission request does not issue an API
  call. Keep both; `Get` exists only so non-context-aware callers still get the bounded request.

## `ScopeSecretReferences` is validation, not normalization

`ScopeSecretReferences` (`secret.go`) rejects a reference to any namespace other than the policy's,
rather than silently rewriting it — the error names the reference, its namespace and the policy
namespace, and the caller wraps it in a `field.Forbidden` that supplies the field path. It also
rejects an empty policy namespace outright, so it can never be called in a way that degrades into
"match anything". `ParseSecretReference` defaults a bare name to the namespace the caller supplies,
which is why the caller must pass the *policy* namespace and not `config.KyvernoNamespace()` on
namespaced paths.

Callers must run these checks before compilation and again before runtime dynamic-attestor
verification, without mutating the cached policy object — compiled policies are immutable and shared
across admission requests.

## Pick the credential default namespace with `CredentialNamespace`, not from the policy

`CredentialNamespace(lister, fallback)` reads the namespace back off a confined lister and returns
`fallback` only when the lister is not confined, which is exactly the cluster-scoped and
global-credential case. Deriving it from the lister keeps the two from drifting: at the three call
sites the same `lister` value is passed to both `CredentialNamespace` and the keychain, so the
namespace used to resolve a bare name is by construction the namespace the lookup is allowed to read.
(The signature does not enforce that — don't pass one lister to the helper and a different one to the
keychain.)

This is what covers `pkg/image/verifiers/ivpol/cosign/opts.go`, where `sourceRemoteOpts` resolves
`signaturePullSecrets` with no policy in scope at all and previously hardcoded
`config.KyvernoNamespace()`. On the namespaced path `ScopePolicy` has already canonicalized every
reference to `namespace/name`, so this is defence in depth rather than the thing correctness rests
on — which is the point: a future path that reaches the keychain without canonicalizing first must
not silently read the installation namespace again.
