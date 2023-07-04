package registryclient

import (
	"net/url"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	// "github.com/docker/docker-credential-helpers/credentials"
)

const (
	ghcrHostname   = "ghcr.io"
	gcrHostname    = "gcr.io"
	dockerHostname = "docker.io"
	acrHostname    = "mcr.microsoft.com"
	ecrHostname    = "ecr.io"
	tokenUsername  = "<token>"
	DefaultAuthKey = "https://" + name.DefaultRegistry + "/v1/"
)

var RegistryKeychain authn.Keychain = registryKeychain{}

type registryKeychain struct {
	credentialProviders string
}

func (reg registryKeychain) Resolve(r authn.Resource) (authn.Authenticator, error) {

	serverURL, err := url.Parse("https://" + r.String())
	if err != nil {
		return authn.Anonymous, nil
	}

	if serverURL.Hostname() == ghcrHostname {
		return ghcrAuthenticator()
	}

	if serverURL.Hostname() == dockerHostname {
		return defaultAuthenticator(r)
	}

	if serverURL.Hostname() == gcrHostname {
		return gcrAuthenticator(r)
	}

	if serverURL.Hostname() == acrHostname {
		return acrAuthenticator(r)
	}

	if serverURL.Hostname() == ecrHostname {
		return ecrAuthenticator(r)
	}

	return authn.Anonymous, nil
}
