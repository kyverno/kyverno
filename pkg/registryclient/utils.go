package registryclient

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	kauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// generateKeychainForPullSecrets generates keychain by fetching secrets data from imagePullSecrets.
func generateKeychainForPullSecrets(ctx context.Context, lister corev1listers.SecretNamespaceLister, imagePullSecrets ...string) (authn.Keychain, error) {
	var secrets []corev1.Secret
	for _, imagePullSecret := range imagePullSecrets {
		secret, err := lister.Get(imagePullSecret)
		if err == nil {
			secrets = append(secrets, *secret)
		} else if !k8serrors.IsNotFound(err) {
			return nil, err
		}
	}
	return kauth.NewFromPullSecrets(ctx, secrets)
}
