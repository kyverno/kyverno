# Registry client ownership and scope

The Kyverno SDK owns registry parsing, credential composition and Secret refresh,
remote option construction, pullers/pushers, and descriptor fetching.
`pkg/registryclient` only adapts operator configuration and resource-image egress.

`GlobalImageDataOptions` and `ImageDataOptsFromImageVerificationCredentials` return
native SDK `NewRemoteOptions` for policy-authored CEL lookups. They carry no egress
transport or guarded keychain and bind the evaluation context at runtime.
The CEL image receiver adds scoped guards when a lookup uses a known current/old
resource image value, preserving its passed credentials. Other author-selected
references remain native, even when they use the same registry host.
`ResourceImageOptions` and `ResourceImageOptionsFromCredentials` construct
separate guarded options for resource image verification. Do not append a transport
to SDK `Options` containing `remote.Reuse`: those pullers already own their transport.

The image-verification runtime records built-in image fields and resource-field paths
selected by custom image extractors before CEL match conditions. This includes
container/initContainer/ephemeralContainer images in templates. Canonical reference
aliases share their resource scope; unrelated resource strings and policy literals do
not expand the resource-image set. Extractors are not evaluated an extra time.
`ValidateResourceImage` runs before verification-cache hits. Resource image fetches use
a separate request-local image context, so a native policy preload cannot seed an
unguarded `ImageData` or remote options into resource verification. Callers provide a
fresh-context factory through `WithResourceImageContextFactory`, preserving their
lister and provider configuration. It is invoked only for a validated resource fetch,
not a verification-cache hit; unsupported providers return an error instead of being
silently replaced with a network loader.

The guarded SDK constructor uses `WithTransport` and `WithKeychainWrapper` to protect
registry, token, redirect, and referrer requests, and validate before credentials.
Operator-equivalent settings share connection pools, not credentials or request state.
Secret namespace authorization remains with the supplied lister.

Cosign receives the policy's native remote options separately. An explicit
`source.Repository` replaces resource options before source credentials are applied;
without that override, signature downloads retain the resource guard. Sigstore/TUF,
public-key/certificate, Rekor/Fulcio/TSA, and trust URL handling stays upstream-native.
Provider-internal cloud credential exchanges and KMS clients are unchanged.

Cosign verification uses the already-resolved image digest for bundle enumeration,
signature verification, and attestation verification. Switching to native source options
must not re-resolve a workload tag through the unguarded policy transport.
