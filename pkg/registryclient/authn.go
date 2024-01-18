package registryclient

import (
	"context"
	"net/url"
	"regexp"

	"github.com/fluxcd/pkg/oci/auth/azure"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

var acrRE = regexp.MustCompile(`.*\.azurecr\.io|.*\.azurecr\.cn|.*\.azurecr\.de|.*\.azurecr\.us`)

const (
	mcrHostname = "mcr.microsoft.com"
)

type autoRefreshSecrets struct {
	lister           corev1listers.SecretNamespaceLister
	imagePullSecrets []string
}

func NewAutoRefreshSecretsKeychain(lister corev1listers.SecretNamespaceLister, imagePullSecrets ...string) (authn.Keychain, error) {
	return &autoRefreshSecrets{
		lister:           lister,
		imagePullSecrets: imagePullSecrets,
	}, nil
}

func (kc *autoRefreshSecrets) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	inner, err := generateKeychainForPullSecrets(kc.lister, kc.imagePullSecrets...)
	if err != nil {
		return nil, err
	}
	return inner.Resolve(resource)
}

type anonymuskc struct{}

var AnonymousKeychain authn.Keychain = anonymuskc{}

func (anonymuskc) Resolve(_ authn.Resource) (authn.Authenticator, error) {
	return authn.Anonymous, nil
}

type azurekeychain struct{}

var AzureKeychain authn.Keychain = azurekeychain{}

func (azurekeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
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
	if serverURL.Hostname() == mcrHostname {
		return true
	}
	matches := acrRE.FindStringSubmatch(serverURL.Hostname())
	return len(matches) != 0
}
