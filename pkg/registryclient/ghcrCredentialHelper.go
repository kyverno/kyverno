package registryclient

import (
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
)

func ghcrAuthenticator() (authn.Authenticator, error) {
	username := os.Getenv("GITHUB_ACTOR")
	if username == "" {
		username = "unset"
	}
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		return authn.FromConfig(authn.AuthConfig{Username: username, Password: tok}), nil
	}
	return authn.Anonymous, nil
}
