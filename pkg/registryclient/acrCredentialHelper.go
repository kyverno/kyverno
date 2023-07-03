package registryclient

import (
	"context"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/google/go-containerregistry/pkg/authn"
)

func acrAuthenticator(r authn.Resource) (authn.Authenticator, error) {
	if !isACRRegistry(r.RegistryStr()) {
		return authn.Anonymous, nil
	}
	// spToken, settings, err := token.GetServicePrincipalTokenFromEnvironment()
	// if err != nil {
	// 	return authn.Anonymous, nil
	// }
	// refreshToken, err := registry.GetRegistryRefreshTokenFromAADExchange(r.RegistryStr(), spToken, settings.Values[auth.TenantID])
	// if err != nil {
	// 	return authn.Anonymous, nil
	// }
	cred_, err := azidentity.NewClientSecretCredential(os.Getenv("TENANT_ID"), os.Getenv("CLIENT_ID"), os.Getenv("SECRET"), nil)

	tk, err := cred_.GetToken(
		context.TODO(), policy.TokenRequestOptions{Scopes: []string{"https://vault.azure.net/.default"}},
	)
	if err != nil {
		return authn.Anonymous, nil
	}
	return authn.FromConfig(authn.AuthConfig{Username: tokenUsername, IdentityToken: tk.token}), nil
}

// // getServicePrincipalToken retrieves an Azure AD OAuth2 token from the supplied environment settings for the specified resource
// func getServicePrincipalToken(settings auth.EnvironmentSettings, resource string) (*azidentity.ServicePrincipalToken, error) {
// 	//1.Client Credentials
// 	if _, e := settings.GetClientCredentials(); e == nil {
// 		clientCredentialsConfig, err := settings.GetClientCredentials()
// 		if err != nil {
// 			return &azidentity.ServicePrincipalToken{}, fmt.Errorf("failed to get client credentials settings from environment - %w", err)
// 		}
// 		oAuthConfig, err := azidentity.NewOAuthConfig(settings.Environment.ActiveDirectoryEndpoint, clientCredentialsConfig.TenantID)
// 		if err != nil {
// 			return &azidentity.ServicePrincipalToken{}, fmt.Errorf("failed to initialise OAuthConfig - %w", err)
// 		}
// 		return azidentity.NewServicePrincipalToken(*oAuthConfig, clientCredentialsConfig.ClientID, clientCredentialsConfig.ClientSecret, clientCredentialsConfig.Resource)
// 	}

// 	//2. Client Certificate
// 	if _, e := settings.GetClientCertificate(); e == nil {
// 		return &azidentity.ServicePrincipalToken{}, fmt.Errorf("authentication method currently unsupported")
// 	}

// 	//3. Username Password
// 	if _, e := settings.GetUsernamePassword(); e == nil {
// 		return &azidentity.ServicePrincipalToken{}, fmt.Errorf("authentication method currently unsupported")
// 	}

// 	// federated OIDC JWT assertion
// 	jwt, err := jwtLookup()
// 	if err == nil {
// 		clientID, isPresent := os.LookupEnv("AZURE_CLIENT_ID")
// 		if !isPresent {
// 			return &azidentity.ServicePrincipalToken{}, fmt.Errorf("failed to get client id from environment")
// 		}
// 		tenantID, isPresent := os.LookupEnv("AZURE_TENANT_ID")
// 		if !isPresent {
// 			return &azidentity.ServicePrincipalToken{}, fmt.Errorf("failed to get client id from environment")
// 		}

// 		oAuthConfig, err := azidentity.NewOAuthConfig(settings.Environment.ActiveDirectoryEndpoint, tenantID)
// 		if err != nil {
// 			return &azidentity.ServicePrincipalToken{}, fmt.Errorf("failed to initialise OAuthConfig - %w", err)
// 		}

// 		return azidentity.NewServicePrincipalTokenFromFederatedToken(*oAuthConfig, clientID, *jwt, resource)
// 	}

// 	// 4. MSI
// 	return azidentity.NewServicePrincipalTokenFromManagedIdentity(resource, &azidentity.ManagedIdentityOptions{
// 		ClientID: os.Getenv("AZURE_CLIENT_ID"),
// 	})
// }

// func GetServicePrincipalTokenFromEnvironment() (*azidentity.ServicePrincipalToken, auth.EnvironmentSettings, error) {
// 	settings, err := auth.GetSettingsFromEnvironment()
// 	if err != nil {
// 		return &azidentity.ServicePrincipalToken{}, auth.EnvironmentSettings{}, fmt.Errorf("failed to get auth settings from environment - %w", err)
// 	}

// 	spToken, err := getServicePrincipalToken(settings, settings.Environment.ResourceManagerEndpoint)
// 	if err != nil {
// 		return &azidentity.ServicePrincipalToken{}, auth.EnvironmentSettings{}, fmt.Errorf("failed to initialise sp token config %w", err)
// 	}

// 	return spToken, settings, nil
// }

// func jwtLookup() (*string, error) {
// 	jwt, isPresent := os.LookupEnv("AZURE_FEDERATED_TOKEN")
// 	if isPresent {
// 		return &jwt, nil
// 	}

// 	if jwtFile, isPresent := os.LookupEnv("AZURE_FEDERATED_TOKEN_FILE"); isPresent {
// 		jwtBytes, err := os.ReadFile(jwtFile)
// 		if err != nil {
// 			return nil, err
// 		}
// 		jwt = string(jwtBytes)
// 		return &jwt, nil
// 	}
// 	return nil, fmt.Errorf("no JWT found")
// }

func isACRRegistry(host string) bool {
	return host == ".azurecr.io" ||
		strings.HasSuffix(host, ".azurecr.cn") ||
		strings.HasSuffix(host, ".azurecr.de") ||
		strings.HasSuffix(host, ".azurecr.us")
}
