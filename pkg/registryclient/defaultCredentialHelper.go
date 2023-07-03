package registryclient

import (
	"os"
	"path/filepath"

	Config "github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/mitchellh/go-homedir"
)

func defaultAuthenticator(r authn.Resource) (authn.Authenticator, error) {
	foundDockerConfig := false
	home, err := homedir.Dir()
	if err == nil {
		foundDockerConfig = fileExists(filepath.Join(home, ".docker/config.json"))
	}
	if !foundDockerConfig && os.Getenv("DOCKER_CONFIG") != "" {
		foundDockerConfig = fileExists(filepath.Join(os.Getenv("DOCKER_CONFIG"), "config.json"))
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
