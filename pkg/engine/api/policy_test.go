package api

import (
	"encoding/json"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAlphaMutatingAdmissionPolicyMatchResources(t *testing.T) {
	exact := admissionregistrationv1alpha1.Exact
	source := &admissionregistrationv1alpha1.MatchResources{
		NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"environment": "development"}},
		ObjectSelector:    &metav1.LabelSelector{MatchLabels: map[string]string{"mutate": "yes"}},
		ResourceRules: []admissionregistrationv1alpha1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
				Operations: []admissionregistrationv1alpha1.OperationType{admissionregistrationv1alpha1.Create},
				Rule: admissionregistrationv1alpha1.Rule{
					APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"configmaps"},
				},
			},
		}},
		ExcludeResourceRules: []admissionregistrationv1alpha1.NamedRuleWithOperations{{
			ResourceNames: []string{"do-not-touch"},
			RuleWithOperations: admissionregistrationv1alpha1.RuleWithOperations{
				Operations: []admissionregistrationv1alpha1.OperationType{admissionregistrationv1alpha1.Create},
				Rule: admissionregistrationv1alpha1.Rule{
					APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"configmaps"},
				},
			},
		}},
		MatchPolicy: &exact,
	}
	want, err := json.Marshal(source)
	if !assert.NoError(t, err) {
		return
	}
	for _, tc := range []struct {
		name    string
		convert func(*admissionregistrationv1alpha1.MatchResources) *admissionregistrationv1beta1.MatchResources
	}{
		{"policy", func(resources *admissionregistrationv1alpha1.MatchResources) *admissionregistrationv1beta1.MatchResources {
			policy := &admissionregistrationv1alpha1.MutatingAdmissionPolicy{
				Spec: admissionregistrationv1alpha1.MutatingAdmissionPolicySpec{MatchConstraints: resources},
			}
			return NewMutatingAdmissionPolicyAlpha(policy).AsMutatingAdmissionPolicy().GetDefinition().Spec.MatchConstraints
		}},
		{"binding", func(resources *admissionregistrationv1alpha1.MatchResources) *admissionregistrationv1beta1.MatchResources {
			bindings := ConvertMutatingAdmissionPolicyBindingsAlpha([]admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding{{
				Spec: admissionregistrationv1alpha1.MutatingAdmissionPolicyBindingSpec{MatchResources: resources},
			}})
			return bindings[0].Spec.MatchResources
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.convert(source)
			if !assert.NotNil(t, got) {
				return
			}
			converted, err := json.Marshal(got)
			if !assert.NoError(t, err) {
				return
			}
			assert.JSONEq(t, string(want), string(converted))
			got.NamespaceSelector.MatchLabels["environment"] = "production"
			got.ObjectSelector.MatchLabels["mutate"] = "no"
			assert.Equal(t, "development", source.NamespaceSelector.MatchLabels["environment"])
			assert.Equal(t, "yes", source.ObjectSelector.MatchLabels["mutate"])
		})
	}
}

func TestGenericPolicy_NamespacedGeneratingPolicy(t *testing.T) {
	ngpol := &policiesv1beta1.NamespacedGeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ngpol",
			Namespace: "test-ns",
		},
	}

	p := NewNamespacedGeneratingPolicy(ngpol)
	assert.Equal(t, "test-ngpol", p.GetName())
	assert.Equal(t, "test-ns", p.GetNamespace())
	assert.Equal(t, "NamespacedGeneratingPolicy", p.GetKind())
	assert.Equal(t, policiesv1beta1.GroupVersion.String(), p.GetAPIVersion())
	assert.True(t, p.IsNamespaced())
	assert.NotNil(t, p.AsNamespacedGeneratingPolicy())
	assert.NotNil(t, p.AsGeneratingPolicyLike())
	assert.Nil(t, p.AsGeneratingPolicy())
}

func TestGenericPolicy_GeneratingPolicy(t *testing.T) {
	gpol := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gpol",
		},
	}

	p := NewGeneratingPolicy(gpol)
	assert.Equal(t, "test-gpol", p.GetName())
	assert.Equal(t, "", p.GetNamespace())
	assert.Equal(t, "GeneratingPolicy", p.GetKind())
	assert.Equal(t, policiesv1beta1.GroupVersion.String(), p.GetAPIVersion())
	assert.False(t, p.IsNamespaced())
	assert.NotNil(t, p.AsGeneratingPolicy())
	assert.NotNil(t, p.AsGeneratingPolicyLike())
	assert.Nil(t, p.AsNamespacedGeneratingPolicy())
}

func TestGenericPolicy_KyvernoPolicy(t *testing.T) {
	cpol := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cpol",
		},
	}
	p := NewKyvernoPolicy(cpol)
	assert.Equal(t, "ClusterPolicy", p.GetKind())
	assert.Equal(t, kyvernov1.GroupVersion.String(), p.GetAPIVersion())
	assert.False(t, p.IsNamespaced())

	pol := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pol",
			Namespace: "default",
		},
	}
	p = NewKyvernoPolicy(pol)
	assert.Equal(t, "Policy", p.GetKind())
	assert.Equal(t, kyvernov1.GroupVersion.String(), p.GetAPIVersion())
	assert.True(t, p.IsNamespaced())
}

func TestGenericPolicy_CleanupPolicy(t *testing.T) {
	clean := &kyvernov2.CleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-clean",
			Namespace: "default",
		},
	}
	p := NewCleanupPolicyFromInterface(clean)
	assert.Equal(t, "CleanupPolicy", p.GetKind())
	assert.True(t, p.IsNamespaced())

	clusterClean := &kyvernov2.ClusterCleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-clusterclean",
		},
	}
	p = NewCleanupPolicyFromInterface(clusterClean)
	assert.Equal(t, "ClusterCleanupPolicy", p.GetKind())
	assert.False(t, p.IsNamespaced())
}
