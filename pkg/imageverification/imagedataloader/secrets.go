package imagedataloader

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	kauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecretInterface interface {
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Secret, error)
}

// generateKeychainForPullSecrets generates keychain by fetching secrets data from imagePullSecrets.
func generateKeychainForPullSecrets(ctx context.Context, lister SecretInterface, imagePullSecrets ...string) (authn.Keychain, error) {
	var secrets []corev1.Secret
	for _, imagePullSecret := range imagePullSecrets {
		secret, err := lister.Get(ctx, imagePullSecret, metav1.GetOptions{})
		if err == nil {
			secrets = append(secrets, *secret)
		} else if !k8serrors.IsNotFound(err) {
			return nil, err
		}
	}
	return kauth.NewFromPullSecrets(context.TODO(), secrets)
}
