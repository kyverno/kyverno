# Workload image registry egress

## Scope and trust boundary

ImageValidatingPolicy guards registry destinations taken from untrusted workload
images: `containers`, `initContainers`, `ephemeralContainers`, Pod controller
and CronJob templates, and resource image references selected by image extractors.
The same guard applies during digest mutation and subsequent verification reads.
Well-known workload image references are recognized before match conditions,
variables and validations, including evaluation without GVR-derived extractors.
Workload image values consumed by the CEL `image.GetMetadata` library are guarded
too, using current and old resource objects; an author-selected literal on the
same registry remains native when it is outside the workload image set.

Policy authors are trusted to choose network destinations and already have HTTP
access. These settings do **not** restrict author-selected CEL `image()` calls,
image-data/network/context lookups, or image references outside the resource image
set. Policy-authored key/certificate URLs, Rekor, Fulcio, TSA, trusted roots and
TUF endpoints retain native upstream behavior; no custom Sigstore/TUF HTTP client,
cache migration or trust-material implementation is installed.

An explicit signature repository/source override is author-controlled and uses
native options, even when verifying a guarded workload image. The workload image
itself is still fetched through the guard; verification uses its resolved digest
without re-fetching its tag using native options. Without an explicit repository
override, signature, attestation and referrer registry reads remain guarded.
Author and resource image data use separate caches to prevent author-preloaded
data or native remote options from crossing this boundary.

## Modes and configuration

The default `privateRegistryEgressMode` is `audit`. Ordinary private addresses
(RFC1918, IPv6 unique-local and carrier-grade NAT) remain reachable, while
verbosity-2 logs identify requests `enforce` would reject. `enforce` requires an
allowlist entry for those private destinations. Both modes always block metadata,
link-local, loopback, unspecified, multicast and broadcast addresses. Allowlist
entries never exempt those hard blocks. Invalid settings fail controller startup.

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

Equivalent flags are `--privateRegistryEgressMode=enforce` and
`--privateRegistryAllowlist=registry.corp.example,auth.corp.example,10.20.30.0/24,fd12:3456:789a::/64`.
Entries accept exact hostnames, IP addresses and CIDRs, not wildcard patterns,
URLs or empty entries. A hostname permission applies across ports. Use `[]` for
an empty Helm list, with one non-empty, comma-free item per entry.

Resource registry token services and redirect destinations are checked on every
request and need their own private-address allowlist entries in enforce mode.
Allowing a registry never grants another destination its credentials. These
checks are not applied to endpoints chosen by an explicit author repository
or verification-service override.

Audit mode preserves private connectivity; it is not protection against access
to arbitrary ordinary private addresses. Exercise workload verification, review
logs, add required registry/token/redirect destinations, then enable enforce.
Admission, background, reports and cleanup controllers accept these settings,
including `featuresOverride.registryClient`. Cleanup/DeletingPolicy author calls
remain native; only workload-origin lookups consume these settings. Cleanup does
not enable registry credential helpers or image-pull Secret informers.

## DNS, proxies and credential helpers

Direct connections are pinned to validated IP addresses. DNS and connection
attempts share a five-second budget, shortened by the admission deadline.
An operator HTTP(S) proxy can use an ordinary private address without an
allowlist entry, but hard-blocked proxy addresses remain forbidden. The target
is checked independently; a private proxy does not exempt a private target.
An HTTP proxy resolves upstream hostnames itself, so enforce matching DNS and
egress controls on that trusted proxy. `NO_PROXY` keeps native behavior.

The guard runs before workload registry credential resolution. Azure's internal
SDK/Flux credential exchange uses its existing client and is outside this
transport; this is not a process-wide network policy.

## Validation

Render the example with `helm template kyverno charts/kyverno --namespace kyverno
-f docs/user/registry-egress/enforce-values.yaml`. To check chart arguments without
a cluster, run `python3 scripts/check-registry-egress-helm.py --render-only` with
Helm and PyYAML available.

The conformance proof at
`test/conformance/chainsaw/image-validating-policies/standard/registry-egress-hard-block`
explicitly extracts all three container categories before fetching image data.
It rejects metadata/loopback workload images and preserves unmatched Pods.
Run `python3 scripts/check-ivpol-registry-egress.py --cli /path/to/kubectl-kyverno`
with a CLI built from this checkout, or run that directory with Chainsaw.
Focused Go regressions also cover early callbacks, custom resource image fields,
cache isolation, native author lookups and native signature repository overrides.
