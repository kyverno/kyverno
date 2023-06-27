package registryclient

import (
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
)

const (
	ghcrHostName = "ghcr.io"
	dockerhostName = "docker.io"
	gcrHostName = "gcr.io"
)

var credentialHelper authn.Helper = credHelper{}

type credHelper struct{}

func (ch credHelper) Get(serverUrl string) (string, string, error) {
	if serverUrl == ghcrHostName {
		username := os.Getenv("GITHUB_ACTOR")
		if username == "" {
			username = "unset"
		}
		if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
			return username, tok, nil
		}
	}

	if serverUrl == dockerhostName {
		// TODO
	}

	if serverUrl == gcrHostName {
		// TODO
	}
	
	return "","",nil
}
