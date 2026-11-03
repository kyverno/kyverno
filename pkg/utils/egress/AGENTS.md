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
- Credential composition belongs to the SDK and is deliberately unguarded. The transport is the
  only chokepoint: every request a resolved credential could ride on passes through it, so a
  keychain check would only decide whether credentials are *selected* for a host they can never
  be *sent* to. Do not reintroduce one — the earlier `GuardKeychain` forced the proxy-deferral
  carve-out to be implemented identically in two places, and changing one side alone either made
  a proxy-only-resolvable registry unusable or made the carve-out dead code. Provider-internal
  exchanges such as the Flux Azure ACR exchanger build their own `http.Client` and cannot be
  reached from here — see `docs/dev/registry-client.md`.

## Security invariants
- `Policy` is immutable after construction. Validate every DNS answer, even for allowlisted
  hosts, including mapped/embedded IPv4; exemptions never override hard-block rules.
- Allowlist entries are host-granular. Reject ports rather than stripping them:
  `resolveAndValidate` only ever receives a hostname, so a port could not be enforced and a
  stripped one would silently exempt every port on that host.
- `pkg/image/verification/credentials.RemoteOptions` is the only other place the guard is
  wired, because `regcreds.RemoteOptsFromIvpolCredentials` pins its own transport. Its
  explicit-Secrets branch replaces the SDK-composed keychain rather than layering on it; that
  swap does not affect the guard, which rides on the transport those options already carry. A
  nil lister with declared Secrets still fails loudly there, via that package's context-aware
  `NewSecretsKeychain`.
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
- That list is `DefaultPrivateRanges`, and operators replace it wholesale via
  `features.registryClient.privateRegistryRanges` / `--privateRegistryRanges`. Two copies exist
  on purpose — the chart's is what operators read and edit, the Go one is the flag default for
  installs without the chart — and `TestDefaultPrivateRangesMatchTheChart` fails if they drift,
  so edit both. `Config.PrivateRanges` distinguishes nil ("unset, keep the defaults") from an
  empty slice ("operator cleared it"); `ParseRanges` never returns nil for that reason, and the
  chart renders `--privateRegistryRanges` even when empty while the allowlist flag is omitted.
  Nothing here reaches `hardBlocked` or Go's `IsPrivate`: clearing the list cannot expose
  metadata, loopback or RFC 1918.
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
- Credentials resolved for a deferred target travel with that request, since nothing validated
  where the proxy sends it. That follows from the deferral, not from where the guard sits: the
  transport is a backstop only for destinations it can resolve. Tighten it by narrowing when the
  deferral applies, not by adding a second check elsewhere.
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
