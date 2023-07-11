package registryclient

import (
	"net/url"
	"os"
	"path/filepath"

	Config "github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mitchellh/go-homedir"
)

const (
	DefaultAuthKey  = "https://" + name.DefaultRegistry + "/v1/"
	ghcrHostname    = "ghcr.io"
	defaultHostname = "docker.io"
)

type registrykeychain struct{}

func NewRegistryKeychain() authn.Keychain {
	return &registrykeychain{}
}

func (reg *registrykeychain) Resolve(r authn.Resource) (authn.Authenticator, error) {
	if reg.isGHCRRegistry(r) {
		if !reg.isGHCRRegistry(r) {
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

	if reg.isDefaultRegistry(r) {
		if !reg.isDefaultRegistry(r) {
			return authn.Anonymous, nil
		}
		foundDockerConfig := false
		home, err := homedir.Dir()
		if err == nil {
			foundDockerConfig = reg.fileExists(filepath.Join(home, ".docker/config.json"))
		}
		if !foundDockerConfig && os.Getenv("DOCKER_CONFIG") != "" {
			foundDockerConfig = reg.fileExists(filepath.Join(os.Getenv("DOCKER_CONFIG"), "config.json"))
		}
		var cf *configfile.ConfigFile
		if foundDockerConfig {
			cf, err = Config.Load(os.Getenv("DOCKER_CONFIG"))
			if err != nil {
				return nil, err
			}
		} else {
			f, err := os.Open(filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "containers/auth.json"))
			if err != nil {
				return authn.Anonymous, nil
			}
			defer f.Close()
			cf, err = Config.LoadFromReader(f)
			if err != nil {
				return nil, err
			}
		}

		var cfg, empty types.AuthConfig
		for _, key := range []string{
			r.String(),
			r.RegistryStr(),
		} {
			if key == name.DefaultRegistry {
				key = DefaultAuthKey
			}
			cfg, err = cf.GetAuthConfig(key)
			if err != nil {
				return nil, err
			}
			cfg.ServerAddress = ""
			if cfg != empty {
				break
			}
		}

		if cfg == empty {
			return authn.Anonymous, nil
		}

		return authn.FromConfig(authn.AuthConfig{
			Username:      cfg.Username,
			Password:      cfg.Password,
			Auth:          cfg.Auth,
			IdentityToken: cfg.IdentityToken,
			RegistryToken: cfg.RegistryToken,
		}), nil
	}
	return authn.Anonymous, nil
}

func (reg *registrykeychain) fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func (reg *registrykeychain) isGHCRRegistry(r authn.Resource) bool {
	serverURL, err := url.Parse("https://" + r.String())
	if err != nil {
		return false
	}
	return serverURL.Hostname() == ghcrHostname
}

func (reg *registrykeychain) isDefaultRegistry(r authn.Resource) bool {
	serverURL, err := url.Parse("https://" + r.String())
	if err != nil {
		return false
	}
	return serverURL.Hostname() == defaultHostname
}
