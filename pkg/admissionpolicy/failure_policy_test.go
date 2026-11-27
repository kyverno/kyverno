package admissionpolicy

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type builderForceFailurePolicyIgnoreToggles struct {
	toggle.Toggles
}

func (builderForceFailurePolicyIgnoreToggles) ForceFailurePolicyIgnore() bool { return true }

// TestBuildNativePoliciesWithEffectiveFailurePolicy checks that the native policy builders
// emit the failure policy the generator resolves with the forceFailurePolicyIgnore toggle.
func TestBuildNativePoliciesWithEffectiveFailurePolicy(t *testing.T) {
	explicitFail := admissionregistrationv1.Fail
	tests := []struct {
		name          string
		forceIgnore   bool
		failurePolicy *admissionregistrationv1.FailurePolicyType
		want          admissionregistrationv1.FailurePolicyType
	}{
		{
			name:        "force ignore with no failurePolicy",
			forceIgnore: true,
			want:        admissionregistrationv1.Ignore,
		},
		{
			name: "no force with no failurePolicy",
			want: admissionregistrationv1.Fail,
		},
		{
			name:          "force ignore overrides explicit fail",
			forceIgnore:   true,
			failurePolicy: &explicitFail,
			want:          admissionregistrationv1.Ignore,
		},
		{
			name:          "no force preserves explicit fail",
			failurePolicy: &explicitFail,
			want:          admissionregistrationv1.Fail,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.forceIgnore {
				ctx = toggle.NewContext(ctx, builderForceFailurePolicyIgnoreToggles{toggle.FromContext(ctx)})
			}
			forceIgnore := toggle.FromContext(ctx).ForceFailurePolicyIgnore()

			vpol := &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-vpol"},
				Spec: policiesv1beta1.ValidatingPolicySpec{
					FailurePolicy:    tt.failurePolicy,
					MatchConstraints: &admissionregistrationv1.MatchResources{},
					Validations:      []admissionregistrationv1.Validation{{Expression: "true"}},
				},
			}
			// The generator resolves the effective policy and builds from a copy.
			vpol.Spec.FailurePolicy = ptr.To(vpol.GetFailurePolicy(forceIgnore))
			vap := &admissionregistrationv1.ValidatingAdmissionPolicy{}
			require.NoError(t, BuildValidatingAdmissionPolicy(nil, vap, engineapi.NewValidatingPolicy(vpol), nil))
			require.NotNil(t, vap.Spec.FailurePolicy)
			assert.Equal(t, tt.want, *vap.Spec.FailurePolicy)

			mpol := &policiesv1beta1.MutatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-mpol"},
				Spec: policiesv1beta1.MutatingPolicySpec{
					FailurePolicy:    tt.failurePolicy,
					MatchConstraints: &admissionregistrationv1.MatchResources{},
					Mutations: []admissionregistrationv1alpha1.Mutation{{
						PatchType:          admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: "Object{}"},
					}},
				},
			}
			mpol.Spec.FailurePolicy = ptr.To(mpol.GetFailurePolicy(forceIgnore))
			mapol := &admissionregistrationv1.MutatingAdmissionPolicy{}
			BuildMutatingAdmissionPolicyV1(mapol, mpol, nil)
			require.NotNil(t, mapol.Spec.FailurePolicy)
			assert.Equal(t, tt.want, *mapol.Spec.FailurePolicy)
		})
	}
}
