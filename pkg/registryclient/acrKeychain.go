package registryclient

import (
	"context"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/google/go-containerregistry/pkg/authn"
)

const (
	tokenUsername = "<token>"
)

var (
	tenantID                    = os.Getenv("AZURE_TENANT_ID")
	clientID                    = os.Getenv("AZURE_CLEINT_ID")
	clientSecret                = os.Getenv("AZURE_CLIENT_SECRET")
	ACRKeychain  authn.Keychain = acrKeychain{}
)

type acrKeychain struct{}

func (acr acrKeychain) Resolve(r authn.Resource) (authn.Authenticator, error) {
	if !isACRRegistry(r.RegistryStr()) {
		return authn.Anonymous, nil
	}
	return envCredential()
}

func isACRRegistry(host string) bool {
	return host == ".azurecr.io" ||
		strings.HasSuffix(host, ".azurecr.cn") ||
		strings.HasSuffix(host, ".azurecr.de") ||
		strings.HasSuffix(host, ".azurecr.us")
}

func envCredential() (authn.Authenticator, error) {
	env_cred, err := azidentity.NewEnvironmentCredential(&azidentity.EnvironmentCredentialOptions{})

	tk, err := env_cred.GetToken(
		context.TODO(), policy.TokenRequestOptions{Scopes: []string{"https://vault.azure.net/.default"}},
	)

	if err != nil {
		return authn.Anonymous, nil
	}

	return authn.FromConfig(authn.AuthConfig{Username: tokenUsername, IdentityToken: tk.Token}), nil
}

func azureCliCredential() (authn.Authenticator, error) {
	// getting cli credential
	cli_creds, err := azidentity.NewAzureCLICredential(&azidentity.AzureCLICredentialOptions{
		TenantID: tenantID,
	})

	tk, err := cli_creds.GetToken(
		context.TODO(), policy.TokenRequestOptions{Scopes: []string{"https://vault.azure.net/.default"}},
	)
	if err != nil {
		return authn.Anonymous, nil
	}

	return authn.FromConfig(authn.AuthConfig{Username: tokenUsername, IdentityToken: tk.Token}), nil
}

func clientCredential() (authn.Authenticator, error) {
	cred_, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
	tk, err := cred_.GetToken(
		context.TODO(), policy.TokenRequestOptions{Scopes: []string{"https://vault.azure.net/.default"}},
	)
	if err != nil {
		return authn.Anonymous, nil
	}
	return authn.FromConfig(authn.AuthConfig{Username: tokenUsername, IdentityToken: tk.Token}), nil
}
