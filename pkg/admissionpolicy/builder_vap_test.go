package admissionpolicy

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestBuildValidatingAdmissionPolicy_FromValidatingPolicy covers the previously
// untested validating-side builder: a CEL ValidatingPolicy is converted into a
// native ValidatingAdmissionPolicy. Its match constraints, validations, variables
// and match conditions must carry over, and the VAP must own-reference the source
// policy. The vpol branch does not use discovery, so a nil discovery client is
// sufficient.
func TestBuildValidatingAdmissionPolicy_FromValidatingPolicy(t *testing.T) {
	vpol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "require-env-label", UID: "vpol-uid"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				}},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{Name: "only-default", Expression: "object.metadata.namespace == 'default'"},
			},
			Variables: []admissionregistrationv1.Variable{
				{Name: "hasEnv", Expression: "has(object.metadata.labels) && 'env' in object.metadata.labels"},
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "variables.hasEnv", Message: "env label is required"},
			},
		},
	}

	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "vpol-require-env-label"}}
	err := BuildValidatingAdmissionPolicy(nil, vap, engineapi.NewValidatingPolicy(vpol), nil)
	require.NoError(t, err)

	require.NotNil(t, vap.Spec.MatchConstraints)
	require.Len(t, vap.Spec.MatchConstraints.ResourceRules, 1)
	assert.Equal(t, []string{"pods"}, vap.Spec.MatchConstraints.ResourceRules[0].Resources)

	require.Len(t, vap.Spec.Validations, 1)
	assert.Equal(t, "variables.hasEnv", vap.Spec.Validations[0].Expression)
	assert.Equal(t, "env label is required", vap.Spec.Validations[0].Message)

	require.Len(t, vap.Spec.Variables, 1)
	assert.Equal(t, "hasEnv", vap.Spec.Variables[0].Name)

	require.Len(t, vap.Spec.MatchConditions, 1)
	assert.Equal(t, "only-default", vap.Spec.MatchConditions[0].Name)

	require.Len(t, vap.OwnerReferences, 1)
	assert.Equal(t, "require-env-label", vap.OwnerReferences[0].Name)
	assert.Equal(t, vpol.UID, vap.OwnerReferences[0].UID)
	assert.Contains(t, vap.Labels, "app.kubernetes.io/managed-by")
}

// TestBuildValidatingAdmissionPolicy_NegatesCELException verifies the security-
// relevant conversion: a PolicyException's match condition must be negated and
// added to the generated VAP, so the VAP skips exactly the requests the exception
// exempts. A sign error here would either over-exempt (bypass) or under-exempt.
func TestBuildValidatingAdmissionPolicy_NegatesCELException(t *testing.T) {
	vpol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-all"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
			Validations:      []admissionregistrationv1.Validation{{Expression: "false", Message: "denied"}},
		},
	}
	celException := &policiesv1beta1.PolicyException{
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{Name: "skip-kube-system", Expression: "object.metadata.namespace == 'kube-system'"},
			},
		},
	}

	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "vpol-deny-all"}}
	err := BuildValidatingAdmissionPolicy(nil, vap, engineapi.NewValidatingPolicy(vpol),
		[]engineapi.GenericException{engineapi.NewCELPolicyException(celException)})
	require.NoError(t, err)

	require.Len(t, vap.Spec.MatchConditions, 1)
	assert.Equal(t, "skip-kube-system", vap.Spec.MatchConditions[0].Name)
	assert.Equal(t, "!(object.metadata.namespace == 'kube-system')", vap.Spec.MatchConditions[0].Expression)
}

// TestBuildValidatingAdmissionPolicyBinding_FromVpol checks the binding carries the
// policy's enforcement action and references the VAP by the name the builder gives
// it (the vpol- prefixed name), so the binding is not left pointing at nothing.
func TestBuildValidatingAdmissionPolicyBinding_FromVpol(t *testing.T) {
	vpol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-all", UID: "u"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: "vpol-deny-all-binding"}}
	err := BuildValidatingAdmissionPolicyBinding(binding, engineapi.NewValidatingPolicy(vpol))
	require.NoError(t, err)

	assert.Equal(t, "vpol-deny-all", binding.Spec.PolicyName)
	assert.Equal(t, []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}, binding.Spec.ValidationActions)
	require.Len(t, binding.OwnerReferences, 1)
	assert.Equal(t, "deny-all", binding.OwnerReferences[0].Name)
}

// TestCanGenerateVAP_RuleCountBoundary pins the eligibility gate the generator
// relies on: a policy must have exactly one rule to be convertible to a single
// VAP. Zero or multiple rules must be reported as ineligible with a clear reason.
func TestCanGenerateVAP_RuleCountBoundary(t *testing.T) {
	ok, msg := CanGenerateVAP(&kyvernov1.Spec{}, nil, true)
	assert.False(t, ok)
	assert.Contains(t, msg, "no rules")

	ok, msg = CanGenerateVAP(&kyvernov1.Spec{Rules: []kyvernov1.Rule{{Name: "a"}, {Name: "b"}}}, nil, true)
	assert.False(t, ok)
	assert.Contains(t, msg, "multiple rules")
}
