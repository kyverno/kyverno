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
	kauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
	"github.com/sigstore/cosign/pkg/oci/remote"
	"k8s.io/client-go/kubernetes"
)

// DefaultClient is default registry client.
var DefaultClient, _ = InitClient()

// Client provides registry related objects.
type Client interface {
	// Keychain provides keychain object.
	Keychain() authn.Keychain

	// Transport provides transport object.
	Transport() *http.Transport

	// FetchImageDescriptor fetches Descriptor from registry with given imageRef
	// and provides access to metadata about remote artifact.
	FetchImageDescriptor(imageRef string) (*gcrremote.Descriptor, error)

	// UseLocalKeychain updates keychain with the default local keychain.
	UseLocalKeychain()

	// RefreshKeychainPullSecrets loads fresh data from pull secrets and updates Keychain.
	// If pull secrets are empty - returns.
	RefreshKeychainPullSecrets() error
}

// InitClient initialize registry client with given options.
func InitClient(options ...Option) (Client, error) {
	baseKeychain := authn.NewMultiKeychain(
		authn.DefaultKeychain,
		google.Keychain,
		authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithLogger(io.Discard))),
		authn.NewKeychainFromHelper(credhelper.NewACRCredentialsHelper()),
		github.Keychain,
	)
	c := &client{
		keychain:     baseKeychain,
		baseKeychain: baseKeychain,
		transport:    gcrremote.DefaultTransport,
	}

	for _, opt := range options {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// Option is an option to initialize registry client.
type Option func(*client) error

// WithKeychainPullSecrets provides initialize registry client option that allows to use pull secrets.
func WithKeychainPullSecrets(kubClient kubernetes.Interface, namespace, serviceAccount string, imagePullSecrets []string) Option {
	return func(c *client) error {
		refresher := func(c *client) error {
			freshKeychain, err := generateKeychainForPullSecrets(kubClient, namespace, serviceAccount, imagePullSecrets)
			if err != nil {
				return err
			}

			c.keychain = authn.NewMultiKeychain(
				c.baseKeychain,
				freshKeychain,
			)

			return nil
		}

		c.pullSecretRefresher = refresher
		return refresher(c)
	}
}

// WithKeychainPullSecrets provides initialize registry client option that allows to use insecure registries.
func WithAllowInsecureRegistry() Option {
	return func(c *client) error {
		c.transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
		return nil
	}
}

type client struct {
	keychain  authn.Keychain
	transport *http.Transport

	baseKeychain        authn.Keychain
	pullSecretRefresher func(*client) error
}

// Keychain provides keychain object.
func (c *client) Keychain() authn.Keychain {
	return c.keychain
}

// Transport provides transport object.
func (c *client) Transport() *http.Transport {
	return c.transport
}

// UseLocalKeychain updates keychain with the default local keychain.
func (c *client) UseLocalKeychain() {
	c.keychain = authn.DefaultKeychain
	c.baseKeychain = authn.DefaultKeychain
}

// FetchImageDescriptor fetches Descriptor from registry with given imageRef
// and provides access to metadata about remote artifact.
func (c *client) FetchImageDescriptor(imageRef string) (*gcrremote.Descriptor, error) {
	parsedRef, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference: %s, error: %v", imageRef, err)
	}

	desc, err := gcrremote.Get(parsedRef, gcrremote.WithAuthFromKeychain(c.keychain))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image reference: %s, error: %v", imageRef, err)
	}

	return desc, nil
}

// RefreshKeychainPullSecrets loads fresh data from pull secrets and updates Keychain.
// If pull secrets are empty - returns.
func (c *client) RefreshKeychainPullSecrets() error {
	if c.pullSecretRefresher == nil {
		return nil
	}

	return c.pullSecretRefresher(c)
}

// generateKeychainForPullSecrets generates keychain by fetching secrets data from imagePullSecrets.
func generateKeychainForPullSecrets(
	client kubernetes.Interface,
	namespace, serviceAccount string,
	imagePullSecrets []string,
) (authn.Keychain, error) {
	kcOpts := kauth.Options{
		Namespace:          namespace,
		ServiceAccountName: serviceAccount,
		ImagePullSecrets:   imagePullSecrets,
	}

	kc, err := kauth.New(context.Background(), client, kcOpts) // uses k8s client to fetch secrets data
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize registry keychain")
	}
	return kc, err
}

// BuildRemoteOption builds remote.Option based on client.
func BuildRemoteOption(c Client) remote.Option {
	return remote.WithRemoteOptions(
		gcrremote.WithAuthFromKeychain(c.Keychain()),
		gcrremote.WithTransport(c.Transport()),
	)
}
