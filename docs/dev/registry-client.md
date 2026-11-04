# Registry client ownership and egress

The Kyverno SDK (`github.com/kyverno/sdk/extensions/registryclient`) owns the registry client:
reference parsing, credential composition, Secret refresh, remote option construction, pullers and
pushers, and descriptor fetching. Kyverno does not wrap or reimplement it.

Kyverno contributes one thing to that client, from `pkg/utils/egress`:

- `registryclient.WithTransport(guard.Transport())` — a transport that checks the destination of
  every request, including token challenges, redirects, and requests that reuse a pooled
  connection, then passes it to an ordinary `http.Transport`. That inner transport resolves the
  name again to dial it, so the address each direct connection reaches is checked as well, which
  stops a DNS answer that changes after the check (rebinding).

What it validates against is two operator denylists — exact hostnames and CIDRs/IP addresses, read
from the `kyverno` ConfigMap at startup — plus the `--privateRegistryEgressMode` flag that decides
whether a match is logged (`audit`) or rejected (`enforce`). There is no allowlist, no built-in
public/private classification, and no entry that an operator cannot remove; the shipped defaults
are chart values. Two destinations are outside that check by construction: a proxy endpoint, which
is operator configuration from the environment rather than something an image reference can
choose, and a proxied name the local resolver cannot find, which still meets the host denylist but
is deferred to the proxy for its address because none is ever known here. More generally, for any
proxied request the proxy does its own resolution, so the address check binds what this process
resolved, not where the proxy connects. See `docs/user/registry-egress.md`.

That is the only guard. Credential resolution is deliberately unguarded: every request a resolved
credential could ride on goes through this transport, which refuses a denied destination before
the first byte leaves, so a keychain-level check would only decide whether credentials are
*selected* for a host they can never be *sent* to. Keeping one chokepoint also removes an
invariant that had to be mirrored by hand — the keychain and the transport each implemented the
proxy-deferral carve-out, and changing either alone made a proxy-only-resolvable registry unusable
or the carve-out dead code.

`cmd/internal/registry.go` installs it on the process-wide client in
`registryclient.SetupGlobalRegistryClient`. There is one registry client per process. Its
`egressConfig` pairs the mode flag with `config.Configuration`'s denylists, which
`startConfigController` has already loaded; they are not re-read afterwards, so a ConfigMap edit
needs a controller restart until a ConfigMap informer lands. A nil list means the ConfigMap says
nothing about it and the shipped defaults apply; an empty non-nil list is an operator who cleared
it. A malformed entry is logged and skipped rather than failing startup — only the mode is
validated fatally, in `cmd/internal/check.go`.

`setupRegistryEgress` configures that same global for controllers that read image metadata but
carry no registry credentials — it passes a nil Secret lister and no credential helpers, so those
controllers need no image-pull Secret informers or the RBAC they require.

Because the guard lives on the client, every caller that fetches through it is covered without
knowing the guard exists: `registryclient.GlobalOptsOrDefault`, the image data loader, the notary
verifier, and cosign's `ociremote` options all derive from the global client's options.

## The one place the guard must be passed explicitly

`regcreds.RemoteOptsFromIvpolCredentials` pins `regcreds.DefaultTransport`. A policy that declares
its own `spec.credentials` would therefore fetch through an unguarded transport while every other
policy went through the global client. Both call sites —
`pkg/image/verification/evaluator/compiler.go` and `pkg/cel/libs/imageverify/impl.go` — reach
those credentials through `pkg/image/verification/credentials.RemoteOptions`, which is where the
guarded transport is applied. It builds the same credential chain through
`registryclient.NewRemoteOptions` and adds `WithTransport`. If a third site ever needs
policy-declared credentials, it must go through that function rather than `regcreds`.

When the policy declares explicit Secrets, `RemoteOptions` appends a context-aware keychain with
`remote.WithAuthFromKeychain`, which *replaces* the chain the SDK composed rather than layering on
it. That swap does not affect the guard, which lives on the transport the same options already
carry.

Swapping in `registryclient.NewRemoteOptions` also changes one behavior. It only builds the Secret
keychain when a lister is present:

```go
if len(o.imagePullSecrets) > 0 && o.secretLister != nil {
```

where `regcreds` built it regardless and failed at resolution with `secret lister is not
configured`. Without that, a policy declaring Secrets with no lister would fall back to
`authn.DefaultKeychain` and fetch unauthenticated — silently. The explicit-Secrets branch above
preserves the loud failure, because its `NewSecretsKeychain` returns that same error for a nil
lister. This is reachable from the CLI, which has no cluster client to build a lister from.

With no guard installed (CLI, unit tests) `egress.Global()` returns nil, `Transport()` returns nil,
and `registryclient.WithTransport(nil)` leaves the SDK's native behavior.

## Scope boundaries

Sigstore infrastructure (Rekor, Fulcio, TSA, TUF), KMS clients, and the CEL `http` library do not
use the registry client and are untouched. This is a registry-client boundary, not a process-wide
network policy.

Provider-internal credential exchanges are a known gap: the Flux Azure ACR exchanger reached
through the SDK constructs its own `http.Client{}` and exposes no transport injection, so the
guarded transport does not reach inside it. It is not reachable from an attacker-chosen reference
either — the SDK's Azure keychain only engages for `.azurecr.io`-family registry suffixes. Closing
it properly requires transport injection in the provider, then a dependency bump here — not a
consumer-local Azure wrapper.

See `pkg/utils/egress/AGENTS.md` for the primitives' invariants and
`docs/user/registry-egress.md` for the operator-facing contract.
