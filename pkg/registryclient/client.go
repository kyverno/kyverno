package registryclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/tracing"
	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/kyverno/sdk/extensions/regcreds"
	sdkregistryclient "github.com/kyverno/sdk/extensions/registryclient"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// Client is the registry client contract shared with the Kyverno SDK.
type Client = sdkregistryclient.Client

type options struct {
	secretLister             corev1listers.SecretLister
	defaultNamespace         string
	imagePullSecrets         []string
	credentialHelpers        []string
	keychain                 authn.Keychain
	privateRegistryAllowlist []string
	egressMode               string
	allowInsecureRegistry    bool
}

// Option configures a registry client.
type Option func(*options)

// WithSecretLister configures the source for Kubernetes image pull secrets.
func WithSecretLister(lister corev1listers.SecretLister, defaultNamespace string) Option {
	return func(o *options) {
		o.secretLister = lister
		o.defaultNamespace = defaultNamespace
	}
}

// WithImagePullSecrets configures Kubernetes image pull secrets.
func WithImagePullSecrets(secrets ...string) Option {
	return func(o *options) { o.imagePullSecrets = append(o.imagePullSecrets, secrets...) }
}

// WithCredentialHelpers configures registry credential helpers.
func WithCredentialHelpers(providers ...string) Option {
	return func(o *options) { o.credentialHelpers = append(o.credentialHelpers, providers...) }
}

// WithAllowInsecureRegistry allows plain HTTP registry access.
func WithAllowInsecureRegistry(allow bool) Option {
	return func(options *options) {
		options.allowInsecureRegistry = allow
	}
}

// WithKeychain layers a keychain ahead of the configured credential sources.
func WithKeychain(keychain authn.Keychain) Option {
	return func(o *options) {
		if keychain == nil {
			return
		}
		if o.keychain == nil {
			o.keychain = keychain
		} else {
			o.keychain = authn.NewMultiKeychain(keychain, o.keychain)
		}
	}
}

// WithPrivateRegistryAllowlist permits private destinations matching exact hosts,
// IP literals, or CIDRs in enforce mode. Hard-blocked destinations are never
// exempted, and every direct connection is pinned to validated addresses.
func WithPrivateRegistryAllowlist(hosts ...string) Option {
	return func(options *options) {
		options.privateRegistryAllowlist = append(options.privateRegistryAllowlist, hosts...)
	}
}

// WithEgressMode selects audit (the compatibility default) or enforce mode.
func WithEgressMode(mode string) Option {
	return func(options *options) { options.egressMode = mode }
}

// ValidateEgressConfig validates operator settings before controllers start.
func ValidateEgressConfig(allowlist []string, mode string) error {
	_, err := egress.New(egress.Config{Allowlist: allowlist, Mode: mode})
	return err
}

type client struct {
	keychain              authn.Keychain
	transport             http.RoundTripper
	policy                *egress.Policy
	egressConfig          egress.Config
	configErr             error
	allowInsecureRegistry bool
}

type sharedEgress struct {
	policy    *egress.Policy
	transport *egress.Transport
}

var (
	transportMu sync.Mutex
	transports  = make(map[string]*sharedEgress)
)

// Pools are keyed only by operator egress settings, never credentials. All clients
// for the same settings share connections, including clients created per admission.
func sharedTransport(config egress.Config) (*sharedEgress, error) {
	if config.Mode == "" {
		config.Mode = "audit"
	}
	config.Allowlist = slices.Clone(config.Allowlist)
	slices.Sort(config.Allowlist)
	config.Allowlist = slices.Compact(config.Allowlist)
	key, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	transportMu.Lock()
	defer transportMu.Unlock()
	if shared := transports[string(key)]; shared != nil {
		return shared, nil
	}
	policy, err := egress.New(config)
	if err != nil {
		return nil, err
	}
	shared := &sharedEgress{policy: policy, transport: policy.WrapTransport(regcreds.DefaultTransport)}
	transports[string(key)] = shared
	return shared, nil
}

type invalidTransport struct{ err error }

func (t invalidTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Body != nil {
		_ = request.Body.Close()
	}
	return nil, t.err
}

