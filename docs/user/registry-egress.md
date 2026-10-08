# CEL registry egress configuration

CEL image lookups and ImageValidatingPolicy verification apply an egress guard before registry requests and credential resolution.
The guard also covers registry token services, redirect destinations, and the HTTP
services used for signature verification: Rekor, certificate transparency (CT)
services, TUF mirrors, certificate roots, and public key URLs. These services share
the registry egress mode and allowlist. Direct connections use the validated IP address, including when the registry is allowlisted,
so a second DNS lookup cannot replace it with a different destination. DNS
resolution and all dial attempts share a five-second deadline, shortened by the
request deadline when present. IPv4 and IPv6 attempts can race for faster fallback.
The remaining time is also apportioned across addresses within each family, so an
unreachable first address can fall back to another within the same deadline.

KMS key references retain their provider SDK networking; this registry and Sigstore
HTTP guard is not a general process-wide network policy.

## Release behavior and migration

The default `privateRegistryEgressMode` is `audit`. This preserves access to private
registries during an upgrade, while logging private destinations that `enforce`
would reject at verbosity 2. RFC1918 IPv4, IPv6 unique-local, and carrier-grade NAT
addresses are allowed in audit mode. These requests still receive DNS validation
and direct-connection IP pinning.

Both modes always reject cloud metadata endpoints, link-local, loopback,
unspecified, multicast, and broadcast addresses. An allowlist entry cannot bypass
these restrictions. Registries using those addresses must move to an ordinary
private or public address before upgrading.

Audit mode preserves private network connectivity; it does not prevent a policy
from reaching arbitrary private addresses in those permitted ranges. Operators
who need that protection should configure the allowlist and enable `enforce`.
Malformed allowlist entries and unrecognized mode values stop controller startup
before informers are started.

## Configure enforcement

Use these Helm values to permit an internal registry, its token service, and a
specific registry subnet:

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

The equivalent controller flags are:

```text
--privateRegistryEgressMode=enforce
--privateRegistryAllowlist=registry.corp.example,auth.corp.example,10.20.30.0/24,fd12:3456:789a::/64
```

Entries accept exact DNS hostnames, IP literals, and CIDRs. A hostname entry permits
that hostname's ordinary private addresses; an IP or CIDR entry permits matching
resolved addresses. Hostname matching is case-insensitive and ignores a trailing
dot. A hostname may include a port, but the permission applies to the hostname
across ports. Wildcards, URLs, paths, malformed addresses, and empty list elements
are rejected. Use the narrowest practical hostnames or subnets. Public registries
do not need allowlist entries.

Registry credentials remain scoped to their configured registry. Permitting a
network destination does not give that destination another registry's credentials.
Private token services, redirect destinations, and signature verification services
need their own allowlist entries in enforce mode; a registry entry does not
automatically permit them.

1. Upgrade with the default audit mode and normal verbosity 2 logging.
2. Exercise ImageValidatingPolicy signature verification, CEL image lookups,
   and background scans that use your registries.
3. Review the audit logs and add the private registries, token services, redirect destinations, and
   signature verification services that your policies require.
4. Enable enforce mode and repeat those operations before completing the rollout.

The setting is passed to admission, background, and reports controllers. These
controllers support `featuresOverride.registryClient` to change it for one
controller. Keep these settings consistent unless a difference is intentional.
Cleanup receives neither registry egress flag.

## HTTP and HTTPS proxies

An operator-configured `HTTP_PROXY` or `HTTPS_PROXY` may itself use an ordinary
private address without appearing in the registry allowlist, in either mode.
Proxy addresses still cannot use the address categories that are always blocked.
`NO_PROXY` keeps its normal environment-proxy behavior.

The registry destination is checked independently before forwarding through a
proxy. A private proxy does not authorize a private destination in enforce mode.
Kyverno pins the connection to the validated proxy IP, but an upstream HTTP proxy
resolves the destination hostname itself. Kyverno cannot pin or inspect that
upstream DNS resolution; configure the trusted proxy's own DNS and egress controls
to enforce the same destination restrictions. Direct connections are pinned to the
validated destination IP.

## Reproducing the chart configuration

The example values in [registry-egress/enforce-values.yaml](registry-egress/enforce-values.yaml)
can be rendered without a cluster:

```sh
helm template kyverno charts/kyverno --namespace kyverno --kube-version 1.32.0 \
  -f docs/user/registry-egress/enforce-values.yaml
```

The admission, background, and reports deployments should include
`--privateRegistryEgressMode=enforce` and the same `--privateRegistryAllowlist`
values. Rendering without this values file should produce
`--privateRegistryEgressMode=audit` on those three controllers. The cleanup
deployment should contain neither flag.

Use `privateRegistryAllowlist: []` when no entries are needed. Helm rejects empty
strings (including whitespace-only entries) before serializing the list to a
controller flag. This also applies to controller-specific overrides.

A local regression check renders both modes and each supported controller override,
then passes the rendered arguments to the corresponding executable with `--help`.
This checks the actual flag parsers without starting controllers or contacting a
cluster. It also checks that Helm rejects empty allowlist entries. With Helm and
Python's PyYAML installed, run from the repository root:

```sh
binary_dir="$(mktemp -d)"
for controller in kyverno background-controller reports-controller cleanup-controller; do
  go build -o "$binary_dir/$controller" "./cmd/$controller"
done
python3 scripts/check-registry-egress-helm.py --bin-dir "$binary_dir" --helm .tools/helm
```

## Reproduce blocked CEL image lookups

The [CEL image metadata proof](../../test/conformance/chainsaw/validating-policies/context/registry-egress-hard-block/README.md) checks blocked metadata and loopback image references in a cluster. The [ImageValidatingPolicy proof](../../test/conformance/chainsaw/image-validating-policies/standard/registry-egress-hard-block/README.md) can also run through the standalone CLI. The unit tests also cover private-address audit compatibility, allowlisted enforce mode, cancellation, redirects, proxy forwarding, and dual-stack fallback.
