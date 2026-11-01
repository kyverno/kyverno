package credentials

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestScopePolicySecretReferences(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"credentials", "attestor"} {
		for _, tc := range []struct {
			name, reference, namespace, want string
			denied                           bool
		}{
			{name: "bare", reference: "registry", namespace: "team-a", want: "team-a/registry"},
			{name: "same namespace", reference: "team-a/registry", namespace: "team-a", want: "team-a/registry"},
			{name: "foreign namespace", reference: "team-b/registry", namespace: "team-a", denied: true},
			{name: "installation namespace", reference: "kyverno/registry", namespace: "team-a", denied: true},
			{name: "empty policy namespace", reference: "registry", denied: true},
			{name: "empty secret", namespace: "team-a", denied: true},
			{name: "extra path segment", reference: "team-a/registry/extra", namespace: "team-a", denied: true},
			{name: "empty secret name", reference: "team-a/", namespace: "team-a", denied: true},
		} {
			t.Run(source+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				policy := &policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: tc.namespace}}
				if source == "credentials" {
					policy.Spec.Credentials = &policiesv1beta1.Credentials{Secrets: []string{tc.reference}}
				} else {
					policy.Spec.Attestors = []policiesv1beta1.Attestor{{Name: "signer", Cosign: &policiesv1beta1.Cosign{
						Source: &policiesv1beta1.Source{SignaturePullSecrets: []corev1.LocalObjectReference{{Name: tc.reference}}},
					}}}
				}
				original := policy.DeepCopy()
				scoped, errs := ScopePolicy(policy)
				assert.Equal(t, original, policy, "cached policy must remain unchanged")
				if tc.denied {
					require.NotEmpty(t, errs)
					assert.Nil(t, scoped)
					return
				}
				require.Empty(t, errs)
				require.NotSame(t, policy, scoped)
				if source == "credentials" {
					assert.Equal(t, tc.want, scoped.GetSpec().Credentials.Secrets[0])
				} else {
					assert.Equal(t, tc.want, scoped.GetSpec().Attestors[0].Cosign.Source.SignaturePullSecrets[0].Name)
				}
			})
		}
	}
}

func TestScopePolicyClusterCompatibility(t *testing.T) {
	t.Parallel()
	for _, reference := range []string{"registry", "team-b/registry", "kyverno/registry"} {
		t.Run(reference, func(t *testing.T) {
			t.Parallel()
			policy := &policiesv1beta1.ImageValidatingPolicy{Spec: policiesv1beta1.ImageValidatingPolicySpec{
				Credentials: &policiesv1beta1.Credentials{Secrets: []string{reference}},
				Attestors: []policiesv1beta1.Attestor{{Name: "signer", Cosign: &policiesv1beta1.Cosign{
					Source: &policiesv1beta1.Source{SignaturePullSecrets: []corev1.LocalObjectReference{{Name: reference}}},
				}}},
			}}
			original := policy.DeepCopy()
			scoped, errs := ScopePolicy(policy)
			require.Empty(t, errs)
			assert.Equal(t, original, scoped)
		})
	}
}

func TestScopePolicyEmptyNamespaceFailsWithoutCredentials(t *testing.T) {
	t.Parallel()
	_, errs := ScopePolicy(&policiesv1beta1.NamespacedImageValidatingPolicy{})
	require.NotEmpty(t, errs)
	assert.Equal(t, "metadata.namespace", errs[0].Field)
}

func TestScopePolicyRejectsMissingOrMalformedNamespace(t *testing.T) {
	t.Parallel()
	var nilPolicy *policiesv1beta1.NamespacedImageValidatingPolicy
	for _, policy := range []policiesv1beta1.ImageValidatingPolicyLike{
		nil, nilPolicy,
		&policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a/other"}},
	} {
		_, errs := ScopePolicy(policy)
		require.NotEmpty(t, errs)
	}
}
