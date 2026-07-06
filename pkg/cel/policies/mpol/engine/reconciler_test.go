package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Mocks

type fakeClient struct {
	client.Client
	policy *policiesv1beta1.MutatingPolicy
	nmpol  *policiesv1beta1.NamespacedMutatingPolicy
	err    error
}

func (f *fakeClient) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	if f.err != nil {
		return f.err
	}
	switch o := obj.(type) {
	case *policiesv1beta1.MutatingPolicy:
		if f.policy != nil && key.Name == f.policy.Name && key.Namespace == f.policy.Namespace {
			*o = *f.policy
			return nil
		}
	case *policiesv1beta1.NamespacedMutatingPolicy:
		if f.nmpol != nil && key.Name == f.nmpol.Name && key.Namespace == f.nmpol.Namespace {
			*o = *f.nmpol
			return nil
		}
	}
	return apierrors.NewNotFound(schema.GroupResource{}, "")
}

func TestReconcile(t *testing.T) {
	ctx := context.Background()

	t.Run("successful reconciliation of cluster-scoped MutatingPolicy", func(t *testing.T) {
		mp := &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
		}
		rec := newReconciler(
			&fakeClient{policy: mp},
			compiler.NewCompiler(),
			nil, false,
		)
		// Cluster-scoped: no namespace in request.
		name := types.NamespacedName{Name: "test-policy"}
		res, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: name})
		assert.NoError(t, err)
		assert.Equal(t, reconcile.Result{}, res)
	})

	t.Run("successful reconciliation of NamespacedMutatingPolicy", func(t *testing.T) {
		nmp := &policiesv1beta1.NamespacedMutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "test-nmpol", Namespace: "test-ns"},
		}
		rec := newReconciler(
			&fakeClient{nmpol: nmp},
			compiler.NewCompiler(),
			nil, false,
		)
		// Namespaced: namespace in request.
		name := types.NamespacedName{Namespace: "test-ns", Name: "test-nmpol"}
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
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy1"},
						},
					},
					{
						Policy: &policiesv1beta1.MutatingPolicy{
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
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy1"},
							Spec: policiesv1beta1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
										Enabled: &trueBool,
									},
								},
							},
						},
					},
					{
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy2"},
							Spec: policiesv1beta1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
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
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy2"},
							Spec: policiesv1beta1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
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

			got := r.Fetch(context.Background(), tt.mutateExisting)

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

func TestMatchesMutateExisting_NamespacedPolicyReturnsPolicyKey(t *testing.T) {
	trueBool := true
	comp := compiler.NewCompiler()
	policy := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "rca-cert-rotation", Namespace: "kyverno-mpol-rca"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
				Admission: &policiesv1beta1.AdmissionConfiguration{Enabled: ptr.To(false)},
				MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
					Enabled: &trueBool,
				},
			},
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					ResourceNames: []string{"rca-cert"},
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"secrets"},
						},
					},
				}},
			},
			TargetMatchConstraints: &policiesv1beta1.TargetMatchConstraints{
				MatchResources: admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
						ResourceNames: []string{"rca-sts-a"},
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"statefulsets"},
							},
						},
					}},
				},
			},
		},
	}
	compiled, errs := comp.Compile(policy, nil)
	assert.Empty(t, errs)
	r := &reconciler{
		lock: &sync.RWMutex{},
		policies: map[string][]Policy{
			"kyverno-mpol-rca/rca-cert-rotation": {{Policy: policy, CompiledPolicy: compiled}},
		},
	}

	attr := admission.NewAttributesRecord(
		&unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "rca-cert",
				"namespace": "kyverno-mpol-rca",
			},
		}},
		nil,
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
		"kyverno-mpol-rca",
		"rca-cert",
		schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
		"",
		admission.Update,
		nil,
		false,
		nil,
	)

	got := r.MatchesMutateExisting(context.TODO(), attr, &admissionv1.AdmissionRequest{Operation: admissionv1.Update}, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kyverno-mpol-rca"}})
	assert.Equal(t, []string{"kyverno-mpol-rca/rca-cert-rotation"}, got)
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
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy1"},
							Spec: policiesv1beta1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
										Enabled: &trueBool,
									},
								},
								MatchConstraints: &admissionregistrationv1.MatchResources{}, // empty constraints should match
								MatchConditions:  nil,                                       // no conditions
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
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy2"},
							Spec: policiesv1beta1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
										Enabled: &trueBool,
									},
								},
								MatchConstraints: &admissionregistrationv1.MatchResources{},
								MatchConditions: []admissionregistrationv1.MatchCondition{
									{
										Expression: `object.metadata.labels.env == "dev"`,
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
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy3"},
						},
					},
				},
			},
			expectedNames: []string{},
		},
	}

	comp := compiler.NewCompiler()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, l := range tt.policies {
				for i, p := range l {
					c, _ := comp.Compile(p.Policy, nil)
					tt.policies[k][i].CompiledPolicy = c
				}
			}
			r := &reconciler{
				lock:     &sync.RWMutex{},
				policies: tt.policies,
			}
			attrs := &mockAttributes{}
			namespace := &corev1.Namespace{}
			got := r.MatchesMutateExisting(context.TODO(), attrs, nil, namespace)
			assert.ElementsMatch(t, tt.expectedNames, got)
		})
	}
}
