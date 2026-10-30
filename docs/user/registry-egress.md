# Image registry egress

Image references reaching Kyverno are attacker-influenced: a user who can create a Pod chooses
the `image:` value, and Kyverno connects to whatever registry that value names in order to read
metadata, verify signatures, or resolve a digest. Without a destination check, a reference such
as `169.254.169.254/app:v1` turns admission control into a request against the cluster's cloud
metadata service.

Kyverno validates the destination of every request its image registry client makes. The check
happens where the image is fetched, so it covers any reference that reaches a registry — one read
from the admission object, one written into the policy, or one produced by a CEL expression. No
part of Kyverno has to decide which of those a reference was.

## What is covered

Everything that fetches through Kyverno's registry client:

- CEL `image.getMetadata` and `imagedata` lookups
- ImageValidatingPolicy image metadata, signature and attestation verification, digest mutation
- DeletingPolicy image metadata lookups in the cleanup controller
- Registry token challenges, redirects, and signature/referrer downloads that follow the first
  request, including the signature repository selected by `source.repository`

For each request the guard resolves the destination, rejects it or pins the connection to the
addresses it just validated, and re-checks on every redirect and reused connection. It validates
before credentials are resolved, so a blocked host never receives registry credentials.

## What is not covered

Sigstore infrastructure (Rekor, Fulcio, timestamp authorities, TUF trust roots), KMS endpoints,
and the CEL `http` library do not use the registry client, so they keep their upstream clients and
are unaffected. These are operator- or policy-author-configured endpoints rather than values taken
from an admitted resource.

Cloud provider credential exchanges are a known gap. The Azure ACR exchanger reached through the
SDK builds its own HTTP client and accepts neither Kyverno's transport nor its request context, so
the destination check does not extend inside that exchange. The registry the credentials are
being fetched *for* is still validated first.

## Rollout

`privateRegistryEgressMode` defaults to `audit`: ordinary private registry addresses stay
reachable, and requests that `enforce` would reject are logged at verbosity 2. Run in `audit`
first and collect those log lines to build the allowlist.

Both modes always reject cloud metadata, loopback, link-local, unspecified, multicast, and
broadcast addresses, including addresses reached through a listed hostname and IPv4 addresses
embedded in IPv6 (NAT64, 6to4, IPv4-compatible). No allowlist entry overrides those.

In `enforce`, a private destination must match an allowlist entry:

```yaml
features:
  registryClient:
    privateRegistryEgressMode: enforce
    privateRegistryAllowlist:
      - registry.corp.example
      - auth.corp.example
      - 10.20.30.0/24
      - fd12:3456:789a::/64
```

Include private registry token and redirect services, not just the registry hostname. Entries must
be exact hostnames, IP literals, or CIDRs; wildcards, URLs, empty elements, and commas inside an
entry are rejected. Use `[]` for no exemptions. Invalid configuration fails controller startup
rather than being ignored at request time.

Ports are rejected too — write `registry.corp.example`, not `registry.corp.example:5000`. An
exemption is matched against the destination host, so a port could not be enforced and would
quietly exempt every port on that host. Allowlisting is host-granular by design.

The equivalent flags are `--privateRegistryEgressMode` and `--privateRegistryAllowlist` (the
latter comma-separated). The admission, background, reports, and cleanup controllers all accept
them. Cleanup applies them to DeletingPolicy image metadata lookups without enabling registry
credential helpers or image-pull Secret informers, so it needs no additional RBAC. Per-controller
values can be overridden under `<controller>.featuresOverride.registryClient`; setting
`privateRegistryAllowlist: []` there clears the global list for that controller.

Proxies keep their existing environment configuration and are treated as trusted operator
configuration, so an ordinary private proxy address needs no allowlist entry in either mode. Hard
blocks still apply to the proxy endpoint itself. The proxy and the target registry are validated
separately: routing through a proxy does not exempt the target from the allowlist requirement in
`enforce`. Because the proxy performs its own DNS and routing, Kyverno cannot pin what it connects
to downstream — that remains the proxy's responsibility.
