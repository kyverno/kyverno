package registryclient

import (
	"github.com/awslabs/amazon-ecr-credential-helper/ecr-login/api"
	"github.com/google/go-containerregistry/pkg/authn"
)

var ECRKeychain authn.Keychain = &ecrKeychain{}

type ecrKeychain struct {}

func (ecr ecrKeychain) Resolve(r authn.Resource) (authn.Authenticator, error) {
	registry, err := api.ExtractRegistry(r.RegistryStr())

	if err != nil {
		return authn.Anonymous, nil
	}
	var client api.Client
	clientFactory := api.DefaultClientFactory{}
	if registry.FIPS {
		client, err = clientFactory.NewClientWithFipsEndpoint(registry.Region)
		if err != nil {
			return authn.Anonymous, nil
		}
	} else {
		client = clientFactory.NewClientFromRegion(registry.Region)
	}
	auth, err := client.GetCredentials(r.RegistryStr())
	if err != nil {
		return authn.Anonymous, nil
	}
	return authn.FromConfig(authn.AuthConfig{Username: auth.Username, Password: auth.Password}), nil
}
