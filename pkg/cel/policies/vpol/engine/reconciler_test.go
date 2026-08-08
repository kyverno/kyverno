package engine

import (
	"context"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// fakeVpolClient serves a single ValidatingPolicy by name and a fixed list of
// PolicyExceptions, standing in for a controller-runtime manager cache that has
// already observed both objects (e.g. because it is the same cache that delivered
// the watch event which triggered reconciliation).
type fakeVpolClient struct {
	client.Client
	policy     *policiesv1beta1.ValidatingPolicy
	exceptions []policiesv1beta1.PolicyException
}

func (f *fakeVpolClient) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	if f.policy != nil && key.Name == f.policy.Name {
		if o, ok := obj.(*policiesv1beta1.ValidatingPolicy); ok {
			*o = *f.policy
			return nil
		}
	}
	return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
}

func (f *fakeVpolClient) List(_ context.Context, list client.ObjectList, _ ...client.ListOption) error {
	if l, ok := list.(*policiesv1beta1.PolicyExceptionList); ok {
		l.Items = f.exceptions
		return nil
	}
	return nil
}

// staleExceptionLister always reports no exceptions. It stands in for the
// pre-fix wiring, where exceptions were read through a separate client-go
// SharedInformerFactory lister that is not guaranteed to have caught up with the
// controller-runtime manager cache that triggered the reconcile (issue #16989).
type staleExceptionLister struct{}

func (staleExceptionLister) List(labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	return nil, nil
}

// denyMissingTeamLabelPolicy is a JSON-mode ValidatingPolicy that fails any payload
// without a "team" field.
func denyMissingTeamLabelPolicy(name string) *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeJSON,
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "has(object.team)", Message: "object must carry a team field"},
			},
		},
	}
}

// fullExemption is a PolicyException that unconditionally exempts every resource
// from policyName: no MatchConditions (always matches) and no Images/AllowedValues
// scoping (grants a full exemption rather than a partial CEL-visible allowlist).
func fullExemption(name, policyName string) policiesv1beta1.PolicyException {
	return policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{
				Name: policyName,
				Kind: policieskyvernoio.ValidatingPolicyKind,
			}},
		},
	}
}

// TestReconcile_ExceptionLister_StaleCacheMissesException reproduces the bug in
// issue #16989: when exceptions are looked up through a lister backed by a cache
// different from (and lagging behind) the one that triggered the reconcile, a
// PolicyException that already exists in the cluster is compiled out of the
// policy and the resource is denied as if the exception did not exist.
func TestReconcile_ExceptionLister_StaleCacheMissesException(t *testing.T) {
	ctx := context.Background()
	policy := denyMissingTeamLabelPolicy("require-team")
	fc := &fakeVpolClient{
		policy:     policy,
		exceptions: []policiesv1beta1.PolicyException{fullExemption("exempt-all", policy.Name)},
	}

	// The stale lister never sees the exception that fc (the reconciling cache)
	// already has, mirroring the dual-cache race.
	rec := newReconciler(vpolcompiler.NewCompiler(), fc, staleExceptionLister{}, true)
	_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: policy.Name}})
	require.NoError(t, err)

	eng := NewEngine(rec, nil, nil)
	payload := &unstructured.Unstructured{Object: map[string]any{"name": "no-team-field"}}
	resp, err := eng.Handle(ctx, celengine.RequestFromJSON(nil, payload), nil)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	require.Len(t, resp.Policies[0].Rules, 1)

	// Bug reproduced: the exception is silently ignored and the rule fails.
	require.Equal(t, engineapi.RuleStatusFail, resp.Policies[0].Rules[0].Status(),
		"a stale exception lister must reproduce the pre-fix behavior of ignoring an existing PolicyException")
}

// TestReconcile_ManagerCacheExceptionLister_SeesException verifies the fix: sourcing
// exceptions from the same manager-cache-backed reader used to fetch the policy
// (celengine.NewManagerPolicyExceptionLister) guarantees a PolicyException that
// exists by the time reconciliation runs is always picked up, because
// controller-runtime only invokes watch handlers after the cache's local store has
// already been updated.
func TestReconcile_ManagerCacheExceptionLister_SeesException(t *testing.T) {
	ctx := context.Background()
	policy := denyMissingTeamLabelPolicy("require-team")
	fc := &fakeVpolClient{
		policy:     policy,
		exceptions: []policiesv1beta1.PolicyException{fullExemption("exempt-all", policy.Name)},
	}

	polexLister := celengine.NewManagerPolicyExceptionLister(fc, "")
	rec := newReconciler(vpolcompiler.NewCompiler(), fc, polexLister, true)
	_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: policy.Name}})
	require.NoError(t, err)

	eng := NewEngine(rec, nil, nil)
	payload := &unstructured.Unstructured{Object: map[string]any{"name": "no-team-field"}}
	resp, err := eng.Handle(ctx, celengine.RequestFromJSON(nil, payload), nil)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	require.Len(t, resp.Policies[0].Rules, 1)

	// Fixed: the exception is honored and the rule is skipped instead of failing.
	require.Equal(t, engineapi.RuleStatusSkip, resp.Policies[0].Rules[0].Status(),
		"the manager-cache-backed lister must honor a PolicyException present in the same cache used to trigger reconciliation")
}
