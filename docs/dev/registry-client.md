# Registry client ownership

The Kyverno SDK owns registry parsing, credential composition, Secret refresh,
request options and descriptor fetching. `pkg/registryclient` adapts controller
and policy configuration without replacing those operations.

## Trust boundary

General registry clients and author-selected CEL `image()`/image-data/context
references use native SDK options. Recognized workload image references use
scoped options instead. Policy authors can already make HTTP requests; registry
egress settings are not a policy-author sandbox.

`image.GetMetadata` also recognizes workload container references from current
and old objects. Its scoped wrapper validates before fetching, appends a guarded
transport, and preserves policy-specific authentication options. An author-chosen
reference on the same registry remains native unless it is a workload image.

Cleanup retains egress-only operator configuration for workload-origin lookups,
without enabling registry credential helpers or Secret informers.

`ResourceImageOptions` and `ResourceImageOptionsFromCredentials` are separate
builders for workload-origin image requests only. They install the egress
transport and wrap the SDK credential chain before credential resolution. They
use `NewRemoteOptions`, never `Client.Options` plus an appended transport: SDK
`Options` may retain pullers/pushers created with the original transport.

IVPOL binds well-known workload container references before match conditions,
variables and validations, even without a request GVR. Extracted resource image
references extend that set before prefetch and verification. The resource cache
is separate from the author cache; preloaded author data cannot replace guarded
resource data. Resource `ImageData` retains guarded remote options for manifests,
configs, signatures, attestations and referrers, including token/redirect targets.
The scoped cache belongs to one policy evaluation rather than being shared
across policy credentials.

An explicitly configured Cosign signature repository uses native author options,
including its own credentials and token/redirect requests. Verification uses the
already-fetched resource digest, avoiding native re-resolution of the workload
tag. A default signature repository continues to use resource options. Sigstore,
TUF, key/certificate URLs, Rekor, Fulcio and TSA retain upstream native behavior.

Image loaders bind request context at evaluation time; credentials remain
refreshable. Namespace authorization stays with the Secret lister. The Azure
provider's internal SDK/Flux credential exchange remains outside this transport.
