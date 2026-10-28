# Registry client adapter

## Ownership and scope
- The SDK owns reference parsing, credential composition/Secret refresh, remote options,
  pullers/pushers, and descriptor fetching; this adapter wires operator egress settings.
- `SetupGlobalRegistryClient` initializes the process-wide client once with `sync.Once`;
  initialize it before serving evaluations rather than replacing it per request.
- `New` deliberately remains native for legacy Policy integration, including `Keychain`,
  `Options`, and `FetchImageDescriptor`; do not change it to a guarded client.
- `GlobalImageDataOptions`, `ImageDataOptsFromImageVerificationCredentials`, and
  `imageDataOptions` return native options backed by `imageOptions`; there is no
  `nativeOptions` field. `ResourceImageOptions`, `ResourceImageOptionsFromCredentials`,
  and `resourceImageOptions` return guarded options for workload-origin images.
- Scope follows workload image values, not registry hosts; another author-chosen reference
  on the same host remains native. Callers own resource recognition and cache separation.
- Policy-controlled signature repositories, key/certificate and Sigstore/TUF endpoints
  remain upstream-native and outside this guard; do not introduce replacement clients.
- Provider-internal credential exchanges, including SDK/Flux Azure, and KMS networking
  retain their own clients; the registry transport does not guard those exchanges.

## Security and concurrency
- Validate operator configuration at startup; invalid programmatic settings fail closed.
- The guarded keychain validates the registry before credentials, preserving cancellation.
- `GuardResourceImageOptions` prevalidates a workload destination while retaining supplied
  credentials. Use context-free `NewRemoteOptions`; a transport appended to SDK `Options`
  cannot replace the transport already captured by its `remote.Reuse` puller.
- Bind the evaluation context at runtime, not policy compilation; return cloned option
  slices so one caller cannot mutate another caller's options.
- Keep `ValidateResourceImage` ahead of verification-cache hits and guarded image data
  separate from policy-native preloads (`pkg/cel/libs/imageverify/resource_context.go`).
- `sharedTransport` synchronizes its map with `transportMu` and shares pools by operator
  mode/allowlist, never credentials or request state. Secret authorization stays with the lister.
- Tests overriding global clients or `regcreds.DefaultTransport` must restore them and
  must not run concurrently with users of those globals.

## Checks and references
- Run `go test -race ./pkg/registryclient`; retain `scope_test.go` native/workload and
  token/redirect coverage plus cancellation, Secret lookup, and pool-isolation coverage.
- See `docs/dev/registry-client.md` and `docs/user/registry-egress.md` for caller boundaries.
