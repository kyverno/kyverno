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
`pkg/image/verification/evaluator/compiler.go` and `pkg/cel/libs/imageverify/impl.go` — use
`egress.GuardedRemoteOptions` instead, which builds the same credential chain through
`registryclient.NewRemoteOptions` and adds the two guard options. If a third site ever needs
policy-declared credentials, it must do the same.

Swapping in `registryclient.NewRemoteOptions` changes one behavior that has to be restored by
hand. It only builds the Secret keychain when a lister is present:

```go
if len(o.imagePullSecrets) > 0 && o.secretLister != nil {
```

where `regcreds` built it regardless and failed at resolution with `secret lister is not
configured`. Without that, a policy declaring Secrets with no lister falls back to
`authn.DefaultKeychain` and fetches unauthenticated — silently. `requiredSecretsKeychain` keeps
the failure explicit. This is reachable from the CLI, which has no cluster client to build a
lister from.

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
