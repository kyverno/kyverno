package registryclient

import (
	"net/url"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
)

const ghcrHostname = "ghcr.io"

var GHCRKeychain authn.Keychain = &ghcrKeychain{}

type ghcrKeychain struct{}

func (ghcr ghcrKeychain) Resolve(r authn.Resource) (authn.Authenticator, error) {
	if !isGHCRRegistry(r) {
		return authn.Anonymous, nil
	}
	username := os.Getenv("GITHUB_ACTOR")
	if username == "" {
		username = "unset"
	}
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		return authn.FromConfig(authn.AuthConfig{Username: username, Password: tok}), nil
	}
	return authn.Anonymous, nil
}

func isGHCRRegistry(r authn.Resource) bool {
	serverURL, err := url.Parse("https://" + r.String())
	if err != nil {
		return false
	}
	return serverURL.Hostname() == ghcrHostname
}
