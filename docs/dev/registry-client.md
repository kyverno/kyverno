# Registry client ownership and egress

The Kyverno SDK (`github.com/kyverno/sdk/extensions/registryclient`) owns the registry client:
reference parsing, credential composition, Secret refresh, remote option construction, pullers and
pushers, and descriptor fetching. Kyverno does not wrap or reimplement it.

Kyverno contributes two things to that client, both from `pkg/utils/egress`:

- `registryclient.WithTransport(guard.Transport())` — a transport that validates and pins the
  destination of every request, including token challenges, redirects, and requests that reuse a
  pooled connection.
- `registryclient.WithKeychainWrapper(guard.GuardKeychain)` — validates the registry before the
  underlying keychain resolves credentials for it, so a blocked destination never receives them.

`cmd/internal/registry.go` installs both on the process-wide client in
`registryclient.SetupGlobalRegistryClient`. There is one registry client per process.
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
guard is applied. It builds the same credential chain through `registryclient.NewRemoteOptions`
and adds the two guard options. If a third site ever needs policy-declared credentials, it must
go through that function rather than `regcreds`.

One subtlety inside it: when the policy declares explicit Secrets, `RemoteOptions` appends a
context-aware keychain with `remote.WithAuthFromKeychain`, which *replaces* the chain the SDK
composed rather than layering on it. `registryclient.WithKeychainWrapper` only ever wrapped the
SDK's own chain, so that branch applies `GuardKeychain` itself. Dropping that wrap would let a
policy with explicit Secrets resolve credentials for a destination the guard would refuse.

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

With no guard installed (CLI, unit tests) `egress.Global()` returns nil and both guard options are
no-ops, leaving the SDK's native behavior.

## Scope boundaries

Sigstore infrastructure (Rekor, Fulcio, TSA, TUF), KMS clients, and the CEL `http` library do not
use the registry client and are untouched. This is a registry-client boundary, not a process-wide
network policy.

Provider-internal credential exchanges are a known gap: the Flux Azure ACR exchanger reached
through the SDK constructs its own `http.Client{}` and exposes no transport or context injection,
so neither the transport nor the keychain wrapper reaches inside it. The registry the credentials
are being fetched for is validated before the exchanger is invoked. Closing this requires
transport and context injection in the provider, then a dependency bump here — not a
consumer-local Azure wrapper.

Connection pools are keyed by the validated destination address set, so clients sharing operator
settings share pools while keeping their own credentials and request state.

See `pkg/utils/egress/AGENTS.md` for the primitives' invariants and
`docs/user/registry-egress.md` for the operator-facing contract.
