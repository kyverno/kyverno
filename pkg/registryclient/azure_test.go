package registryclient

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	sdk "github.com/kyverno/sdk/extensions/regcreds"
)

type azureResource string

func (r azureResource) String() string      { return string(r) + "/test" }
func (r azureResource) RegistryStr() string { return string(r) }

type azureCountingKeychain struct{ calls int }

func (k *azureCountingKeychain) Resolve(authn.Resource) (authn.Authenticator, error) {
	k.calls++
	return authn.Anonymous, nil
}

func TestAzureRegistryDomainBoundary(t *testing.T) {
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
			inner := &azureCountingKeychain{}
			_, err := (azureDomainKeychain{inner: inner}).Resolve(azureResource(test.host))
			if err != nil {
				t.Fatal(err)
			}
			if (inner.calls != 0) != test.allowed {
				t.Fatalf("provider calls = %d, allowed = %t", inner.calls, test.allowed)
			}
		})
	}
}

func TestAzureGuardDoesNotChangeSDKProvider(t *testing.T) {
	original := sdk.AzureKeychain
	chains := keychainsForProviders("azure")
	if len(chains) != 1 {
		t.Fatalf("expected one provider, got %d", len(chains))
	}
	if _, ok := chains[0].(azureDomainKeychain); !ok {
		t.Fatalf("unguarded CEL Azure provider: %T", chains[0])
	}
	if sdk.AzureKeychain != original {
		t.Fatal("CEL provider changed the legacy SDK provider")
	}
}
