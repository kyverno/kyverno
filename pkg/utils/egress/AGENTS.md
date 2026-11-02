# Registry egress primitives

## Ownership and scope
- `policy.go` validates destinations; `network.go` dials validated addresses;
  `transport.go` enforces per-request validation and manages pinned connection pools.
- `global.go` holds the process-wide policy and wrapped transport that
  `cmd/internal/registry.go` installs on the SDK registry client. Everything that fetches
  through that client is guarded; this is a registry-client boundary, not a process-wide
  network restriction. Image references are validated where they are fetched, so no caller
  needs to decide whether a reference came from the resource or from the policy author.
- Policies that declare their own `spec.credentials` cannot go through the global client, and
  `regcreds.RemoteOptsFromIvpolCredentials` pins `regcreds.DefaultTransport`, so they would
  escape the guard. They are built by `pkg/image/verification/credentials.RemoteOptions`
  instead. Any new caller that needs policy-declared credentials must go through it, not
  `regcreds`.
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
- `pkg/image/verification/credentials.RemoteOptions` is the only other place the guard is
  wired. Its explicit-Secrets branch replaces the SDK-composed keychain rather than layering
  on it, so that branch carries its own `GuardKeychain` wrap — `WithKeychainWrapper` cannot
  reach a keychain appended after the SDK has composed its own. A nil lister with declared
  Secrets still fails loudly there, via that package's context-aware `NewSecretsKeychain`.
- Audit permits ordinary private destinations with logging; enforce requires exemptions.
  Metadata, loopback, link-local, unspecified, multicast, and broadcast remain blocked.
- Private-class covers much more than Go's `IsPrivate()`, which only knows RFC 1918 and
  `fc00::/7`. "Not globally routable" is not "not routed" — a cluster can point documentation
  or site-local space at an internal service, and the address validated here is the one dialed.
  The list in `policy.go` is **derived, not incremental**: it is every IANA special-purpose
  registry entry marked "Globally Reachable: False" that Go does not already report as
  loopback, link-local, unspecified, multicast or private. Entries marked True (`192.31.196.0/24`,
  `192.175.48.0/24`, `192.52.193.0/24`, `2620:4f:8000::/48`) are deliberately excluded and
  `TestGloballyReachableSpecialPurposeRangesStayPublic` pins that. Appending one range in
  isolation is how `198.18.0.0/15`, the documentation ranges and `fec0::/10` each shipped a
  round late — re-derive from the registries instead.
- Validate every request, including reused connections, redirects, and token requests routed
  through this transport. Reject opaque URLs and mismatched HTTP Host/URL authorities.
- Pin direct dials to validated IPs without a second lookup; preserve Host/TLS SNI.
  Custom TLS dialers must not bypass the validated dial path.
- Validate proxy and target separately: a private proxy does not exempt private targets.
  The proxy still owns downstream DNS/routing, so local checks cannot pin its destination.
  Because of that, a proxied target the local resolver reports as not-found is deferred to the
  proxy rather than failing closed — otherwise proxy-only-resolvable registries break in every
  mode. Keep `dnsNotFound` narrow: literal addresses never reach the resolver, and
  `ErrAddressBlocked` plus every other resolver failure must stay fatal.
- A deferred target is unvalidated, not merely unpinned. `plan.targetIPs` is nil and the dial
  goes to `plan.proxyIPs`, so no address for that name is ever compared against the hard-block
  list or the allowlist, and the proxy decides what it connects to. That is the accepted cost of
  the deferral, not an oversight: validating it would require resolving a name the controller
  cannot resolve. Do not restore a guarantee here that the guard cannot keep — if the deferral
  ever needs tightening, narrow when it applies rather than claiming the destination is checked.
- The keychain is resolved before the transport dials, so `Transport.RoundTrip`'s proxy carve-out
  is only reachable if `guardedKeychain.ResolveContext` makes the same allowance; change one side
  and the other must change with it, or the deferral becomes dead code and the destination
  unusable after all. Where both sides defer, nothing stops the resolved credentials from
  reaching whatever the proxy connects to; the transport is a backstop only for destinations it
  can resolve. `guardedKeychain.proxied` tries both `https` and `http` because a keychain is
  given a host and no scheme; erring towards true only defers a check the transport still makes
  when no proxy is actually configured for the request.
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
- Do not use documentation addresses (`192.0.2.x`, `198.51.100.x`, `203.0.113.x`, `2001:db8::x`,
  `3fff::x`) as a stand-in for a reachable public registry or proxy — `privateAddress`
  classifies them, so such a fixture is logged as private in audit mode and rejected in enforce
  mode. Use the `publicIPv4` / `publicIPv4Alt` / `publicIPv6` / `publicIPv4Nth` constants in
  `policy_test.go`; they are real public addresses that nothing ever contacts, because every
  resolver and dialer here is stubbed. Documentation addresses belong only in tests whose point
  is that they are rejected, such as `TestSpecialPurposeRangesAreExemptableByAddress`.
- Preserve hard-block/proxy, DNS-change/HTTP2 pooling, eviction, deadline/fallback, and
  cancellation tests. See `docs/user/registry-egress.md` for the operator-facing boundary.
