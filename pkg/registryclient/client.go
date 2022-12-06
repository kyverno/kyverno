package registryclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"

	ecr "github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/chrismellard/docker-credential-acr-env/pkg/credhelper"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/cosign/pkg/oci/remote"
	"k8s.io/client-go/kubernetes"
)

// // DefaultClient is default registry client.
// var DefaultClient, _ = InitClient()

var baseKeychain = authn.NewMultiKeychain(
	authn.DefaultKeychain,
	google.Keychain,
	authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithLogger(io.Discard))),
	authn.NewKeychainFromHelper(credhelper.NewACRCredentialsHelper()),
	github.Keychain,
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

	// // RefreshKeychainPullSecrets loads fresh data from pull secrets and updates Keychain.
	// // If pull secrets are empty - returns.
	RefreshKeychainPullSecrets(context.Context) error

	// BuildRemoteOption builds remote.Option based on client.
	BuildRemoteOption() remote.Option
}

type client struct {
	keychain  authn.Keychain
	transport *http.Transport
	// baseKeychain        authn.Keychain
	pullSecretRefresher func(context.Context, *client) error
}

// Option is an option to initialize registry client.
type Option = func(*client) error

// New creates a new Client with options
func New(options ...Option) (Client, error) {
	c := &client{
		keychain:  baseKeychain,
		transport: gcrremote.DefaultTransport.(*http.Transport),
	}
	for _, opt := range options {
		if err := opt(c); err != nil {
			return nil, err
		}
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
func WithKeychainPullSecrets(ctx context.Context, kubClient kubernetes.Interface, namespace, serviceAccount string, imagePullSecrets ...string) Option {
	return func(c *client) error {
		refresher := func(ctx context.Context, c *client) error {
			freshKeychain, err := generateKeychainForPullSecrets(ctx, kubClient, namespace, serviceAccount, imagePullSecrets...)
			if err != nil {
				return err
			}
			c.keychain = authn.NewMultiKeychain(
				baseKeychain,
				freshKeychain,
			)
			return nil
		}
		c.pullSecretRefresher = refresher
		return refresher(ctx, c)
	}
}

// WithKeychainPullSecrets provides initialize registry client option that allows to use insecure registries.
func WithAllowInsecureRegistry() Option {
	return func(c *client) error {
		c.transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
		return nil
	}
}

// WithLocalKeychain provides initialize keychain with the default local keychain.
func WithLocalKeychain() Option {
	return func(c *client) error {
		c.pullSecretRefresher = nil
		c.keychain = authn.DefaultKeychain
		return nil
	}
}

// RefreshKeychainPullSecrets loads fresh data from pull secrets and updates Keychain.
// If pull secrets are empty - returns.
func (c *client) RefreshKeychainPullSecrets(ctx context.Context) error {
	if c.pullSecretRefresher == nil {
		return nil
	}
	return c.pullSecretRefresher(ctx, c)
}

// BuildRemoteOption builds remote.Option based on client.
func (c *client) BuildRemoteOption() remote.Option {
	return remote.WithRemoteOptions(
		gcrremote.WithAuthFromKeychain(c.keychain),
		gcrremote.WithTransport(c.transport),
	)
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
	return desc, nil
}

func (c *client) getKeychain() authn.Keychain {
	return c.keychain
}

func (c *client) getTransport() http.RoundTripper {
	return c.transport
}
