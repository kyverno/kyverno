package registryclient

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"github.com/fluxcd/pkg/oci/auth/aws"
	"github.com/fluxcd/pkg/oci/auth/azure"
	"github.com/fluxcd/pkg/oci/auth/gcp"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

var (
	acrRE      = regexp.MustCompile(`.*\.azurecr\.io|.*\.azurecr\.cn|.*\.azurecr\.de|.*\.azurecr\.us`)
	ecrPattern = regexp.MustCompile(`(^[a-zA-Z0-9][a-zA-Z0-9-_]*)\.dkr\.ecr(-fips)?\.([a-zA-Z0-9][a-zA-Z0-9-_]*)\.amazonaws\.com(\.cn)?$`)
)

const (
	mcrHostname   = "mcr.microsoft.com"
	tokenUsername = "<token>"

	ServiceECR          = "ecr"
	ServiceECRPublic    = "ecr-public"
	proxyEndpointScheme = "https://"
	programName         = "docker-credential-ecr-login"
	ecrPublicName       = "public.ecr.aws"
	ecrPublicEndpoint   = proxyEndpointScheme + ecrPublicName
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
	if !isACRRegistry(resource.RegistryStr()) {
		return authn.Anonymous, nil
	}

	ref, err := name.ParseReference(resource.String())
	if err != nil {
		return authn.Anonymous, nil
	}

	azClient := azure.NewClient()
	auth, err := azClient.Login(context.TODO(), true, resource.String(), ref)
	if err != nil {
		return authn.Anonymous, nil
	}
	return auth, nil
}

func isACRRegistry(input string) bool {
	serverURL, err := url.Parse("https://" + input)
	if err != nil {
		return false
	}
	if serverURL.Hostname() == mcrHostname {
		return true
	}
	matches := acrRE.FindStringSubmatch(serverURL.Hostname())
	return len(matches) != 0
}

type awskeychain struct{}

var AWSKeychain authn.Keychain = awskeychain{}

func (awskeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	if !isAWSRegistry(resource.RegistryStr()) {
		return authn.Anonymous, nil
	}
	awsClient := aws.NewClient()
	auth, err := awsClient.Login(context.TODO(), true, resource.String())
	if err != nil {
		return authn.Anonymous, nil
	}
	return auth, nil
}

func isAWSRegistry(input string) bool {
	input = strings.TrimPrefix(input, proxyEndpointScheme)
	serverURL, err := url.Parse(proxyEndpointScheme + input)
	if err != nil {
		return false
	}
	if serverURL.Hostname() == ecrPublicName {
		return true
	}
	matches := ecrPattern.FindStringSubmatch(serverURL.Hostname())
	return len(matches) >= 3
}

type gcpkeychain struct{}

var GCPKeychain authn.Keychain = gcpkeychain{}

func (gcpkeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	if !gcp.ValidHost(resource.RegistryStr()) {
		return authn.Anonymous, nil
	}

	ref, err := name.ParseReference(resource.String())
	if err != nil {
		return authn.Anonymous, nil
	}

	gcpClient := gcp.NewClient()
	auth, err := gcpClient.Login(context.TODO(), true, resource.String(), ref)
	if err != nil {
		return authn.Anonymous, nil
	}
	return auth, nil
}
