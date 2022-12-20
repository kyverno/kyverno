package registryclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	ecr "github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/chrismellard/docker-credential-acr-env/pkg/credhelper"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/tracing"
	"github.com/sigstore/cosign/pkg/oci/remote"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

var (
	baseKeychain = authn.NewMultiKeychain(
		authn.DefaultKeychain,
		google.Keychain,
		authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithLogger(io.Discard))),
		authn.NewKeychainFromHelper(credhelper.NewACRCredentialsHelper()),
		github.Keychain,
	)
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
)

// Client provides registry related objects.
type Client interface {
	// getKeychain provides keychain object.
	getKeychain() authn.Keychain

	// getTransport provides transport object.
	getTransport() http.RoundTripper

	// FetchImageDescriptor fetches Descriptor from registry with given imageRef
	// and provides access to metadata about remote artifact.
	FetchImageDescriptor(context.Context, string) (*gcrremote.Descriptor, error)

	// BuildRemoteOption builds remote.Option based on client.
	BuildRemoteOption(context.Context) remote.Option
}

type client struct {
	keychain            authn.Keychain
	transport           http.RoundTripper
	pullSecretRefresher func(context.Context, *client) error
}

type config struct {
	keychain            authn.Keychain
	transport           *http.Transport
	pullSecretRefresher func(context.Context, *client) error
	tracing             bool
}

// Option is an option to initialize registry client.
type Option = func(*config) error

// New creates a new Client with options
func New(options ...Option) (Client, error) {
	cfg := &config{
		keychain:  baseKeychain,
		transport: defaultTransport,
	}
	for _, opt := range options {
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}
	c := &client{
		keychain:            cfg.keychain,
		transport:           cfg.transport,
		pullSecretRefresher: cfg.pullSecretRefresher,
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
func WithKeychainPullSecrets(ctx context.Context, lister corev1listers.SecretNamespaceLister, imagePullSecrets ...string) Option {
	return func(c *config) error {
		c.pullSecretRefresher = func(ctx context.Context, c *client) error {
			freshKeychain, err := generateKeychainForPullSecrets(ctx, lister, imagePullSecrets...)
			if err != nil {
				return err
			}
			c.keychain = authn.NewMultiKeychain(
				baseKeychain,
				freshKeychain,
			)
			return nil
		}
		return nil
	}
}

// WithKeychainPullSecrets provides initialize registry client option that allows to use insecure registries.
func WithAllowInsecureRegistry() Option {
	return func(c *config) error {
		c.transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
		return nil
	}
}

// WithLocalKeychain provides initialize keychain with the default local keychain.
func WithLocalKeychain() Option {
	return func(c *config) error {
		c.pullSecretRefresher = nil
		c.keychain = authn.DefaultKeychain
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

// BuildRemoteOption builds remote.Option based on client.
func (c *client) BuildRemoteOption(ctx context.Context) remote.Option {
	return remote.WithRemoteOptions(
		gcrremote.WithAuthFromKeychain(c.keychain),
		gcrremote.WithTransport(c.transport),
		gcrremote.WithContext(ctx),
	)
}

// FetchImageDescriptor fetches Descriptor from registry with given imageRef
// and provides access to metadata about remote artifact.
func (c *client) FetchImageDescriptor(ctx context.Context, imageRef string) (*gcrremote.Descriptor, error) {
	if err := c.refreshKeychainPullSecrets(ctx); err != nil {
		return nil, fmt.Errorf("failed to refresh image pull secrets, error: %v", err)
	}
	parsedRef, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference: %s, error: %v", imageRef, err)
	}
	desc, err := gcrremote.Get(parsedRef, gcrremote.WithAuthFromKeychain(c.keychain), gcrremote.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image reference: %s, error: %v", imageRef, err)
	}
	return desc, nil
}

// refreshKeychainPullSecrets loads fresh data from pull secrets and updates Keychain.
// If pull secrets are empty - returns.
func (c *client) refreshKeychainPullSecrets(ctx context.Context) error {
	if c.pullSecretRefresher == nil {
		return nil
	}
	return c.pullSecretRefresher(ctx, c)
}

func (c *client) getKeychain() authn.Keychain {
	return c.keychain
}

func (c *client) getTransport() http.RoundTripper {
	return c.transport
}
