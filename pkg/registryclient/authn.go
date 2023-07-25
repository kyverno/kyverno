package registryclient

import (
	"context"

	"github.com/fluxcd/pkg/oci/auth/aws"
	"github.com/fluxcd/pkg/oci/auth/azure"
	"github.com/fluxcd/pkg/oci/auth/gcp"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type autoRefreshSecrets struct {
	lister           corev1listers.SecretNamespaceLister
	imagePullSecrets []string
}

func NewAutoRefreshSecretsKeychain(lister corev1listers.SecretNamespaceLister, imagePullSecrets ...string) (authn.Keychain, error) {
	return &autoRefreshSecrets{
		lister:           lister,
		imagePullSecrets: imagePullSecrets,
	}, nil
}

func (kc *autoRefreshSecrets) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	inner, err := generateKeychainForPullSecrets(kc.lister, kc.imagePullSecrets...)
	if err != nil {
		return nil, err
	}
	return inner.Resolve(resource)
}

type azurekeychain struct{}

var AzureKeychain authn.Keychain = azurekeychain{}

func (azurekeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	ref, err := name.ParseReference(resource.String())
	if err != nil {
		return nil, err
	}
	azClient := azure.NewClient()
	return azClient.Login(context.TODO(), true, resource.String(), ref)
}

type awskeychain struct{}

var AWSKeychain authn.Keychain = awskeychain{}

func (awskeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	awsClient := aws.NewClient()
	return awsClient.Login(context.TODO(), true, resource.String())
}

type gcpkeychain struct{}

var GCPKeychain authn.Keychain = gcpkeychain{}

func (gcpkeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	ref, err := name.ParseReference(resource.String())
	if err != nil {
		return nil, err
	}
	gcpClient := gcp.NewClient()
	return gcpClient.Login(context.TODO(), true, resource.String(), ref)
}
