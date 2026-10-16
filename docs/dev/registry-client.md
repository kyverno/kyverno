# Registry client ownership

The Kyverno SDK owns registry reference parsing, credential-provider composition,
Secret credential refresh, request option construction, pullers/pushers and
descriptor fetching. `pkg/registryclient` adapts Kyverno's controller and policy
configuration to the SDK rather than implementing those operations locally.

Kyverno supplies its shared egress transport through `WithTransport` and wraps
the SDK's completed credential chain through `WithKeychainWrapper`. The guard
runs before credential resolution, including credentials supplied by individual
policies. Namespace authorization remains the responsibility of Kyverno's Secret
lister. The SDK passes the request context to `GetWithContext` when that lister
supports live reads, while ordinary informer listers retain their cached lookup.

CEL image loaders use the SDK's `NewRemoteOptions`, then bind the evaluation
context for each request. They do not retain a puller or pusher initialized for an
earlier evaluation. Registry connections share the transport associated with the
operator's egress settings; credentials are resolved per request.

Reusable transport, credential-selection and Secret-refresh regressions belong
in the SDK. Kyverno retains integration checks for its egress settings, guard
ordering, policy credential backend, and evaluation cancellation.

The separate Azure provider exchange uses its existing SDK/Flux client. Custom
client support for that provider remains a distinct upstream dependency.
