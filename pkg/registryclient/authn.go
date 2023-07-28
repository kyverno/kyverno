package registryclient

import (
	"github.com/google/go-containerregistry/pkg/authn"
	corev1listers "k8s.io/client-go/listers/core/v1"
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
