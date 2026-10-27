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
	policyClient          Client
	transport             http.RoundTripper
	policy                *egress.Policy
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
	keychains = append(keychains, regcreds.KeychainsForProviders(config.credentialHelpers...)...)
	var keychain authn.Keychain = authn.DefaultKeychain
	if len(keychains) != 0 {
		keychain = authn.NewMultiKeychain(keychains...)
	}
	if config.keychain != nil {
		keychain = authn.NewMultiKeychain(config.keychain, keychain)
	}
	c := &client{
		allowInsecureRegistry: config.allowInsecureRegistry,
		policyClient: sdkregistryclient.New(
			sdkregistryclient.WithKeychain(keychain),
			sdkregistryclient.WithAllowInsecureRegistry(config.allowInsecureRegistry),
		),
	}
	shared, err := sharedTransport(egress.Config{Allowlist: config.privateRegistryAllowlist, Mode: config.egressMode})
	c.configErr = err
	if err != nil {
		c.transport = invalidTransport{err: err}
	} else {
		c.policy, c.transport = shared.policy, shared.transport
	}
	c.keychain = &guardedKeychain{inner: keychain, policy: c.policy, configErr: err}
	return c
}

// PolicyClient returns a separately constructed native client for policy-authored
// destinations, without resource-image destination or credential restrictions.
func (c *client) PolicyClient() Client {
	return c.policyClient
}

// ValidateImage checks a resource-image destination without fetching its content.
func (c *client) ValidateImage(ctx context.Context, imageRef string) error {
	if c.configErr != nil {
		return c.configErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	reference, err := name.ParseReference(imageRef, c.NameOptions()...)
	if err != nil {
		return err
	}
	validationCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = c.policy.ResolveAndValidate(validationCtx, reference.Context().RegistryStr())
	return err
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
