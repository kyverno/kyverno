package registryclient

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	Secrets []string

	kubeClient       kubernetes.Interface
	kyvernoNamespace string
)

// Initialize loads the image pull secrets and initializes the default auth method for container registry API calls
func Initialize(client kubernetes.Interface, namespace string, imagePullSecrets []string) error {
	kubeClient = client
	kyvernoNamespace = namespace
	Secrets = imagePullSecrets

	ctx := context.Background()

	var pullSecrets []corev1.Secret
	for _, name := range imagePullSecrets {
		ps, err := client.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to fetch image pull secret: %s/%s", namespace, name))
		}
		pullSecrets = append(pullSecrets, *ps)
	}
	var kc authn.Keychain
	kc, err := k8schain.NewFromPullSecrets(ctx, pullSecrets)
	if err != nil {
		return errors.Wrap(err, "failed to initialize registry keychain")
	}

	authn.DefaultKeychain = kc
	return nil
}

// UpdateKeychain reinitializes the image pull secrets and default auth method for container registry API calls
func UpdateKeychain() error {
	var err = Initialize(kubeClient, kyvernoNamespace, Secrets)
	if err != nil {
		return err
	}
	return nil
}
