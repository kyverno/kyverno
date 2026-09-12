package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

// disallowLatestTagMutatingPolicy matches Pods and autogens onto both a built-in kind (deployments)
// and a custom workload CRD named by the explicit "<resource>.<version>.<group>" format, which does
// not need a live cluster to resolve.
func disallowLatestTagMutatingPolicy() *policiesv1beta1.MutatingPolicy {
	return &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "add-team-label"},
		Spec: policiesv1beta1.MutatingPolicySpec{
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
			AutogenConfiguration: &policiesv1beta1.MutatingPolicyAutogenConfiguration{
				PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
					Controllers: []string{"deployments", "jobsets.v1alpha2.jobset.x-k8s.io"},
				},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
					Expression: `Object{spec: Object.spec{containers: object.spec.containers}}`,
				},
			}},
		},
	}
}

// TestReconcile_ExtractionMode is the regression test for the gap that let the mpol extraction wiring
// ship without actually working end to end: provider.go (used by NewProvider, the CLI/test path) set
// Policy.ExtractionMode correctly, but reconciler.go (used by the live cluster controller) had its own
// separate copy of the same loop that never set it, so the live admission path would have tried to
// apply the policy's Pod-shaped mutation directly to the real custom-CRD object instead of skipping it.
func TestReconcile_ExtractionMode(t *testing.T) {
	ctx := context.Background()
	rec := newReconciler(
		&fakeClient{policy: disallowLatestTagMutatingPolicy()},
		compiler.NewCompiler(),
		nil, false,
	)

	_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "add-team-label"}})
	require.NoError(t, err)

	fetched := rec.Fetch(ctx, false)
	require.Len(t, fetched, 3, "expected the base policy plus one autogen'd variant per ReplacementsRef group")

	var sawDeploymentVariant, sawJobSetVariant bool
	for _, p := range fetched {
		mc := p.Policy.GetSpec().MatchConstraints
		if len(mc.ResourceRules) == 0 {
			continue // the base, Pod-targeted policy
		}
		resources := mc.ResourceRules[0].Resources
		switch {
		case len(resources) > 0 && resources[0] == "deployments":
			sawDeploymentVariant = true
			assert.False(t, p.ExtractionMode, "the built-in deployments variant must not be extraction-mode")
		case len(resources) > 0 && resources[0] == "jobsets":
			sawJobSetVariant = true
			assert.True(t, p.ExtractionMode, "the custom-CRD jobsets variant must be extraction-mode")
		}
	}
	assert.True(t, sawDeploymentVariant, "expected a deployments autogen variant")
	assert.True(t, sawJobSetVariant, "expected a jobsets autogen variant")
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
		{
			name: "namespaced policy returns stable policy key",
			policies: map[string][]Policy{
				"default/policy4": {
					{
						Policy: &policiesv1beta1.NamespacedMutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy4", Namespace: "default"},
							Spec: policiesv1beta1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
										Enabled: &trueBool,
									},
								},
								MatchConstraints: &admissionregistrationv1.MatchResources{},
							},
						},
						CompiledPolicy: &compiler.Policy{},
					},
				},
			},
			expectedNames: []string{"default/policy4"},
		},
		{
			name: "admission-disabled policy is never matched by admission requests",
			policies: map[string][]Policy{
				"test/policy5": {
					{
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy5"},
							Spec: policiesv1beta1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
									Admission: &policiesv1beta1.AdmissionConfiguration{Enabled: ptr.To(false)},
									MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
										Enabled: &trueBool,
									},
								},
								MatchConstraints: &admissionregistrationv1.MatchResources{},
							},
						},
						CompiledPolicy: &compiler.Policy{},
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
