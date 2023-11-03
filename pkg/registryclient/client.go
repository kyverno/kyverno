package registryclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/tracing"
	"github.com/sigstore/cosign/v2/pkg/oci/remote"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/release-utils/version"
)

var (
	defaultKeychain  = AnonymousKeychain
	defaultTransport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			// By default we wrap the transport in retries, so reduce the
			// default dial timeout to 5s to avoid 5x 30s of connection
			// timeouts when doing the "ping" on certain http registries.
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	userAgent = fmt.Sprintf("cosign/%s (%s; %s)", version.GetVersionInfo().GitVersion, runtime.GOOS, runtime.GOARCH)
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

	// BuildCosignRemoteOption builds remote.Option for cosign client.
	BuildCosignRemoteOption(context.Context) (remote.Option, error)

	// BuildGCRRemoteOption builds []gcrremote.option based on client.
	BuildGCRRemoteOption(ctx context.Context) ([]gcrremote.Option, error)
}

type client struct {
	keychain  authn.Keychain
	transport http.RoundTripper
}

type config struct {
	keychain  []authn.Keychain
	transport *http.Transport
	tracing   bool
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
		var chains []authn.Keychain
		helpers := sets.New(credentialProviders...)
		if helpers.Has("default") {
			chains = append(chains, authn.DefaultKeychain)
		}
		if helpers.Has("google") {
			chains = append(chains, GCPKeychain)
		}
		if helpers.Has("amazon") {
			chains = append(chains, AWSKeychain)
		}
		if helpers.Has("azure") {
			chains = append(chains, AzureKeychain)
		}
		if helpers.Has("github") {
			chains = append(chains, github.Keychain)
		}
		c.keychain = append(c.keychain, chains...)
		return nil
	}
}

// WithAllowInsecureRegistry initialize registry client option that allows to use insecure registries.
func WithAllowInsecureRegistry() Option {
	return func(c *config) error {
		c.transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
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

// BuildCosignRemoteOption builds remote.Option for cosign client.
func (c *client) BuildCosignRemoteOption(ctx context.Context) (remote.Option, error) {
	gcrRemoteOpts, err := c.getGCRRemoteOption(ctx)
	if err != nil {
		return nil, err
	}
	gcrRemoteOpts = append(gcrRemoteOpts, gcrremote.WithUserAgent(userAgent))
	return remote.WithRemoteOptions(gcrRemoteOpts...), nil
}

// BuildGCRRemoteOption builds []gcrremote.Option based on client.
func (c *client) BuildGCRRemoteOption(ctx context.Context) ([]gcrremote.Option, error) {
	return c.getGCRRemoteOption(ctx)
}

func (c *client) getGCRRemoteOption(ctx context.Context) ([]gcrremote.Option, error) {
	remoteOpts := []gcrremote.Option{
		gcrremote.WithAuthFromKeychain(c.keychain),
		gcrremote.WithTransport(c.transport),
		gcrremote.WithContext(ctx),
	}

	pusher, err := gcrremote.NewPusher(remoteOpts...)
	if err != nil {
		return nil, err
	}
	remoteOpts = append(remoteOpts, gcrremote.Reuse(pusher))

	puller, err := gcrremote.NewPuller(remoteOpts...)
	if err != nil {
		return nil, err
	}
	remoteOpts = append(remoteOpts, gcrremote.Reuse(puller))
	return remoteOpts, nil
}

// FetchImageDescriptor fetches Descriptor from registry with given imageRef
// and provides access to metadata about remote artifact.
func (c *client) FetchImageDescriptor(ctx context.Context, imageRef string) (*gcrremote.Descriptor, error) {
	parsedRef, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference: %s, error: %v", imageRef, err)
	}
	desc, err := gcrremote.Get(parsedRef, gcrremote.WithAuthFromKeychain(c.keychain), gcrremote.WithContext(ctx))
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
