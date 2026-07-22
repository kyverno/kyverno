package registryclient

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type autoRefreshSecrets struct {
	lister           corev1listers.SecretLister
	defaultNamespace string
	imagePullSecrets []string
}

func NewAutoRefreshSecretsKeychain(lister corev1listers.SecretLister, defaultNamespace string, imagePullSecrets ...string) (authn.Keychain, error) {
	return &autoRefreshSecrets{
		lister:           lister,
		defaultNamespace: defaultNamespace,
		imagePullSecrets: imagePullSecrets,
	}, nil
}

func (kc *autoRefreshSecrets) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	inner, err := generateKeychainForPullSecrets(context.Background(), kc.lister, kc.defaultNamespace, kc.imagePullSecrets...)
	if err != nil {
		return nil, err
	}
	return inner.Resolve(resource)
}
