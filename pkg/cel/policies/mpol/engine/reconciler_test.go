package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Mocks

type fakeClient struct {
	client.Client
	policy *policiesv1alpha1.MutatingPolicy
	err    error
}

func (f *fakeClient) Get(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	if f.err != nil {
		return f.err
	}
	*obj.(*policiesv1alpha1.MutatingPolicy) = *f.policy
	return nil
}

func TestReconcile(t *testing.T) {
	ctx := context.Background()
	name := types.NamespacedName{Namespace: "default", Name: "test-policy"}

	//t.Run("policy not found", func(t *testing.T) {
	//	rec := newReconciler(
	//		&fakeClient{err: errors.ErrUnsupported(policiesv1alpha1.Resource("mutatingpolicies"), "test-policy")},
	//		compiler.NewCompiler(), nil, false,
	///	)
	//	res, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: name})
	//	assert.NoError(t, err)
	//	assert.Equal(t, reconcile.Result{}, res)
	//})

	t.Run("successful reconciliation", func(t *testing.T) {
		mp := &policiesv1alpha1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "test-policy", Namespace: "default"},
		}
		rec := newReconciler(
			&fakeClient{policy: mp},
			compiler.NewCompiler(),
			nil, false,
		)
		res, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: name})
		assert.NoError(t, err)
		assert.Equal(t, reconcile.Result{}, res)
	})
}

func TestFetch(t *testing.T) {
	trueBool := true
	falseBool := false

	tests := []struct {
		name           string
		mutateExisting bool
		policyMap      map[string][]Policy
		expectedNames  []string
	}{
		{
			name:           "no policies",
			mutateExisting: false,
			policyMap:      map[string][]Policy{},
			expectedNames:  []string{},
		},
		{
			name:           "mutateExisting = false, return all policies",
			mutateExisting: false,
			policyMap: map[string][]Policy{
				"ns1/policy1": {
					{
						Policy: policiesv1alpha1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy1"},
						},
					},
					{
						Policy: policiesv1alpha1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy2"},
						},
					},
				},
			},
			expectedNames: []string{"policy1", "policy2"},
		},
		{
			name:           "mutateExisting = true, only enabled ones returned",
			mutateExisting: true,
			policyMap: map[string][]Policy{
				"ns1/policy1": {
					{
						Policy: policiesv1alpha1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy1"},
							Spec: policiesv1alpha1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1alpha1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1alpha1.MutateExistingConfiguration{
										Enabled: &trueBool,
									},
								},
							},
						},
					},
					{
						Policy: policiesv1alpha1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy2"},
							Spec: policiesv1alpha1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1alpha1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1alpha1.MutateExistingConfiguration{
										Enabled: &falseBool,
									},
								},
							},
						},
					},
				},
			},
			expectedNames: []string{"policy1"},
		},
		{
			name:           "mutateExisting = true, all disabled",
			mutateExisting: true,
			policyMap: map[string][]Policy{
				"ns1/policy2": {
					{
						Policy: policiesv1alpha1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy2"},
							Spec: policiesv1alpha1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1alpha1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1alpha1.MutateExistingConfiguration{
										Enabled: &falseBool,
									},
								},
							},
						},
					},
				},
			},
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &reconciler{
				policies: tt.policyMap,
				lock:     &sync.RWMutex{},
			}

			got, err := r.Fetch(context.Background(), tt.mutateExisting)

			assert.NoError(t, err)

			var gotNames []string
			for _, p := range got {
				gotNames = append(gotNames, p.Policy.GetName())
			}
			assert.ElementsMatch(t, tt.expectedNames, gotNames)
		})
	}
}

type fakeFetchWithError struct {
	*reconciler
}

func (f *fakeFetchWithError) Fetch(ctx context.Context, mutateExisting bool) ([]Policy, error) {
	return nil, fmt.Errorf("simulated fetch error")
}

func TestMatchesMutateExisting(t *testing.T) {
	trueBool := true

	tests := []struct {
		name          string
		policies      map[string][]Policy
		expectedNames []string
	}{
		{
			name: "single policy matches with conditions true",
			policies: map[string][]Policy{
				"test/policy1": {
					{
						Policy: policiesv1alpha1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy1"},
							Spec: policiesv1alpha1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1alpha1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1alpha1.MutateExistingConfiguration{
										Enabled: &trueBool,
									},
								},
								MatchConstraints: &admissionregistrationv1alpha1.MatchResources{}, // empty constraints should match
								MatchConditions:  nil,                                             // no conditions
							},
						},
						CompiledPolicy: &compiler.Policy{},
					},
				},
			},
			expectedNames: []string{"policy1"},
		},
		{
			name: "policy with conditions that fail",
			policies: map[string][]Policy{
				"test/policy2": {
					{
						Policy: policiesv1alpha1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy2"},
							Spec: policiesv1alpha1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1alpha1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1alpha1.MutateExistingConfiguration{
										Enabled: &trueBool,
									},
								},
								MatchConstraints: &admissionregistrationv1alpha1.MatchResources{},
								MatchConditions: []admissionregistrationv1alpha1.MatchCondition{
									{
										Expression: `request.object.metadata.labels.env == "dev"`,
									},
								},
							},
						},
						CompiledPolicy: &compiler.Policy{},
					},
				},
			},
			expectedNames: []string{},
		},
		{
			name: "no mutateExisting enabled, nothing matched",
			policies: map[string][]Policy{
				"test/policy3": {
					{
						Policy: policiesv1alpha1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy3"},
						},
					},
				},
			},
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &reconciler{
				lock:     &sync.RWMutex{},
				policies: tt.policies,
			}
			attrs := &mockAttributes{}
			namespace := &corev1.Namespace{}
			got := r.MatchesMutateExisting(context.TODO(), attrs, namespace)
			assert.ElementsMatch(t, tt.expectedNames, got)
		})
	}
}