// New creates a registry client with network egress and credential confinement.
// Invalid programmatic settings fail closed; controllers validate them at startup.
func New(opts ...Option) Client {
	var config options
	for _, option := range opts {
		if option != nil {
			option(&config)
		}
	}
	var keychains []authn.Keychain
	if len(config.imagePullSecrets) != 0 && config.secretLister != nil {
		keychains = append(keychains, NewSecretsKeychain(config.secretLister, config.defaultNamespace, config.imagePullSecrets...))
	}
	keychains = append(keychains, keychainsForProviders(config.credentialHelpers...)...)
	var keychain authn.Keychain = authn.DefaultKeychain
	if len(keychains) != 0 {
		keychain = authn.NewMultiKeychain(keychains...)
	}
	if config.keychain != nil {
		keychain = authn.NewMultiKeychain(config.keychain, keychain)
	}
	c := &client{
		allowInsecureRegistry: config.allowInsecureRegistry,
		egressConfig:          egress.Config{Allowlist: slices.Clone(config.privateRegistryAllowlist), Mode: config.egressMode},
	}
	shared, err := sharedTransport(c.egressConfig)
	c.configErr = err
	if err != nil {
		c.transport = invalidTransport{err: err}
	} else {
		c.policy, c.transport = shared.policy, shared.transport
	}
	c.keychain = &guardedKeychain{inner: keychain, policy: c.policy, configErr: err}
	return c
}

// EgressTransport shares the configured network guard with Sigstore HTTP clients.
// It carries no registry credentials and never mutates http.DefaultTransport.
func EgressTransport() http.RoundTripper {
	if configured, ok := globalClient.(*client); ok {
		return configured.transport
	}
	shared, err := sharedTransport(egress.Config{})
	if err != nil {
		return invalidTransport{err: err}
	}
	return shared.transport
}

// GuardKeychain applies the process egress policy to additional credential sources,
// such as a policy's separate signature repository credentials.
func GuardKeychain(inner authn.Keychain) authn.Keychain {
	if configured, ok := globalClient.(*client); ok {
		return &guardedKeychain{inner: inner, policy: configured.policy, configErr: configured.configErr}
	}
	shared, err := sharedTransport(egress.Config{})
	if err != nil {
		return &guardedKeychain{inner: inner, configErr: err}
	}
	return &guardedKeychain{inner: inner, policy: shared.policy}
}

// EgressHTTPClient guards each request, including redirects, with bounded latency.
func EgressHTTPClient() *http.Client {
	return &http.Client{Transport: EgressTransport(), Timeout: 30 * time.Second}
}

type guardedKeychain struct {
	inner     authn.Keychain
	policy    *egress.Policy
	configErr error
}

var _ authn.ContextKeychain = (*guardedKeychain)(nil)

func (g *guardedKeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	return g.ResolveContext(context.Background(), resource)
}

