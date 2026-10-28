# Workload image verification egress

The guard applies to untrusted image references from admitted resources, including
container, initContainer, ephemeralContainer, nested workload template, and custom
image-extractor fields. CEL `image.getMetadata` calls on known workload values and
ImageValidatingPolicy image metadata, signature/attestation verification, and digest
mutation use guarded registry options for those references.
Resource destinations are checked before verification-cache hits. Resource image data
uses a separate request-local cache so policy-authored preloads cannot bypass the guard.

Policy authors already have the CEL HTTP library. CEL image metadata/data/context
lookups of author-chosen references (rather than workload values), signature repository overrides (`source.Repository`),
key and certificate URLs, Rekor, Fulcio, TSA, and TUF/trust URLs retain native upstream
behavior. This is not a general policy-author or process-wide network restriction.

Resource recognition reads built-in image fields and resource-field paths returned by
custom image extractors, including variable aliases and collection projections. It
does not scan unrelated resource strings or evaluate extractors an extra time.
Matching text in labels, annotations, or other unrelated fields does not turn a
policy-authored reference into a workload image. Arbitrary policy transformations
are not treated as resource provenance.

## Registry requests and credentials

For resource images, the guard covers the initial registry, registry token challenges,
redirects, and subsequent signature/referrer downloads. It validates the destination
before invoking credential providers. Explicit policy signature repositories use native
options and their configured credentials instead of inheriting the resource guard.

Direct connections are pinned to validated addresses. DNS resolution and dialing share
a five-second budget, shortened by the request deadline. Credential Secrets remain
refreshable and evaluation cancellation propagates to registry requests.

Provider-internal cloud credential exchanges (including the SDK/Flux Azure client) and
KMS networking retain their provider clients; the registry guard is not injected there.

## Rollout

`privateRegistryEgressMode` defaults to `audit`: ordinary private registry addresses
remain reachable and enforce-mode violations are logged at verbosity 2. Both modes
reject metadata, link-local, loopback, unspecified, multicast, and broadcast addresses
for resource images, even when allowlisted. In `enforce`, private destinations require
an operator allowlist entry. Policy-authored endpoints are unaffected in either mode.

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

Include private registry token and redirect services. Entries must be exact hosts,
IP literals, or CIDRs; wildcards, URLs, empty elements, and commas inside entries are
invalid. Use `[]` for no exceptions. Invalid configuration fails controller startup.
The equivalent flags are `--privateRegistryEgressMode` and `--privateRegistryAllowlist`.
Admission, background, reports, and cleanup controllers support these options. Cleanup
applies them to DeletingPolicy metadata lookups of workload images without enabling
registry credential helpers or image-pull Secret informers. Its settings can be overridden under
`cleanupController.featuresOverride.registryClient`. Author-chosen image references
remain outside the guard in cleanup, just as in the other controllers.

Proxies retain the existing environment configuration, which is treated as trusted
operator configuration. Ordinary private proxy addresses do not require an allowlist
entry in either `audit` or `enforce`, but hard blocks still apply to proxy endpoints.
Proxy endpoints and downstream workload destinations are validated separately;
using a proxy does not exempt the target registry from egress restrictions, including
the allowlist requirement for private targets in `enforce`. The proxy itself remains
responsible for downstream DNS, so operators must trust its routing and resolver
configuration.
