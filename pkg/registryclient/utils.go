package registryclient

import (
	"context"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	kauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/kyverno/kyverno/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// ParseSecretReference parses a secret reference which can be:
// - "secret-name" -> namespace=defaultNamespace, name=secret-name
// - "namespace/secret-name" -> namespace=namespace, name=secret-name
func ParseSecretReference(secretRef string, defaultNamespace string) (namespace string, name string) {
	// trim leading "/" if secret is incorrectly defined without namespace
	secretRef = strings.TrimPrefix(secretRef, "/")
	parts := strings.SplitN(secretRef, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return defaultNamespace, secretRef
}

// generateKeychainForPullSecrets generates keychain by fetching secrets data from imagePullSecrets.
// Supports namespace/name notation for secrets in any namespace.
func generateKeychainForPullSecrets(ctx context.Context, lister corev1listers.SecretLister, defaultNamespace string, imagePullSecrets ...string) (authn.Keychain, error) {
	var secrets []corev1.Secret
	for _, imagePullSecret := range imagePullSecrets {
		namespace, name := ParseSecretReference(imagePullSecret, defaultNamespace)
		secret, err := lister.Secrets(namespace).Get(name)
		if err == nil {
			secrets = append(secrets, *secret)
		} else if !k8serrors.IsNotFound(err) {
			return nil, err
		} else {
			logging.V(4).Info("secret not found, skipping", "namespace", namespace, "name", name)
		}
	}
	return kauth.NewFromPullSecrets(ctx, secrets)
}
