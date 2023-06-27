package registryclient

import (
	"net/url"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
)

var (
	ghcrHostName = "ghcr.io"
	dockerhostName = "docker.io"
	gcrHostName = "gcr.io"
)

var credentialHelper authn.Helper = credHelper{}

type credHelper struct{}

func (ch credHelper) Get(serverUrl string) (string, string, error) {
	serverURL, err := url.Parse(serverUrl)
	if err != nil {
		return "", "", nil
	}
	if serverURL.Hostname() == ghcrHostName {
		username := os.Getenv("GITHUB_ACTOR")
		if username == "" {
			username = "unset"
		}
		if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
			return username, tok, nil
		}
	}
	if serverURL.Hostname == dockerhostName {
		// TODO
	}

	if serverURL.HostName == gcrHostName {
		// TODO
	}
	
	return "","",nil
}
