package imagedataloader

import (
	"context"
	"io"
	"net/url"
	"regexp"

	"github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/fluxcd/pkg/oci/auth/azure"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	AnonymousKeychain authn.Keychain = anonymousKeyChain{}
	AzureKeychain     authn.Keychain = azureKeyChain{}
	acrRE                            = regexp.MustCompile(`.*\.azurecr\.io|.*\.azurecr\.cn|.*\.azurecr\.de|.*\.azurecr\.us`)
)

func KeychainsForProviders(credentialProviders ...string) []authn.Keychain {
	var chains []authn.Keychain
	helpers := sets.New(credentialProviders...)
	if helpers.Has("default") {
		chains = append(chains, authn.DefaultKeychain)
	}
	if helpers.Has("google") {
		chains = append(chains, google.Keychain)
	}
	if helpers.Has("amazon") {
		chains = append(chains, authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithLogger(io.Discard))))
	}
	if helpers.Has("azure") {
		chains = append(chains, AzureKeychain)
	}
	if helpers.Has("github") {
		chains = append(chains, github.Keychain)
	}
	return chains
}

type autoRefreshSecrets struct {
	lister           SecretInterface
	imagePullSecrets []string
}

func NewAutoRefreshSecretsKeychain(lister SecretInterface, imagePullSecrets ...string) (authn.Keychain, error) {
	return &autoRefreshSecrets{
		lister:           lister,
		imagePullSecrets: imagePullSecrets,
	}, nil
}

func (kc *autoRefreshSecrets) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	inner, err := generateKeychainForPullSecrets(context.TODO(), kc.lister, kc.imagePullSecrets...)
	if err != nil {
		return nil, err
	}
	return inner.Resolve(resource)
}

type anonymousKeyChain struct{}

func (anonymousKeyChain) Resolve(_ authn.Resource) (authn.Authenticator, error) {
	return authn.Anonymous, nil
}

type azureKeyChain struct{}

func (azureKeyChain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	if !isACRRegistry(resource.RegistryStr()) {
		return authn.Anonymous, nil
	}

	ref, err := name.ParseReference(resource.String())
	if err != nil {
		return authn.Anonymous, nil
	}

	azClient := azure.NewClient()
	auth, err := azClient.Login(context.TODO(), true, resource.String(), ref)
	if err != nil {
		return authn.Anonymous, nil
	}
	return auth, nil
}

func isACRRegistry(input string) bool {
	serverURL, err := url.Parse("https://" + input)
	if err != nil {
		return false
	}
	matches := acrRE.FindStringSubmatch(serverURL.Hostname())
	return len(matches) != 0
}
