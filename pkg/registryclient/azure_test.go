package registryclient

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
)

type azureResource string

// The invalid reference stops an accepted Azure resource before login, so the
// provider selection test cannot contact Azure or use ambient credentials.
func (r azureResource) String() string      { return "invalid image reference" }
func (r azureResource) RegistryStr() string { return string(r) }

func TestAzureRegistryDomainBoundary(t *testing.T) {
	configured := New(WithCredentialHelpers("azure")).(*client)
	// Exercise the actual SDK provider selected by this consumer. The separate
	// network guard is covered by the registry egress tests.
	keychain := configured.keychain.(*guardedKeychain).inner
	for _, test := range []struct {
		host    string
		allowed bool
	}{
		{"registry.azurecr.io", true}, {"registry.azurecr.cn", true}, {"registry.azurecr.de", true}, {"registry.azurecr.us", true},
		{"registry.azurecr.io:443", true}, {"REGISTRY.AZURECR.IO", true}, {"registry.azurecr.io.", true}, {"registry.azurecr.io.:443", true},
		{"registry.azurecr.io.attacker.example", false}, {"registry.azurecr.cn.attacker.example", false},
		{"registry.azurecr.de.attacker.example", false}, {"registry.azurecr.us.attacker.example", false},
		{"registryazurecr.io", false}, {"azurecr.io", false}, {"localhost", false}, {"127.0.0.1", false},
	} {
		t.Run(test.host, func(t *testing.T) {
			authenticator, err := keychain.Resolve(azureResource(test.host))
			if test.allowed {
				if err == nil {
					t.Fatal("Azure provider did not parse the invalid reference for an allowed registry")
				}
				return
			}
			if err != nil {
				t.Fatalf("Azure provider parsed a non-Azure resource: %v", err)
			}
			if authenticator != authn.Anonymous {
				t.Fatal("expected anonymous authentication for a non-Azure registry")
			}
		})
	}
}
