package factories

import (
	"context"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

func TestGetClient_NilSecretsLister(t *testing.T) {
	// Create a factory with nil secretsLister (simulating CLI usage)
	rclient := registryclient.NewOrDie()
	factory := DefaultRegistryClientFactory(adapters.RegistryClient(rclient), nil)

	tests := []struct {
		name        string
		creds       *kyvernov1.ImageRegistryCredentials
		expectError bool
	}{
		{
			name:        "nil credentials should succeed",
			creds:       nil,
			expectError: false,
		},
		{
			name:        "empty credentials should succeed",
			creds:       &kyvernov1.ImageRegistryCredentials{},
			expectError: false,
		},
		{
			name: "providers only should succeed",
			creds: &kyvernov1.ImageRegistryCredentials{
				Providers: []kyvernov1.ImageRegistryCredentialsProvidersType{"default"},
			},
			expectError: false,
		},
		{
			name: "secrets with nil lister should succeed (secrets silently ignored)",
			creds: &kyvernov1.ImageRegistryCredentials{
				Secrets: []string{"my-secret"},
			},
			expectError: false,
		},
		{
			name: "multiple secrets with nil lister should succeed (secrets silently ignored)",
			creds: &kyvernov1.ImageRegistryCredentials{
				Secrets: []string{"secret1", "secret2"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := factory.GetClient(context.Background(), tt.creds)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