func (g *guardedKeychain) ResolveContext(ctx context.Context, resource authn.Resource) (authn.Authenticator, error) {
	if g.configErr != nil {
		return nil, g.configErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	validationCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, err := g.policy.ResolveAndValidate(validationCtx, resource.RegistryStr())
	cancel()
	if err != nil {
		return nil, err
	}
	return authn.Resolve(ctx, g.inner, resource)
}

func (c *client) imageDataOptions() ([]gcrremote.Option, []name.Option) {
	opts := []gcrremote.Option{
		gcrremote.WithAuthFromKeychain(c.keychain),
		gcrremote.WithTransport(tracing.Transport(c.transport, otelhttp.WithFilter(tracing.RequestFilterIsInSpan))),
		gcrremote.WithUserAgent(regcreds.KyvernoUserAgent),
	}
	return opts, c.NameOptions()
}

func (c *client) optionsWithoutPuller(ctx context.Context) ([]gcrremote.Option, []name.Option) {
	opts, names := c.imageDataOptions()
	return append(opts, gcrremote.WithContext(ctx)), names
}

// Options returns go-containerregistry options for this client.
func (c *client) Options(ctx context.Context) ([]gcrremote.Option, []name.Option, error) {
	if c.configErr != nil {
		return nil, nil, c.configErr
	}
	opts, nameOpts := c.optionsWithoutPuller(ctx)
	pusher, err := gcrremote.NewPusher(opts...)
	if err != nil {
		return nil, nil, err
	}
	opts = append(opts, gcrremote.Reuse(pusher))
	puller, err := gcrremote.NewPuller(opts...)
	if err != nil {
		return nil, nil, err
	}
	return append(opts, gcrremote.Reuse(puller)), nameOpts, nil
}

// NameOptions returns parsing options for registry references.
func (c *client) NameOptions() []name.Option {
	if c.allowInsecureRegistry {
		return []name.Option{name.Insecure}
	}
	return nil
}

// Keychain returns the credential-confined keychain.
func (c *client) Keychain() authn.Keychain {
	return c.keychain
}

// FetchImageDescriptor fetches a registry descriptor through the guarded transport.
func (c *client) FetchImageDescriptor(ctx context.Context, imageRef string) (*gcrremote.Descriptor, error) {
	parsedRef, err := name.ParseReference(imageRef, c.NameOptions()...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference: %s, error: %w", imageRef, err)
	}
	opts, _, err := c.Options(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get registry options: %s, error: %w", imageRef, err)
	}
	descriptor, err := gcrremote.Get(parsedRef, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image reference: %s, error: %w", imageRef, err)
	}
	if _, ok := parsedRef.(name.Digest); ok && parsedRef.Identifier() != descriptor.Digest.String() {
		return nil, fmt.Errorf("digest mismatch, expected: %s, received: %s", parsedRef.Identifier(), descriptor.Digest.String())
	}
	return descriptor, nil
}

var (
	globalClient Client
	globalOnce   sync.Once
)

// SetupGlobalRegistryClient initializes the process-wide guarded registry client.
func SetupGlobalRegistryClient(
	secretLister corev1listers.SecretLister,
	defaultNamespace string,
	imagePullSecrets string,
	credentialHelpers string,
	allowInsecure bool,
	privateRegistryAllowlist string,
	egressMode string,
) Client {
	globalOnce.Do(func() {
		var allowlist []string
		if privateRegistryAllowlist != "" {
			allowlist = strings.Split(privateRegistryAllowlist, ",")
		}
		opts := []Option{
			WithSecretLister(secretLister, defaultNamespace),
			WithPrivateRegistryAllowlist(allowlist...),
			WithEgressMode(egressMode),
		}
		if secrets := splitList(imagePullSecrets); len(secrets) != 0 {
			opts = append(opts, WithImagePullSecrets(secrets...))
		}
		if helpers := splitList(credentialHelpers); len(helpers) != 0 {
			opts = append(opts, WithCredentialHelpers(helpers...))
		}
		if allowInsecure {
			opts = append(opts, WithAllowInsecureRegistry(true))
		}
		globalClient = New(opts...)
	})
	return globalClient
}

// GlobalOptsOrDefault returns guarded options from the global client or secure defaults.
func GlobalOptsOrDefault(ctx context.Context) ([]gcrremote.Option, []name.Option) {
	if configured, ok := globalClient.(*client); ok {
		return configured.optionsWithoutPuller(ctx)
	}
	return New().(*client).optionsWithoutPuller(ctx)
}

// GlobalImageDataOptions returns reusable options without fixing a context at
// compile time. The image data loader supplies the admission request context.
func GlobalImageDataOptions() ([]gcrremote.Option, []name.Option) {
	if configured, ok := globalClient.(*client); ok {
		return configured.imageDataOptions()
	}
	return New().(*client).imageDataOptions()
}

// ImageDataOptsFromImageVerificationCredentials keeps credentials refreshable and
// lets the image data loader bind each request's cancellation and deadline.
func ImageDataOptsFromImageVerificationCredentials(lister corev1listers.SecretLister, credentials policiesv1alpha1.Credentials, defaultNamespace string) ([]gcrremote.Option, []name.Option) {
	return imageVerificationClient(lister, credentials, defaultNamespace).imageDataOptions()
}

// OptsFromImageVerificationCredentials builds guarded registry options for
// credentials embedded in an ImageValidatingPolicy.
func OptsFromImageVerificationCredentials(
	ctx context.Context,
	lister corev1listers.SecretLister,
	credentials policiesv1alpha1.Credentials,
	defaultNamespace string,
) ([]gcrremote.Option, []name.Option) {
	return imageVerificationClient(lister, credentials, defaultNamespace).optionsWithoutPuller(ctx)
}

func imageVerificationClient(lister corev1listers.SecretLister, credentials policiesv1alpha1.Credentials, defaultNamespace string) *client {
	providers := make([]string, len(credentials.Providers))
	for i, provider := range credentials.Providers {
		providers[i] = string(provider)
	}
	privateAllowlist := []string(nil)
	egressMode := ""
	if configured, ok := globalClient.(*client); ok {
		privateAllowlist = configured.egressConfig.Allowlist
		egressMode = configured.egressConfig.Mode
	}
	// Explicit policy Secrets require an available credential lookup backend.
	var requiredSecrets authn.Keychain
	if lister == nil && len(credentials.Secrets) != 0 {
		requiredSecrets = NewSecretsKeychain(nil, defaultNamespace, credentials.Secrets...)
	}
	configured := New(
		WithKeychain(requiredSecrets),
		WithSecretLister(lister, defaultNamespace),
		WithImagePullSecrets(credentials.Secrets...),
		WithCredentialHelpers(providers...),
		WithAllowInsecureRegistry(credentials.AllowInsecureRegistry),
		WithPrivateRegistryAllowlist(privateAllowlist...),
		WithEgressMode(egressMode),
	).(*client)
	return configured
}

// GetRegistryClient returns the configured global client.
func GetRegistryClient() (Client, error) {
	if globalClient == nil {
		return nil, fmt.Errorf("registry client wasn't initialized")
	}
	return globalClient, nil
}

// MustRegistryClient returns the configured global client or panics.
func MustRegistryClient() Client {
	client, err := GetRegistryClient()
	if err != nil {
		panic(err)
	}
	return client
}

func splitList(value string) []string {
	var values []string
	for item := range strings.SplitSeq(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}
