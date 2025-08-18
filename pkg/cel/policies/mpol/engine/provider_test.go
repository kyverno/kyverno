package engine

import (
	"context"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	admissionv1 "k8s.io/apiserver/pkg/admission"

	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type fakeCompiledPolicy struct{}

func (f *fakeCompiledPolicy) MatchesConditions(_ context.Context, _ admissionv1.Attributes, _ *corev1.Namespace) bool {
	return true
}

func TestNewProvider(t *testing.T) {

	tests := []struct {
		name          string
		pols          []policiesv1alpha1.MutatingPolicy
		exceptions    []*policiesv1alpha1.PolicyException
		expectErr     bool
		expectedCount int
	}{
		{
			name: "valid policy without exception",
			pols: []policiesv1alpha1.MutatingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "policy1"},
				},
			},
			expectErr:     false,
			expectedCount: 1, // includes autogen clone
		},
		{
			name: "policy with matching exception",
			pols: []policiesv1alpha1.MutatingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "policy-exc"},
				},
			},
			exceptions: []*policiesv1alpha1.PolicyException{
				{
					Spec: policiesv1alpha1.PolicyExceptionSpec{
						PolicyRefs: []policiesv1alpha1.PolicyRef{
							{
								Name: "policy-exc",
								Kind: "MutatingPolicy",
							},
						},
					},
				},
			},
			expectErr:     false,
			expectedCount: 1, // includes autogen clone
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, err := NewProvider(compiler.NewCompiler(), tt.pols, tt.exceptions)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, prov)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, prov)

				pols, err := prov.Fetch(context.Background(), false)
				assert.NoError(t, err)
				assert.GreaterOrEqual(t, len(pols), tt.expectedCount)
			}
		})
	}
}

func TestStaticProviderFetch(t *testing.T) {
	trueBool := true
	falseBool := false

	policy1 := policiesv1alpha1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "enabled-policy"},
		Spec: policiesv1alpha1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1alpha1.MutatingPolicyEvaluationConfiguration{
				MutateExistingConfiguration: &policiesv1alpha1.MutateExistingConfiguration{
					Enabled: &trueBool,
				},
			},
		},
	}
	policy2 := policiesv1alpha1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "disabled-policy"},
		Spec: policiesv1alpha1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1alpha1.MutatingPolicyEvaluationConfiguration{
				MutateExistingConfiguration: &policiesv1alpha1.MutateExistingConfiguration{
					Enabled: &falseBool,
				},
			},
		},
	}

	provider := &staticProvider{
		policies: []Policy{
			{Policy: policy1},
			{Policy: policy2},
		},
	}

	t.Run("fetch mutateExisting == true", func(t *testing.T) {
		res, err := provider.Fetch(context.TODO(), true)
		assert.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, "enabled-policy", res[0].Policy.GetName())
	})

	t.Run("fetch mutateExisting == false", func(t *testing.T) {
		res, err := provider.Fetch(context.TODO(), false)
		assert.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, "disabled-policy", res[0].Policy.GetName())
	})
}

func TestStaticProviderMatchesMutateExisting(t *testing.T) {
	trueBool := true

	provider := &staticProvider{
		policies: []Policy{
			{
				Policy: policiesv1alpha1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "match"},
					Spec: policiesv1alpha1.MutatingPolicySpec{
						EvaluationConfiguration: &policiesv1alpha1.MutatingPolicyEvaluationConfiguration{
							MutateExistingConfiguration: &policiesv1alpha1.MutateExistingConfiguration{
								Enabled: &trueBool,
							},
						},
						MatchConstraints: &admissionregistrationv1alpha1.MatchResources{}, // match everything
					},
				},
				CompiledPolicy: &compiler.Policy{},
			},
		},
	}

	t.Run("match all", func(t *testing.T) {
		names := provider.MatchesMutateExisting(context.Background(), &mockAttributes{}, &corev1.Namespace{})
		assert.Equal(t, []string{"match"}, names)
	})
}
