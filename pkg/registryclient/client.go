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
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/kyverno/sdk/extensions/regcreds"
	sdkregistryclient "github.com/kyverno/sdk/extensions/registryclient"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// Client is the registry client contract shared with the Kyverno SDK.
type Client = sdkregistryclient.Client

type options struct {
	sdkOptions               []sdkregistryclient.Option
	privateRegistryAllowlist []string
	egressMode               string
}

// Option configures a registry client.
type Option func(*options)

// WithSecretLister configures the source for Kubernetes image pull secrets.
func WithSecretLister(lister corev1listers.SecretLister, defaultNamespace string) Option {
	return func(o *options) {
		o.sdkOptions = append(o.sdkOptions, sdkregistryclient.WithSecretLister(lister, defaultNamespace))
	}
}

// WithImagePullSecrets configures Kubernetes image pull secrets.
func WithImagePullSecrets(secrets ...string) Option {
	return func(o *options) {
		o.sdkOptions = append(o.sdkOptions, sdkregistryclient.WithImagePullSecrets(secrets...))
	}
}

// WithCredentialHelpers configures registry credential helpers.
func WithCredentialHelpers(providers ...string) Option {
	return func(o *options) {
		o.sdkOptions = append(o.sdkOptions, sdkregistryclient.WithCredentialHelpers(providers...))
	}
}

// WithAllowInsecureRegistry allows plain HTTP registry access.
func WithAllowInsecureRegistry(allow bool) Option {
	return func(o *options) {
		o.sdkOptions = append(o.sdkOptions, sdkregistryclient.WithAllowInsecureRegistry(allow))
	}
}

// WithKeychain layers a keychain ahead of the configured credential sources.
func WithKeychain(keychain authn.Keychain) Option {
	return func(o *options) {
		o.sdkOptions = append(o.sdkOptions, sdkregistryclient.WithKeychain(keychain))
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
	Client
	transport    http.RoundTripper
	policy       *egress.Policy
	egressConfig egress.Config
	configErr    error
	imageOptions []gcrremote.Option
	imageNames   []name.Option
	sdkOptions   []sdkregistryclient.Option
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

// New creates a native registry client for policy-author-controlled requests.
func New(opts ...Option) Client {
	var config options
	for _, option := range opts {
		if option != nil {
			option(&config)
		}
	}
	c := &client{
		egressConfig: egress.Config{Allowlist: slices.Clone(config.privateRegistryAllowlist), Mode: config.egressMode},
	}
	shared, err := sharedTransport(c.egressConfig)
	c.configErr = err
	if err != nil {
		c.transport = invalidTransport{err: err}
	} else {
		c.policy, c.transport = shared.policy, shared.transport
	}
	c.sdkOptions = append(config.sdkOptions, sdkregistryclient.WithLogger(logging.GlobalLogger()))
	c.Client = sdkregistryclient.New(c.sdkOptions...)
	c.imageOptions, c.imageNames = sdkregistryclient.NewRemoteOptions(c.sdkOptions...)
	return c
}

func (c *client) resourceImageOptions() ([]gcrremote.Option, []name.Option) {
	opts := append(slices.Clone(c.sdkOptions),
		sdkregistryclient.WithTransport(c.transport),
		sdkregistryclient.WithKeychainWrapper(func(inner authn.Keychain) authn.Keychain {
			return &guardedKeychain{inner: inner, policy: c.policy, configErr: c.configErr}
		}),
	)
	return sdkregistryclient.NewRemoteOptions(opts...)
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
	return slices.Clone(c.imageOptions), slices.Clone(c.imageNames)
}

func (c *client) optionsWithoutPuller(ctx context.Context) ([]gcrremote.Option, []name.Option) {
	opts, names := c.imageDataOptions()
	return append(opts, gcrremote.WithContext(ctx)), names
}

// NewSecretsKeychain delegates request-aware, refreshable credential resolution
// to the SDK. Policy namespace authorization stays with the supplied lister.
func NewSecretsKeychain(lister corev1listers.SecretLister, namespace string, references ...string) authn.Keychain {
	return regcreds.NewSecretsKeychain(lister, namespace, logging.GlobalLogger(), references...)
}

var (
	globalClient Client
	globalOnce   sync.Once
)

// SetupGlobalRegistryClient initializes native clients and workload egress settings.
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

// GlobalOptsOrDefault returns native options for policy-author-controlled requests.
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

// ResourceImageOptions returns guarded options only for workload-origin images.
// It never reuses a native SDK puller or pusher.
func ResourceImageOptions() ([]gcrremote.Option, []name.Option) {
	if configured, ok := globalClient.(*client); ok {
		return configured.resourceImageOptions()
	}
	return New().(*client).resourceImageOptions()
}

// ResourceImageOptionsFromCredentials confines workload image credentials and requests.
func ResourceImageOptionsFromCredentials(lister corev1listers.SecretLister, credentials policiesv1alpha1.Credentials, namespace string) ([]gcrremote.Option, []name.Option) {
	return imageVerificationClient(lister, credentials, namespace).resourceImageOptions()
}

// ValidateResourceImage checks workload destinations before image or verification cache reads.
func ValidateResourceImage(ctx context.Context, image string, names ...name.Option) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	configured, ok := globalClient.(*client)
	if !ok {
		configured = New().(*client)
	}
	if configured.configErr != nil {
		return configured.configErr
	}
	ref, err := name.ParseReference(image, names...)
	if err != nil {
		return err
	}
	validationCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = configured.policy.ResolveAndValidate(validationCtx, ref.Context().RegistryStr())
	return err
}

// ImageDataOptsFromImageVerificationCredentials keeps credentials refreshable and
// lets the image data loader bind each request's cancellation and deadline.
func ImageDataOptsFromImageVerificationCredentials(lister corev1listers.SecretLister, credentials policiesv1alpha1.Credentials, defaultNamespace string) ([]gcrremote.Option, []name.Option) {
	return imageVerificationClient(lister, credentials, defaultNamespace).imageDataOptions()
}

// OptsFromImageVerificationCredentials builds native registry options for
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
