package policy

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidate_VerifyImagesCredentialScope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		namespace     string
		cluster       bool
		noCredentials bool
		secret        string
		errorContains string
	}{
		{name: "bare secret", namespace: "tenant-a", secret: "pull-secret"},
		{name: "same namespace", namespace: "tenant-a", secret: "tenant-a/pull-secret"},
		{name: "foreign namespace", namespace: "tenant-a", secret: "tenant-b/pull-secret", errorContains: "instead of policy namespace"},
		{name: "installation namespace", namespace: "tenant-a", secret: "kyverno/pull-secret", errorContains: "instead of policy namespace"},
		{name: "malformed secret", namespace: "tenant-a", secret: "tenant-a/pull/extra", errorContains: "invalid name"},
		{name: "empty secret name", namespace: "tenant-a", secret: "tenant-a/", errorContains: "empty name"},
		{name: "malformed policy namespace", namespace: "tenant/a", secret: "pull-secret", errorContains: "policy namespace \"tenant/a\" is invalid"},
		{name: "missing policy namespace", secret: "pull-secret", errorContains: "policy namespace must not be empty"},
		{name: "missing namespace without credentials", noCredentials: true, errorContains: "policy namespace must not be empty"},
		{name: "bare secret variable", namespace: "tenant-a", secret: "{{ request.object.metadata.name }}"},
		{name: "same namespace secret variable", namespace: "tenant-a", secret: "tenant-a/{{ request.object.metadata.name }}"},
		{name: "foreign namespace secret variable", namespace: "tenant-a", secret: "tenant-b/{{ request.object.metadata.name }}", errorContains: "instead of policy namespace"},
		{name: "cluster policy bare secret", cluster: true, secret: "pull-secret"},
		{name: "cluster policy foreign namespace", cluster: true, secret: "tenant-b/pull-secret"},
		{name: "no explicit credentials", namespace: "tenant-a", noCredentials: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var credentials *kyvernov1.ImageRegistryCredentials
			if !test.noCredentials {
				credentials = &kyvernov1.ImageRegistryCredentials{Secrets: []string{test.secret}}
			}
			spec := kyvernov1.Spec{Rules: []kyvernov1.Rule{{
				Name: "verify-image",
				MatchResources: kyvernov1.MatchResources{Any: []kyvernov1.ResourceFilter{{
					ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Pod"}},
				}}},
				VerifyImages: []kyvernov1.ImageVerification{{
					ImageReferences:          []string{"ghcr.io/verified/*"},
					Required:                 true,
					ImageRegistryCredentials: credentials,
				}},
			}}}
			var policy kyvernov1.PolicyInterface
			if test.cluster {
				policy = &kyvernov1.ClusterPolicy{
					TypeMeta:   metav1.TypeMeta{APIVersion: "kyverno.io/v1", Kind: "ClusterPolicy"},
					ObjectMeta: metav1.ObjectMeta{Name: "verify-image"},
					Spec:       spec,
				}
			} else {
				policy = &kyvernov1.Policy{
					TypeMeta:   metav1.TypeMeta{APIVersion: "kyverno.io/v1", Kind: "Policy"},
					ObjectMeta: metav1.ObjectMeta{Name: "verify-image", Namespace: test.namespace},
					Spec:       spec,
				}
			}
			_, err := Validate(policy, nil, nil, true, "", "")
			if test.errorContains != "" {
				require.ErrorContains(t, err, test.errorContains)
				assert.Contains(t, err.Error(), "verifyImages[0].imageRegistryCredentials")
			} else {
				require.NoError(t, err)
			}
			if credentials != nil {
				assert.Equal(t, []string{test.secret}, credentials.Secrets, "admission validation must not rewrite the policy")
			}
		})
	}
}

func TestValidateImageRegistryCredentialScope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		namespace     string
		secrets       []string
		noCredentials bool
		errorContains string
	}{
		{name: "missing namespace and no credentials", noCredentials: true, errorContains: "policy namespace must not be empty"},
		{name: "missing namespace and empty credentials", errorContains: "policy namespace must not be empty"},
		{name: "malformed secret", namespace: "tenant-a", secrets: []string{"tenant-a/pull/extra"}, errorContains: "invalid name"},
		{name: "bare secret variable", namespace: "tenant-a", secrets: []string{"{{ request.object.metadata.name }}"}, errorContains: "invalid name"},
		{name: "same namespace secret variable", namespace: "tenant-a", secrets: []string{"tenant-a/{{ request.object.metadata.name }}"}, errorContains: "invalid name"},
		{name: "foreign namespace secret variable", namespace: "tenant-a", secrets: []string{"tenant-b/{{ request.object.metadata.name }}"}, errorContains: "invalid name"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var credentials *kyvernov1.ImageRegistryCredentials
			if !test.noCredentials {
				credentials = &kyvernov1.ImageRegistryCredentials{Secrets: test.secrets}
			}
			entry := kyvernov1.ContextEntry{ImageRegistry: &kyvernov1.ImageRegistry{
				Reference:                "ghcr.io/verified/app:latest",
				ImageRegistryCredentials: credentials,
			}}
			err := validateImageRegistry(entry, true, test.namespace)
			if test.errorContains != "" {
				require.ErrorContains(t, err, test.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
