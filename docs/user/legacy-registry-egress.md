# Legacy resource-image registry egress configuration

Legacy `kyverno.io` image verification guards registry destinations derived from
untrusted workload images, including `containers`, `initContainers`,
`ephemeralContainers`, Pod controller templates, and configured resource image
extractors. The guard applies before registry requests and credential resolution.
Registry token services and redirects reached while fetching these resource
images are guarded as well.

Policy authors already have HTTP access through policy context. Policy-authored
`imageRegistry` context destinations and explicit image signature repository
settings are therefore outside this guard, even when their values use policy
variables. Public-key and certificate references, Rekor, Fulcio, CT, TSA, TUF,
and manifest/bundle verification retain their upstream native clients and trust
behavior. They do not use this registry allowlist or egress mode. This is not a
process-wide network policy.

An explicit image signature repository does not exempt the resource image: its
destination is checked and tags are resolved through the guarded client before
using the policy-selected signature repository with a pinned digest. Digest-only
references keep native verification behavior without fetching image content.

Direct resource-image registry connections use validated IP addresses, including
when the registry is allowlisted, preventing a second DNS lookup from replacing
the destination. DNS resolution and dial attempts share a five-second deadline,
shortened by the request deadline. IPv4 and IPv6 attempts can race for fallback.

Azure credential exchange uses the SDK's official Azure Container Registry
provider checks, including sovereign clouds. Other cloud credential bootstrap
calls retain their provider SDK networking.

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

Audit mode preserves private network connectivity; it does not prevent workload
image values from selecting arbitrary private addresses in those permitted ranges. Operators
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
across ports. Each Helm list item must contain one entry; embedded commas are
rejected instead of being interpreted as additional entries. Wildcards, URLs,
paths, malformed addresses, and empty list elements are also rejected. Use the
narrowest practical hostnames or subnets. Public registries do not need allowlist
entries.

Registry credentials remain scoped to their configured registry. Permitting a
network destination does not give that destination another registry's credentials.
Private token services and redirect destinations reached from workload image
registries need their own allowlist entries in enforce mode; a registry entry does
not automatically permit them. Policy-authored endpoints need no entries.

1. Upgrade with the default audit mode and normal verbosity 2 logging.
2. Exercise workload image verification and background scans using your registries.
3. Review audit logs and add the private resource-image registries, token services,
   and redirect destinations that these operations require.
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
strings (including whitespace-only entries), non-string entries, and entries
containing commas before serializing the list to a controller flag. This also
applies to controller-specific overrides.

To check rendered arguments and rejected values without building controller
binaries, run:

```sh
python3 scripts/check-registry-egress-helm.py --render-only --helm .tools/helm
```

A local regression check renders both modes and each supported controller override,
then passes the rendered arguments to the corresponding executable with `--help`.
This checks the actual flag parsers without starting controllers or contacting a
cluster. It also checks that Helm rejects invalid allowlist entries. With Helm
and Python's PyYAML installed, run from the repository root:

```sh
binary_dir="$(mktemp -d)"
for controller in kyverno background-controller reports-controller cleanup-controller; do
  go build -o "$binary_dir/$controller" "./cmd/$controller"
done
python3 scripts/check-registry-egress-helm.py --bin-dir "$binary_dir" --helm .tools/helm
```
