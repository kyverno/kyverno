package registryclient

import (
	"net/url"
	"slices"
	"strings"

	"github.com/fluxcd/pkg/oci/auth/azure"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/kyverno/sdk/extensions/regcreds"
)

// keychainsForProviders retains the SDK's credential precedence while confining
// Azure credential exchange to provider domains. It never changes SDK globals.
func keychainsForProviders(providers ...string) []authn.Keychain {
	var chains []authn.Keychain
	for _, provider := range []string{"default", "google", "amazon", "azure", "github"} {
		if !slices.Contains(providers, provider) {
			continue
		}
		for _, chain := range regcreds.KeychainsForProviders(provider) {
			if provider == "azure" {
				chain = azureDomainKeychain{inner: chain}
			}
			chains = append(chains, chain)
		}
	}
	return chains
}

type azureDomainKeychain struct{ inner authn.Keychain }

func (k azureDomainKeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	serverURL, err := url.Parse("https://" + resource.RegistryStr())
	if err != nil || !azure.ValidHost(strings.ToLower(strings.TrimSuffix(serverURL.Hostname(), "."))) {
		return authn.Anonymous, nil
	}
	return k.inner.Resolve(resource)
}
