package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
)

type fakeCompiledPolicy struct{}

func (f *fakeCompiledPolicy) MatchesConditions(_ context.Context, _ admission.Attributes, _ *admissionv1.AdmissionRequest, _ *corev1.Namespace) bool {
	return true
}

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name          string
		pols          []policiesv1beta1.MutatingPolicyLike
		exceptions    []*policiesv1beta1.PolicyException
		expectErr     bool
		expectedCount int
	}{
		{
			name: "valid policy without exception",
			pols: []policiesv1beta1.MutatingPolicyLike{
				&policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "policy1"},
				},
			},
			expectErr:     false,
			expectedCount: 1, // includes autogen clone
		},
		{
			name: "policy with matching exception",
			pols: []policiesv1beta1.MutatingPolicyLike{
				&policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "policy-exc"},
				},
			},
			exceptions: []*policiesv1beta1.PolicyException{
				{
					Spec: policiesv1beta1.PolicyExceptionSpec{
						PolicyRefs: []policiesv1beta1.PolicyRef{
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
			prov, err := NewProvider(compiler.NewCompiler(), tt.pols, tt.exceptions, libs.NewFakeContextProvider())
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, prov)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, prov)

				pols := prov.Fetch(context.Background(), false)
				assert.GreaterOrEqual(t, len(pols), tt.expectedCount)
			}
		})
	}
}

func TestStaticProviderFetch(t *testing.T) {
	trueBool := true
	falseBool := false

	policy1 := policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "enabled-policy"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
				MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
					Enabled: &trueBool,
				},
			},
		},
	}
	policy2 := policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "disabled-policy"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
				MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
					Enabled: &falseBool,
				},
			},
		},
	}

	provider := &staticProvider{
		policies: []Policy{
			{Policy: &policy1},
			{Policy: &policy2},
		},
	}

	// The static provider must mirror the cluster reconciler's Fetch semantics:
	// mutateExisting=true returns only the mutate-existing policies, while
	// mutateExisting=false returns all policies (a mutate-existing policy still
	// runs on admission by default). Filtering false down to non-mutate-existing
	// policies drops them from the CLI admission path and from the background
	// reports scanner, which both build this provider and call Fetch(ctx, false).
	t.Run("fetch mutateExisting == true returns only mutate-existing policies", func(t *testing.T) {
		res := provider.Fetch(context.TODO(), true)
		assert.Len(t, res, 1)
		assert.Equal(t, "enabled-policy", res[0].Policy.GetName())
	})

	t.Run("fetch mutateExisting == false returns all policies", func(t *testing.T) {
		res := provider.Fetch(context.TODO(), false)
		var names []string
		for _, p := range res {
			names = append(names, p.Policy.GetName())
		}
		assert.Len(t, res, 2)
		assert.Contains(t, names, "enabled-policy")
		assert.Contains(t, names, "disabled-policy")
	})
}

func TestStaticProviderMatchesMutateExisting(t *testing.T) {
	trueBool := true

	matchPolicy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "match"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
				MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
					Enabled: &trueBool,
				},
			},
			MatchConstraints: &admissionregistrationv1.MatchResources{}, // match everything
		},
	}
	provider := &staticProvider{
		policies: []Policy{
			{
				Policy:         matchPolicy,
				CompiledPolicy: &compiler.Policy{},
			},
		},
	}

	t.Run("match all", func(t *testing.T) {
		names := provider.MatchesMutateExisting(context.Background(), &mockAttributes{}, nil, &corev1.Namespace{}, nil)
		assert.Equal(t, []string{"match"}, names)
	})
}

// TestStaticProviderMatchesMutateExisting_BuildsRequestMapAtMostOnce is the
// CI-assertable regression guard for the mutate-existing matching hoist:
// with N candidate policies that all have matchConditions (so
// MatchesConditions, and therefore prepareData, is actually invoked for
// each one), the caller-supplied requestMapFn - a real sync.OnceValues
// memoized thunk, exactly what engineImpl.MatchedMutateExistingPolicies
// builds - must be invoked at most once across the whole matching pass, not
// once per policy.
func TestStaticProviderMatchesMutateExisting_BuildsRequestMapAtMostOnce(t *testing.T) {
	trueBool := true
	const n = 5

	comp := compiler.NewCompiler()
	policies := make([]Policy, 0, n)
	for i := 0; i < n; i++ {
		mpol := &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("match-%d", i)},
			Spec: policiesv1beta1.MutatingPolicySpec{
				EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
					MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
						Enabled: &trueBool,
					},
				},
				MatchConstraints: &admissionregistrationv1.MatchResources{}, // match everything
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{Name: "always-true", Expression: "true"},
				},
			},
		}
		compiled, errs := comp.Compile(mpol, nil)
		require.Empty(t, errs)
		policies = append(policies, Policy{Policy: mpol, CompiledPolicy: compiled})
	}
	provider := &staticProvider{policies: policies}

	request := &admissionv1.AdmissionRequest{
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Name:      "nginx",
		Namespace: "default",
		Operation: admissionv1.Create,
	}

	calls := 0
	requestMapFn := sync.OnceValues(func() (map[string]any, error) {
		calls++
		return celcompiler.BuildNormalizedRequestMap(request)
	})

	names := provider.MatchesMutateExisting(context.Background(), &mockAttributes{}, request, &corev1.Namespace{}, requestMapFn)
	assert.Len(t, names, n, "all %d policies must have matched", n)
	assert.Equal(t, 1, calls,
		"requestMapFn must be invoked at most once across all %d candidate policies, not once per policy", n)
}
