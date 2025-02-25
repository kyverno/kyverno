package registryclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"runtime"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/release-utils/version"
)

var (
	defaultKeychain  = imagedataloader.AnonymousKeychain
	defaultTransport = imagedataloader.DefaultTransport
	userAgent        = fmt.Sprintf("Kyverno/%s (%s; %s)", version.GetVersionInfo().GitVersion, runtime.GOOS, runtime.GOARCH)
)

// Client provides registry related objects.
type Client interface {
	// Keychain provides the configured credentials
	Keychain() authn.Keychain

	// getTransport provides transport object.
	getTransport() http.RoundTripper

	// FetchImageDescriptor fetches Descriptor from registry with given imageRef
	// and provides access to metadata about remote artifact.
	FetchImageDescriptor(context.Context, string) (*gcrremote.Descriptor, error)

	// Options returns remote.Option configuration for the client.
	Options(context.Context) ([]gcrremote.Option, error)

	// NameOptions returns name.Option configuration for the client.
	NameOptions() []name.Option
}

type client struct {
	keychain              authn.Keychain
	transport             http.RoundTripper
	allowInsecureRegistry bool
}

type config struct {
	keychain              []authn.Keychain
	transport             *http.Transport
	tracing               bool
	allowInsecureRegistry bool
}

// Option is an option to initialize registry client.
type Option = func(*config) error

// New creates a new Client with options
func New(options ...Option) (Client, error) {
	cfg := &config{
		transport: defaultTransport,
	}
	for _, opt := range options {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}
	c := &client{
		keychain:  defaultKeychain,
		transport: cfg.transport,
	}
	if len(cfg.keychain) > 0 {
		c.keychain = authn.NewMultiKeychain(cfg.keychain...)
	}
	if cfg.tracing {
		c.transport = tracing.Transport(cfg.transport, otelhttp.WithFilter(tracing.RequestFilterIsInSpan))
	}
	if cfg.allowInsecureRegistry {
		c.allowInsecureRegistry = true
	}
	return c, nil
}

// New creates a new Client with options
func NewOrDie(options ...Option) Client {
	c, err := New(options...)
	if err != nil {
		panic(err)
	}
	return c
}

// WithKeychainPullSecrets provides initialize registry client option that allows to use pull secrets.
func WithKeychainPullSecrets(lister corev1listers.SecretNamespaceLister, imagePullSecrets ...string) Option {
	return func(c *config) error {
		kc, err := NewAutoRefreshSecretsKeychain(lister, imagePullSecrets...)
		if err != nil {
			return err
		}
		c.keychain = append(c.keychain, kc)
		return nil
	}
}

// WithCredentialProviders initialize registry client option by using registries credentials
func WithCredentialProviders(credentialProviders ...string) Option {
	return func(c *config) error {
		chains := imagedataloader.KeychainsForProviders(credentialProviders...)
		c.keychain = append(c.keychain, chains...)
		return nil
	}
}

// WithAllowInsecureRegistry initialize registry client option that allows to use insecure registries.
func WithAllowInsecureRegistry() Option {
	return func(c *config) error {
		c.transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
		c.allowInsecureRegistry = true
		return nil
	}
}

// WithLocalKeychain provides initialize keychain with the default local keychain.
func WithLocalKeychain() Option {
	return func(c *config) error {
		c.keychain = append(c.keychain, authn.DefaultKeychain)
		return nil
	}
}

// WithTracing enables tracing in the http client.
func WithTracing() Option {
	return func(c *config) error {
		c.tracing = true
		return nil
	}
}

// Options returns remote.Option config parameters for the client
func (c *client) Options(ctx context.Context) ([]gcrremote.Option, error) {
	opts := []gcrremote.Option{
		gcrremote.WithAuthFromKeychain(c.keychain),
		gcrremote.WithTransport(c.transport),
		gcrremote.WithContext(ctx),
		gcrremote.WithUserAgent(userAgent),
	}

	pusher, err := gcrremote.NewPusher(opts...)
	if err != nil {
		return nil, err
	}
	opts = append(opts, gcrremote.Reuse(pusher))

	puller, err := gcrremote.NewPuller(opts...)
	if err != nil {
		return nil, err
	}
	opts = append(opts, gcrremote.Reuse(puller))

	return opts, nil
}

// NameOptions returns name.Option config parameters for the client
func (c *client) NameOptions() []name.Option {
	nameOpts := []name.Option{}

	if c.allowInsecureRegistry {
		nameOpts = append(nameOpts, name.Insecure)
	}

	return nameOpts
}

// FetchImageDescriptor fetches Descriptor from registry with given imageRef
// and provides access to metadata about remote artifact.
func (c *client) FetchImageDescriptor(ctx context.Context, imageRef string) (*gcrremote.Descriptor, error) {
	nameOpts := c.NameOptions()
	parsedRef, err := name.ParseReference(imageRef, nameOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference: %s, error: %v", imageRef, err)
	}
	remoteOpts, err := c.Options(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gcr remote opts: %s, error: %v", imageRef, err)
	}
	desc, err := gcrremote.Get(parsedRef, remoteOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image reference: %s, error: %v", imageRef, err)
	}
	if _, ok := parsedRef.(name.Digest); ok && parsedRef.Identifier() != desc.Digest.String() {
		return nil, fmt.Errorf("digest mismatch, expected: %s, received: %s", parsedRef.Identifier(), desc.Digest.String())
	}
	return desc, nil
}

func (c *client) Keychain() authn.Keychain {
	return c.keychain
}

func (c *client) getTransport() http.RoundTripper {
	return c.transport
}
