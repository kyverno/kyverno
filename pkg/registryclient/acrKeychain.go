package registryclient

import (
	"context"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/google/go-containerregistry/pkg/authn"
)

const tokenUsername = "<token>"

var ACRKeychain authn.Keychain = acrKeychain{}

type acrKeychain struct{}

func (acr acrKeychain) Resolve(r authn.Resource) (authn.Authenticator, error) {
	if !isACRRegistry(r.RegistryStr()) {
		return authn.Anonymous, nil
	}
	cred_, err := azidentity.NewClientSecretCredential(os.Getenv("TENANT_ID"), os.Getenv("CLIENT_ID"), os.Getenv("SECRET"), nil)
	tk, err := cred_.GetToken(
		context.TODO(), policy.TokenRequestOptions{Scopes: []string{"https://vault.azure.net/.default"}},
	)
	if err != nil {
		return authn.Anonymous, nil
	}
	return authn.FromConfig(authn.AuthConfig{Username: tokenUsername, IdentityToken: tk.Token}), nil
}

func isACRRegistry(host string) bool {
	return host == ".azurecr.io" ||
		strings.HasSuffix(host, ".azurecr.cn") ||
		strings.HasSuffix(host, ".azurecr.de") ||
		strings.HasSuffix(host, ".azurecr.us")
}
