# Registry egress primitives

## Ownership and scope
- `policy.go` validates destinations; `network.go` dials validated addresses;
  `transport.go` enforces per-request validation and manages pinned connection pools.
- `global.go` holds the process-wide policy and wrapped transport that
  `cmd/internal/registry.go` installs on the SDK registry client. Everything that fetches
  through that client is guarded; this is a registry-client boundary, not a process-wide
  network restriction. Image references are validated where they are fetched, so no caller
  needs to decide whether a reference came from the resource or from the policy author.
- `registry.go` builds options for policies that declare their own `spec.credentials`.
  Those cannot go through the global client, and `regcreds.RemoteOptsFromIvpolCredentials`
  pins `regcreds.DefaultTransport`, so they would escape the guard. Any new caller that
  needs policy-declared credentials must use `GuardedRemoteOptions`, not `regcreds`.
- Sigstore infrastructure (Rekor, Fulcio, TSA, TUF), KMS, and the CEL HTTP library do not use
  the registry client and are therefore untouched here.
- Credential composition belongs to the SDK. `GuardKeychain` only validates the destination
  before credentials are resolved; it does not compose or replace them. Provider-internal
  exchanges such as the Flux Azure ACR exchanger build their own `http.Client` and cannot be
  reached from here — see `docs/dev/registry-client.md`.

## Security invariants
- `Policy` is immutable after construction. Validate every DNS answer, even for allowlisted
  hosts, including mapped/embedded IPv4; exemptions never override hard-block rules.
- Allowlist entries are host-granular. Reject ports rather than stripping them:
  `resolveAndValidate` only ever receives a hostname, so a port could not be enforced and a
  stripped one would silently exempt every port on that host.
- `registryclient.New` only builds the Secret keychain when a lister is present, so declared
  Secrets with a nil lister would fall back to the default keychain and fetch
  unauthenticated. `requiredSecretsKeychain` keeps that failure explicit; do not drop it.
- Audit permits ordinary private destinations with logging; enforce requires exemptions.
  Metadata, loopback, link-local, unspecified, multicast, and broadcast remain blocked.
- Validate every request, including reused connections, redirects, and token requests routed
  through this transport. Reject opaque URLs and mismatched HTTP Host/URL authorities.
- Pin direct dials to validated IPs without a second lookup; preserve Host/TLS SNI.
  Custom TLS dialers must not bypass the validated dial path.
- Validate proxy and target separately: a private proxy does not exempt private targets.
  The proxy still owns downstream DNS/routing, so local checks cannot pin its destination.
- DNS and connection attempts share `DialTimeout` or the shorter caller deadline.
  Preserve the original request context for response bodies; close rejected request bodies.
- Keep resolved IPs/DNS server details out of returned `hostError` messages while retaining
  `errors.Is`/`errors.As` through unwrapping; detailed diagnostics stay in logs.

## Concurrency and checks
- Keep dial plans request-local. `Transport.mu` protects the bounded destination-pool LRU;
  pools are keyed by validated target/proxy address sets, not credentials or request state.
- Pool eviction retires idle connections without interrupting active responses; retain
  atomic retirement tracking and cleanup when those response bodies close.
- Run `go test -race ./pkg/utils/egress`; use injected resolvers/dialers and local fixtures.
- Preserve hard-block/proxy, DNS-change/HTTP2 pooling, eviction, deadline/fallback, and
  cancellation tests. See `docs/user/registry-egress.md` for the operator-facing boundary.
