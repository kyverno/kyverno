package registryclient

import (
	"context"
	"fmt"
	"io/ioutil"

	ecr "github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/chrismellard/docker-credential-acr-env/pkg/credhelper"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	kauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
)

var (
	isLocal         bool
	secrets         []string
	kubeClient      kubernetes.Interface
	namespace       string
	serviceAccount  string
	defaultKeychain = authn.NewMultiKeychain(
		authn.DefaultKeychain,
		google.Keychain,
		authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithLogger(ioutil.Discard))),
		authn.NewKeychainFromHelper(credhelper.NewACRCredentialsHelper()),
		github.Keychain,
	)
)

// InitializeLocal loads the docker credentials and initializes the default auth method for container registry API calls
func InitializeLocal() {
	isLocal = true
}

// Initialize loads the image pull secrets and initializes the default auth method for container registry API calls
func Initialize(client kubernetes.Interface, ns, sa string, imagePullSecrets []string) error {
	isLocal = false
	kubeClient = client
	namespace = ns
	serviceAccount = sa
	secrets = imagePullSecrets
	_, err := getKeychain()
	return err
}

func getKeychain() (authn.Keychain, error) {
	if isLocal {
		return authn.DefaultKeychain, nil
	}
	if len(secrets) == 0 {
		return defaultKeychain, nil
	}
	kcOpts := kauth.Options{
		Namespace:          namespace,
		ServiceAccountName: serviceAccount,
		ImagePullSecrets:   secrets,
	}
	kc, err := kauth.New(context.Background(), kubeClient, kcOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize registry keychain")
	}
	return authn.NewMultiKeychain(defaultKeychain, kc), nil
}

func Get(ref string) (name.Reference, *remote.Descriptor, error) {
	parsedRef, err := name.ParseReference(ref)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse image reference: %s, error: %v", ref, err)
	}
	kc, err := getKeychain()
	if err != nil {
		return nil, nil, err
	}
	desc, err := remote.Get(parsedRef, remote.WithAuthFromKeychain(kc))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch image reference: %s, error: %v", ref, err)
	}
	return parsedRef, desc, nil
}

func GetOptions() (remote.Option, error) {
	kc, err := getKeychain()
	if err != nil {
		return nil, err
	}
	return remote.WithAuthFromKeychain(kc), nil
}
