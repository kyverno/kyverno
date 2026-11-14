# Registry egress primitives

## Ownership and scope
- `policy.go` validates destinations;
  `transport.go` checks every request and hands it to a cloned `http.Transport`, which does its
  own dialing and pooling.
- `global.go` holds the process-wide policy and wrapped transport that
  `cmd/internal/registry.go` installs on the SDK registry client. Everything that fetches
  through that client is guarded; this is a registry-client boundary, not a process-wide
  network restriction. Image references are validated where they are fetched, so no caller
  needs to decide whether a reference came from the resource or from the policy author.
- Policies that declare their own `spec.credentials` cannot go through the global client, and
  `regcreds.RemoteOptsFromIvpolCredentials` hardcodes `regcreds.DefaultTransport`, so they would
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
- The policy is two denylists and nothing else: `DeniedHosts` (exact hostnames) and `DeniedIPs`
  (CIDRs or bare addresses). The order in `resolveAndValidate` *is* the policy — host match,
  else resolve, else address match on any resolved address — and there is no allowlist, no
  public/private classification, and no entry an operator cannot remove. Do not reintroduce a
  privileged tier: cloud metadata and loopback are ordinary default values, on the same footing
  as any other Kubernetes or cloud-provider default that guides you toward the right
  configuration without preventing another one.
- `Policy` is immutable after construction. Validate every DNS answer, including mapped/embedded
  IPv4 (`addressCandidates`), or any IPv4 entry becomes bypassable by spelling the address as
  IPv6.
- Denied-host entries are host-granular. Reject ports rather than stripping them:
  `resolveAndValidate` only ever receives a hostname, so a port could not be enforced and a
  stripped one would silently deny every port on that host. An IP address under the host list is
  likewise rejected, because literals are matched against the address list.
- `pkg/image/verification/credentials.RemoteOptions` is the only other place the guard is
  wired, because `regcreds.RemoteOptsFromIvpolCredentials` hardcodes its own transport. Its
  explicit-Secrets branch replaces the SDK-composed keychain rather than layering on it; that
  swap does not affect the guard, which rides on the transport those options already carry. A
  nil lister with declared Secrets still fails loudly there, via that package's context-aware
  `NewSecretsKeychain`.
- Audit logs at V(2) what enforce would reject and connects anyway; enforce rejects. Audit is the
  default so that enabling the guard cannot break image fetches before an operator has seen what
  it would deny.
- A malformed entry is logged and skipped, not fatal. These values come from a ConfigMap an
  operator edits at runtime, and a typo must not crashloop the admission controller and take the
  cluster's admission path with it. Only the mode is a hard error (`ValidateMode`, called from
  `cmd/internal/check.go`), because it is a flag fixed at deploy time.
- The lists are read once, at startup: `cmd/internal/registry.go` snapshots them from
  `config.Configuration` after the config controller has loaded the ConfigMap, and nothing
  re-reads them. An operator's edit needs a controller restart; a ConfigMap informer is a
  deliberate follow-up, so do not document or assume live reload.
- The chart's `config.registryEgressDeniedIPs` is **derived, not incremental**: every IANA special-purpose registry entry
  marked "Globally Reachable: False", plus the loopback/unspecified/link-local/multicast/RFC 1918
  space and the cloud-provider metadata addresses that a denylist now has to spell out explicitly
  because no classifier stands behind it. "Not globally routable" is not "not routed" — a
  cluster can point documentation or site-local space at an internal service, and the address
  validated here is the one dialed. Entries marked Globally Reachable: True (`192.31.196.0/24`,
  `192.175.48.0/24`, `192.52.193.0/24`, `2620:4f:8000::/48`) are deliberately absent, and a test
  pins that. Appending one range in isolation is how `198.18.0.0/15`, the documentation ranges
  and `fec0::/10` each shipped a round late — that history is why this bullet exists; re-derive
  from the registries instead.
- The defaults live only in `charts/kyverno/values.yaml` under `config.registryEgressDeniedHosts`
  / `config.registryEgressDeniedIPs`, as the maintainer asked. Do not add a Go copy: an empty or
  missing list denies nothing. Tests read the chart through `shippedConfig`, and
  `TestShippedDenylistsParse` fails if a chart entry would be skipped.
- Validate every request, including reused connections, redirects, and token requests routed
  through this transport. Reject opaque URLs and mismatched HTTP Host/URL authorities.
- Keep the policy the four steps the maintainer asked for: denied host, resolve, denied IP, hand
  off. The only addition is that the inner transport's direct dials check the address they reached
  (`checkAddress` on `RemoteAddr`), because it resolves the name again and could otherwise be sent
  elsewhere by DNS rebinding. Do not reintroduce address pinning, dial racing, or per-destination
  pools here. Custom TLS dialers must not bypass `DialContext`.
- Validate proxy and target separately. The proxy endpoint is fully exempt from both denylists,
  and its connection is not re-checked at dial time: it is operator configuration read from the process
  environment, not a destination an image reference can choose, and that exemption is what makes
  a sidecar egress proxy on loopback or inside RFC 1918 usable while those ranges are denied for
  registries. Exempting the proxy never exempts the target. The proxy still owns downstream
  DNS/routing, so local checks cannot bind its destination. Because of that, a proxied target the
  local resolver reports as not-found is deferred to the proxy rather than failing closed —
  otherwise proxy-only-resolvable registries break in every mode. Keep `dnsNotFound` narrow:
  literal addresses never reach the resolver, and `ErrAddressBlocked` plus every other resolver
  failure must stay fatal.
- A deferred target meets the denied-host list only. No address for that name is ever known
  here, so the denied-IP list never applies to it, and the proxy decides what it connects to. That is the accepted cost of
  the deferral, not an oversight: validating it would require resolving a name the controller
  cannot resolve. Do not restore a guarantee here that the guard cannot keep — if the deferral
  ever needs tightening, narrow when it applies rather than claiming the destination is checked.
- Credentials resolved for a deferred target travel with that request, since nothing validated
  where the proxy sends it. That follows from the deferral, not from where the guard sits: the
  transport is a backstop only for destinations it can resolve. Tighten it by narrowing when the
  deferral applies, not by adding a second check elsewhere.
- The DNS lookup in `RoundTrip` is bounded by `DialTimeout` or the shorter caller deadline.
  Close rejected request bodies.
- Keep resolved IPs/DNS server details out of returned `hostError` messages while retaining
  `errors.Is`/`errors.As` through unwrapping; detailed diagnostics stay in logs.

## Concurrency and checks
- `Policy` is immutable after `New`; `Transport` holds no mutable state of its own. The proxy
  decision travels with the request (`route` in its context), so the inner transport's proxy and
  dial hooks act on what `RoundTrip` checked.
- Run `go test -race ./pkg/utils/egress`; use injected resolvers/dialers and local fixtures.
- Do not use documentation addresses (`192.0.2.x`, `198.51.100.x`, `203.0.113.x`, `2001:db8::x`,
  `3fff::x`) as a stand-in for a reachable registry or proxy — they are in the shipped denied-IP
  list, so such a fixture is logged in audit mode and rejected in enforce mode. Use the public
  address constants in `policy_test.go` instead; they are real public addresses that nothing ever
  contacts, because every resolver and dialer here is stubbed. Documentation addresses belong only
  in tests whose point is that they are denied, or that an operator can remove them from the list.
- Preserve coverage for: denylist matching by host and by address, the proxy exemption and the
  not-found deferral, the dial-time address check, and that every chart entry parses. See `docs/user/registry-egress.md` for the operator-facing
  boundary.
