package registryclient

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
)

var (
	Secrets []string

	kubeClient            kubernetes.Interface
	kyvernoNamespace      string
	kyvernoServiceAccount string
	DefaultKeychain       authn.Keychain
)

// Initialize loads the image pull secrets and initializes the default auth method for container registry API calls
func Initialize(client kubernetes.Interface, namespace, serviceAccount string, imagePullSecrets []string) error {
	kubeClient = client
	kyvernoNamespace = namespace
	kyvernoServiceAccount = serviceAccount
	Secrets = imagePullSecrets

	var kc authn.Keychain
	kcOpts := &k8schain.Options{
		Namespace:          namespace,
		ServiceAccountName: serviceAccount,
		ImagePullSecrets:   imagePullSecrets,
	}

	kc, err := k8schain.New(context.Background(), client, *kcOpts)
	if err != nil {
		return errors.Wrap(err, "failed to initialize registry keychain")
	}

	DefaultKeychain = kc
	return nil
}

// UpdateKeychain reinitializes the image pull secrets and default auth method for container registry API calls
func UpdateKeychain() error {
	var err = Initialize(kubeClient, kyvernoNamespace, kyvernoServiceAccount, Secrets)
	if err != nil {
		return err
	}
	return nil
}
