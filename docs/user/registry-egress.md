# Image registry egress

Image references reaching Kyverno are attacker-influenced: a user who can create a Pod chooses
the `image:` value, and Kyverno connects to whatever registry that value names in order to read
metadata, verify signatures, or resolve a digest. Without a destination check, a reference such
as `169.254.169.254/app:v1` turns admission control into a request against the cluster's cloud
metadata service.

Kyverno validates the destination of every request its image registry client makes, with one
exception for proxied registries the controller cannot resolve, described under Rollout. The check
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
addresses it just validated, and re-checks on every redirect and reused connection. Credentials
are selected before that check runs, so a denied host can cause a credential lookup — but in
`enforce` mode they are never sent to it, because the transport refuses the request before the
first byte is written.
The same holds for `source.signaturePullSecrets`. One exception: a proxied name the local resolver
cannot find is checked against the denied hosts only, so credentials are resolved and sent for an
address only the proxy decides. See Proxies.

## What is not covered

Sigstore infrastructure (Rekor, Fulcio, timestamp authorities, TUF trust roots), KMS endpoints,
and the CEL `http` library do not use the registry client, so they keep their upstream clients and
are unaffected. These are operator- or policy-author-configured endpoints rather than values taken
from an admitted resource.

Cloud provider credential exchanges are a known gap. The Azure ACR exchanger reached through the
SDK builds its own HTTP client and does not accept Kyverno's transport, so the destination check
does not extend inside that exchange. It is not reachable from an image reference in an admitted resource:
that exchanger only engages for `.azurecr.io`-family registries.

## Rollout

The policy is two denylists and nothing else. There is no allowlist, no built-in notion of a
"private" address, and nothing that cannot be switched off:

1. the destination host matches an entry in `registryEgressDeniedHosts` — deny; otherwise
2. resolve the name, and if any address it resolves to matches an entry in
   `registryEgressDeniedIPs` — deny; otherwise
3. the request goes through.

Everything Kyverno considers dangerous out of the box — cloud metadata names and addresses,
loopback, link-local, RFC 1918, and the rest — ships as ordinary values you can read and remove,
exactly like any other Kubernetes or cloud-provider default. The defaults tell you what the right
configuration looks like; they do not stop you from choosing a different one.

`privateRegistryEgressMode` defaults to `audit`: a denied destination is connected to anyway, and
every request `enforce` would reject is logged at verbosity 2. Run in `audit` first, collect those log lines, and confirm
the set of denied destinations matches your cluster before switching to `enforce`.

The two lists come from the `kyverno` ConfigMap, under the keys `registryEgressDeniedHosts` and
`registryEgressDeniedIPs` (comma-separated values if you edit the ConfigMap by hand). With the
chart they are Helm values:

```yaml
features:
  registryClient:
    privateRegistryEgressMode: enforce

config:
  registryEgressDeniedHosts:
    - metadata.internal
    - metadata.google.internal
    - instance-data.ec2.internal
    # Added for this cluster: an internal service that answers on a name, not a fixed address.
    - vault.corp.example
```

Setting a key replaces the shipped list rather than adding to it, so write out every entry you
want to keep — `docs/user/registry-egress/enforce-values.yaml` carries the full shipped lists as a
starting point, and `charts/kyverno/values.yaml` documents each default with the reason it is
there. Leaving a key out entirely keeps the shipped list; setting it to `[]` clears the list,
which is honored as written.

Denied-host entries are exact hostnames: no wildcards, no subdomain matching, no ports. Write
`registry.corp.example`, not `registry.corp.example:5000` — the list is matched against the
destination host alone, so a port could not be enforced and would silently deny every port on that
host. An IP address written under `registryEgressDeniedHosts` is ignored with a log line, because
a literal reference is checked against the address list instead; put it under
`registryEgressDeniedIPs`. Denied-IP entries are CIDRs or bare IP addresses, one list item each.

Removing a shipped default is a supported thing to do. If your cluster genuinely routes one of
these ranges to real infrastructure that registry fetches must reach, drop it:

```yaml
config:
  registryEgressDeniedIPs:
    # 100.64.0.0/10 is deliberately absent: this cluster's registry sits in carrier-grade NAT
    # space, which is ordinary routed address space here.
    - 127.0.0.0/8
    - ::1/128
    - 169.254.0.0/16
    # ... plus every other shipped entry you want kept; see enforce-values.yaml.
```

IPv4 addresses embedded in IPv6 are decoded before the address check, so a denied IPv4 entry
cannot be bypassed by spelling the same address as IPv6. This covers the NAT64 well-known prefix
(`64:ff9b::/96`), 6to4 (`2002::/16`), IPv4-mapped (`::ffff:a.b.c.d`), and IPv4-compatible
(`::a.b.c.d`) forms. A cluster using its own network-specific NAT64 prefix instead of the
well-known one is not decoded, because that prefix is a local choice Kyverno cannot infer — deny
that prefix explicitly if you use one.

### Applying a change

The ConfigMap is read once, during controller startup. Editing it does not take effect until the
controller restarts; a ConfigMap informer that would pick the change up live is deliberately left
to a follow-up rather than half-done here. Restart the controllers after changing either list:

```sh
kubectl -n kyverno rollout restart deployment/kyverno-admission-controller
```

An invalid entry — a malformed CIDR, a hostname with a port, an IP under the host list — is logged
and skipped, and the rest of the list still applies. A typo in a ConfigMap an operator edits at
runtime must not crashloop the admission controller and take the cluster's admission path down
with it. Only the mode flag is a hard error at startup, because it is fixed at deploy time rather
than changing under a running process.

### Flags and per-controller settings

`--privateRegistryEgressMode` is the only flag; the lists are not flags, because they are expected
to grow long enough that passing them on a command line is unworkable. The admission, background,
reports, and cleanup controllers all accept it, and all four read the same ConfigMap, so the
denylists are identical across them. Cleanup applies them to DeletingPolicy image metadata lookups
without enabling registry credential helpers or image-pull Secret informers, so it needs no
additional RBAC. The mode can be set per controller under
`<controller>.featuresOverride.registryClient.privateRegistryEgressMode`.

### Proxies

Proxies keep their existing environment configuration. The proxy endpoint is fully exempt from
both denylists: it is operator configuration taken from the process environment, not a destination
an image reference can choose. A sidecar egress proxy on loopback, or one inside RFC 1918, works
while those ranges are denied for registries.

The proxy and the target registry are validated separately — routing through a proxy does not
exempt the target. Because the proxy performs its own DNS and routing, Kyverno cannot pin what it
connects to downstream: a name that resolves to an allowed address here can resolve to a denied one
at the proxy. Restricting that remains the proxy's responsibility.

A registry that resolves only from the proxy's network is still reachable. When a proxy is
configured and the controller's own resolver reports the name as not found, the destination is
left to the proxy and the request proceeds; the skip is logged at verbosity 2.

Deferring means Kyverno never learns an address for that name. The name is still checked against
`registryEgressDeniedHosts`, which runs before resolution, but `registryEgressDeniedIPs` cannot be
applied to an address Kyverno never sees. The connection is pinned to the proxy, and
wherever the proxy resolves the name to is where the request goes, along with any credentials
resolved for it. Restricting what the proxy connects to is the proxy's responsibility. If that
matters for your cluster, either configure the proxy to refuse the destinations you care about, or
give the controller a resolver that can see these names so the ordinary checks apply.

This applies only to that one case. A name that does resolve is validated as usual, a literal IP
address never consults the resolver and so can never be excused this way, and any other resolver
failure still fails closed.
