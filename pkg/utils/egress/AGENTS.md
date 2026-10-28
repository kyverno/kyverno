# Workload egress primitives

## Ownership and scope
- `policy.go` validates destinations; `network.go` dials validated addresses;
  `transport.go` enforces per-request validation and manages pinned connection pools.
- Callers apply these primitives only to workload-derived image access. Author-controlled
  endpoints retain native clients and remain out of scope, not a process-wide restriction.
- Credential composition belongs to the SDK/registry adapter. Provider-internal exchanges
  such as Azure and policy-selected Sigstore/TUF endpoints are not guarded here.

## Security invariants
- `Policy` is immutable after construction. Validate every DNS answer, even for allowlisted
  hosts, including mapped/embedded IPv4; exemptions never override hard-block rules.
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
