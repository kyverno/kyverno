package registryclient

import (
	"context"

	ecr "github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/awslabs/amazon-ecr-credential-helper/ecr-login/api"
	"github.com/chrismellard/docker-credential-acr-env/pkg/credhelper"
	"github.com/google/go-containerregistry/pkg/authn"
	kauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
)

var (
	Secrets []string

	kubeClient            kubernetes.Interface
	kyvernoNamespace      string
	kyvernoServiceAccount string

	amazonKeychain  authn.Keychain = authn.NewKeychainFromHelper(ecr.ECRHelper{ClientFactory: api.DefaultClientFactory{}})
	azureKeychain   authn.Keychain = authn.NewKeychainFromHelper(credhelper.NewACRCredentialsHelper())
	defaultKeychain authn.Keychain = authn.NewMultiKeychain(
		authn.DefaultKeychain,
		google.Keychain,
		amazonKeychain,
		azureKeychain,
	)
	DefaultKeychain authn.Keychain = defaultKeychain
)

// Initialize loads the image pull secrets and initializes the default auth method for container registry API calls
func Initialize(client kubernetes.Interface, namespace, serviceAccount string, imagePullSecrets []string) error {
	kubeClient = client
	kyvernoNamespace = namespace
	kyvernoServiceAccount = serviceAccount
	Secrets = imagePullSecrets

	var kc authn.Keychain
	kcOpts := kauth.Options{
		Namespace:          namespace,
		ServiceAccountName: serviceAccount,
		ImagePullSecrets:   imagePullSecrets,
	}

	kc, err := kauth.New(context.Background(), client, kcOpts)
	if err != nil {
		return errors.Wrap(err, "failed to initialize registry keychain")
	}

	DefaultKeychain = authn.NewMultiKeychain(
		defaultKeychain,
		kc,
	)

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
