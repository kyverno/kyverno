package registryclient

import (
	"context"
	"io/ioutil"

	"github.com/google/go-containerregistry/pkg/authn/github"

	ecr "github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/chrismellard/docker-credential-acr-env/pkg/credhelper"
	"github.com/google/go-containerregistry/pkg/authn"
	kauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
)

var (
	Secrets []string

	kubeClient     kubernetes.Interface
	namespace      string
	serviceAccount string

	defaultKeychain = authn.NewMultiKeychain(
		authn.DefaultKeychain,
		google.Keychain,
		authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithLogger(ioutil.Discard))),
		authn.NewKeychainFromHelper(credhelper.NewACRCredentialsHelper()),
		github.Keychain,
	)

	DefaultKeychain = defaultKeychain
)

// InitializeLocal loads the docker credentials and initializes the default auth method for container registry API calls
func InitializeLocal() {
	DefaultKeychain = authn.DefaultKeychain
}

// Initialize loads the image pull secrets and initializes the default auth method for container registry API calls
func Initialize(client kubernetes.Interface, ns, sa string, imagePullSecrets []string) error {
	kubeClient = client
	namespace = ns
	serviceAccount = sa
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
	var err = Initialize(kubeClient, namespace, serviceAccount, Secrets)
	if err != nil {
		return err
	}
	return nil
}
