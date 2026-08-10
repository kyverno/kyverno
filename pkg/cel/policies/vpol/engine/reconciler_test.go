package engine

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// fakeClient serves a fixed ValidatingPolicy by name so the reconciler can be driven without an API
// server.
type fakeClient struct {
	client.Client
	policy *policiesv1beta1.ValidatingPolicy
}

func (f *fakeClient) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	if vp, ok := obj.(*policiesv1beta1.ValidatingPolicy); ok && f.policy != nil && key.Name == f.policy.Name {
		*vp = *f.policy
		return nil
	}
	return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
}

// disallowLatestTagPolicy matches Pods and autogens onto both a built-in kind (deployments) and a
// custom workload CRD named by the explicit "<resource>.<version>.<group>" format, which does not
// need a live cluster to resolve.
func disallowLatestTagPolicy() *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "disallow-latest-tag"},
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
			AutogenConfiguration: &policiesv1beta1.ValidatingPolicyAutogenConfiguration{
				PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
					Controllers: []string{"deployments", "jobsets.v1alpha2.jobset.x-k8s.io"},
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "object.spec.containers.all(c, !c.image.endsWith(':latest'))"},
			},
		},
	}
}

// TestReconcile_ExtractionMode is the regression test for the gap that let the JobSet extraction
// mechanism ship without actually working end to end: provider.go (used by NewProvider, the CLI/test
// path) set Policy.ExtractionMode correctly, but reconciler.go (used by the live cluster controller)
// had its own separate copy of the same loop that never set it, so the live admission path evaluated
// the custom-CRD target directly against the real object instead of extracting its pod template -
// exactly the "no such key: containers" failure this test would have caught.
func TestReconcile_ExtractionMode(t *testing.T) {
	ctx := context.Background()
	rec := newReconciler(
		compiler.NewCompiler(),
		&fakeClient{policy: disallowLatestTagPolicy()},
		nil, false,
	)

	_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "disallow-latest-tag"}})
	require.NoError(t, err)

	fetched, err := rec.Fetch(ctx)
	require.NoError(t, err)
	require.Len(t, fetched, 3, "expected the base policy plus one autogen'd variant per ReplacementsRef group")

	var sawDeploymentVariant, sawJobSetVariant bool
	for _, p := range fetched {
		mc := p.Policy.GetValidatingPolicySpec().MatchConstraints
		if mc == nil || len(mc.ResourceRules) == 0 {
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
