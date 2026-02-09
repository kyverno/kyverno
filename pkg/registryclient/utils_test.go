package registryclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSecretReference(t *testing.T) {
	tests := []struct {
		name             string
		secretRef        string
		defaultNamespace string
		wantNamespace    string
		wantName         string
	}{
		{
			name:             "simple secret name defaults to kyverno namespace",
			secretRef:        "my-secret",
			defaultNamespace: "kyverno",
			wantNamespace:    "kyverno",
			wantName:         "my-secret",
		},
		{
			name:             "namespaced secret with forward slash",
			secretRef:        "app-namespace/my-secret",
			defaultNamespace: "kyverno",
			wantNamespace:    "app-namespace",
			wantName:         "my-secret",
		},
		{
			name:             "default namespace when empty",
			secretRef:        "secret",
			defaultNamespace: "default",
			wantNamespace:    "default",
			wantName:         "secret",
		},
		{
			name:             "explicit kyverno namespace",
			secretRef:        "kyverno/registry-creds",
			defaultNamespace: "default",
			wantNamespace:    "kyverno",
			wantName:         "registry-creds",
		},
		{
			name:             "multiple slashes uses first as separator",
			secretRef:        "namespace/with/slashes",
			defaultNamespace: "kyverno",
			wantNamespace:    "namespace",
			wantName:         "with/slashes",
		},
		{
			name:             "secret ref starts with a slash",
			secretRef:        "/missing-namespace",
			defaultNamespace: "kyverno",
			wantNamespace:    "kyverno",
			wantName:         "missing-namespace",
		},
		{
			name:             "empty secret name",
			secretRef:        "",
			defaultNamespace: "kyverno",
			wantNamespace:    "kyverno",
			wantName:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNamespace, gotName := ParseSecretReference(tt.secretRef, tt.defaultNamespace)
			assert.Equal(t, tt.wantNamespace, gotNamespace, "namespace mismatch")
			assert.Equal(t, tt.wantName, gotName, "name mismatch")
		})
	}
}

func TestParseSecretReference_BackwardCompatibility(t *testing.T) {
	// Test that existing simple secret names continue to work with Kyverno namespace
	testCases := []string{
		"registry-secret",
		"gcr-secret",
		"dockerhub-creds",
	}

	for _, secretName := range testCases {
		t.Run(secretName, func(t *testing.T) {
			namespace, name := ParseSecretReference(secretName, "kyverno")
			assert.Equal(t, "kyverno", namespace, "simple names should default to kyverno namespace")
			assert.Equal(t, secretName, name, "name should be preserved")
		})
	}
}
