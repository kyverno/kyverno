package registryclient

import (
	"context"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	kauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/kyverno/kyverno/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type secretsKeychain struct {
	lister           corev1listers.SecretLister
	defaultNamespace string
	references       []string
}

// NewSecretsKeychain preserves per-request cancellation for listers that support
// live Secret reads. Credential sources fall back only when a Secret is absent.
func NewSecretsKeychain(lister corev1listers.SecretLister, namespace string, references ...string) authn.Keychain {
	return &secretsKeychain{lister: lister, defaultNamespace: namespace, references: references}
}

func (k *secretsKeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	return k.ResolveContext(context.Background(), resource)
}

func (k *secretsKeychain) ResolveContext(ctx context.Context, resource authn.Resource) (authn.Authenticator, error) {
	var secrets []corev1.Secret
	for _, reference := range k.references {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		namespace, secretName := k.defaultNamespace, strings.TrimPrefix(reference, "/")
		if parts := strings.SplitN(secretName, "/", 2); len(parts) == 2 {
			namespace, secretName = parts[0], parts[1]
		}
		lister := k.lister.Secrets(namespace)
		var secret *corev1.Secret
		var err error
		if contextual, ok := lister.(interface {
			GetWithContext(context.Context, string) (*corev1.Secret, error)
		}); ok {
			secret, err = contextual.GetWithContext(ctx, secretName)
		} else {
			secret, err = lister.Get(secretName)
		}
		if err == nil {
			secrets = append(secrets, *secret)
		} else if !apierrors.IsNotFound(err) {
			return nil, err
		} else {
			logging.V(4).Info("secret not found, skipping", "namespace", namespace, "name", secretName)
		}
	}
	inner, err := kauth.NewFromPullSecrets(ctx, secrets)
	if err != nil {
		return nil, err
	}
	return authn.Resolve(ctx, inner, resource)
}
