package registryclient

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	kauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
)

// generateKeychainForPullSecrets generates keychain by fetching secrets data from imagePullSecrets.
func generateKeychainForPullSecrets(ctx context.Context, client kubernetes.Interface, namespace, serviceAccount string, imagePullSecrets ...string) (authn.Keychain, error) {
	kcOpts := kauth.Options{
		Namespace:          namespace,
		ServiceAccountName: serviceAccount,
		ImagePullSecrets:   imagePullSecrets,
	}
	kc, err := kauth.New(ctx, client, kcOpts) // uses k8s client to fetch secrets data
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize registry keychain")
	}
	return kc, err
}
