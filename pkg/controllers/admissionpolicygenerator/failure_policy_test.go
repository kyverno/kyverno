package admissionpolicygenerator

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type forceFailurePolicyIgnoreToggles struct {
	toggle.Toggles
}

func (forceFailurePolicyIgnoreToggles) ForceFailurePolicyIgnore() bool { return true }

func withForceFailurePolicyIgnore(ctx context.Context) context.Context {
	return toggle.NewContext(ctx, forceFailurePolicyIgnoreToggles{toggle.FromContext(ctx)})
}

// nativeFailurePolicyCase describes the failure policy an already generated native policy carries
// and the one reconciliation must leave on it. The existing value always differs from the expected
// one, so the assertions fail unless the update path rewrites the failure policy.
type nativeFailurePolicyCase struct {
	name                  string
	forceFailurePolicy    bool
	sourceFailurePolicy   *admissionregistrationv1.FailurePolicyType
	existingFailurePolicy admissionregistrationv1.FailurePolicyType
	wantFailurePolicy     admissionregistrationv1.FailurePolicyType
}

var nativeFailurePolicyCases = []nativeFailurePolicyCase{
	{
		name:                  "force ignore enabled, source failure policy unset",
		forceFailurePolicy:    true,
		existingFailurePolicy: admissionregistrationv1.Fail,
		wantFailurePolicy:     admissionregistrationv1.Ignore,
	},
	{
		name:                  "force ignore enabled, source failure policy explicitly Fail",
		forceFailurePolicy:    true,
		sourceFailurePolicy:   ptr.To(admissionregistrationv1.Fail),
		existingFailurePolicy: admissionregistrationv1.Fail,
		wantFailurePolicy:     admissionregistrationv1.Ignore,
	},
	{
		name:                  "force ignore disabled, source failure policy explicitly Fail",
		sourceFailurePolicy:   ptr.To(admissionregistrationv1.Fail),
		existingFailurePolicy: admissionregistrationv1.Ignore,
		wantFailurePolicy:     admissionregistrationv1.Fail,
	},
	{
		name:                  "no force with unset source and existing Ignore resolves to Fail",
		existingFailurePolicy: admissionregistrationv1.Ignore,
		wantFailurePolicy:     admissionregistrationv1.Fail,
	},
}

// TestHandleVAPGeneration_ForceFailurePolicyIgnore checks that a generated ValidatingAdmissionPolicy
// carries the failure policy selected by the toggle.
func TestHandleVAPGeneration_ForceFailurePolicyIgnore(t *testing.T) {
	ctx := withForceFailurePolicyIgnore(context.Background())
	vpol := disallowPrivilegeEscalationVpol(nil)
	vpol.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
	vpol.Spec.FailurePolicy = ptr.To(admissionregistrationv1.Fail)
	sourceSnapshot := vpol.DeepCopy()
	c, kubeClient, _ := newVAPTestController(t, vpol)

	require.NoError(t, c.handleVAPGeneration(ctx, engineapi.NewValidatingPolicy(vpol)))

	vap, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, "vpol-disallow-privilege-escalation", metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, vap.Spec.FailurePolicy)
	assert.Equal(t, admissionregistrationv1.Ignore, *vap.Spec.FailurePolicy)
	assert.Equal(t, sourceSnapshot, vpol, "reconciliation must not mutate the source ValidatingPolicy")
}

// TestHandleVAPGeneration_ExistingVAPFailurePolicy checks that reconciliation rewrites the failure
// policy of an already generated ValidatingAdmissionPolicy to the effective value, and never writes
// that value back into the informer-owned ValidatingPolicy.
func TestHandleVAPGeneration_ExistingVAPFailurePolicy(t *testing.T) {
	for _, tt := range nativeFailurePolicyCases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.forceFailurePolicy {
				ctx = withForceFailurePolicyIgnore(ctx)
			}
			vpol := disallowPrivilegeEscalationVpol(nil)
			vpol.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
			vpol.Spec.FailurePolicy = tt.sourceFailurePolicy
			sourceSnapshot := vpol.DeepCopy()

			existingVAP := &admissionregistrationv1.ValidatingAdmissionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "vpol-disallow-privilege-escalation",
					ResourceVersion: "1",
				},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
					FailurePolicy: ptr.To(tt.existingFailurePolicy),
				},
			}
			existingBinding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "vpol-disallow-privilege-escalation-binding",
					ResourceVersion: "1",
				},
			}
			c, kubeClient, _ := newVAPTestController(t, vpol, existingVAP, existingBinding)

			require.NoError(t, c.handleVAPGeneration(ctx, engineapi.NewValidatingPolicy(vpol)))

			vap, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, existingVAP.Name, metav1.GetOptions{})
			require.NoError(t, err)
			require.NotNil(t, vap.Spec.FailurePolicy)
			assert.Equal(t, tt.wantFailurePolicy, *vap.Spec.FailurePolicy)
			assert.Equal(t, sourceSnapshot, vpol, "reconciliation must not mutate the source ValidatingPolicy")
		})
	}
}

// TestHandleMAPGeneration_ForceFailurePolicyIgnore checks that a generated MutatingAdmissionPolicy
// carries the failure policy selected by the toggle.
func TestHandleMAPGeneration_ForceFailurePolicyIgnore(t *testing.T) {
	ctx := withForceFailurePolicyIgnore(context.Background())
	mpol := testMutatingPolicy()
	mpol.Spec.FailurePolicy = ptr.To(admissionregistrationv1.Fail)
	sourceSnapshot := mpol.DeepCopy()
	c, kubeClient, _ := newMAPTestController(t, mpol, nil, nil)

	require.NoError(t, c.handleMAPGeneration(ctx, mpol))

	mapol, err := kubeClient.AdmissionregistrationV1beta1().MutatingAdmissionPolicies().Get(ctx, "mpol-test-mutate-pods", metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, mapol.Spec.FailurePolicy)
	assert.Equal(t, admissionregistrationv1beta1.Ignore, *mapol.Spec.FailurePolicy)
	assert.Equal(t, sourceSnapshot, mpol, "reconciliation must not mutate the source MutatingPolicy")
}

// TestHandleMAPGeneration_ExistingMAPFailurePolicy checks that reconciliation rewrites the failure
// policy of an already generated MutatingAdmissionPolicy to the effective value, and never writes
// that value back into the informer-owned MutatingPolicy.
func TestHandleMAPGeneration_ExistingMAPFailurePolicy(t *testing.T) {
	for _, tt := range nativeFailurePolicyCases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.forceFailurePolicy {
				ctx = withForceFailurePolicyIgnore(ctx)
			}
			mpol := testMutatingPolicy()
			mpol.Spec.FailurePolicy = tt.sourceFailurePolicy
			sourceSnapshot := mpol.DeepCopy()

			existingMAP := &admissionregistrationv1beta1.MutatingAdmissionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mpol-test-mutate-pods",
					ResourceVersion: "1",
				},
				Spec: admissionregistrationv1beta1.MutatingAdmissionPolicySpec{
					FailurePolicy: ptr.To(admissionregistrationv1beta1.FailurePolicyType(tt.existingFailurePolicy)),
				},
			}
			existingBinding := &admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "mpol-test-mutate-pods-binding",
					ResourceVersion: "1",
				},
			}
			c, kubeClient, _ := newMAPTestController(t, mpol, existingMAP, existingBinding)

			require.NoError(t, c.handleMAPGeneration(ctx, mpol))

			mapol, err := kubeClient.AdmissionregistrationV1beta1().MutatingAdmissionPolicies().Get(ctx, existingMAP.Name, metav1.GetOptions{})
			require.NoError(t, err)
			require.NotNil(t, mapol.Spec.FailurePolicy)
			assert.Equal(t, admissionregistrationv1beta1.FailurePolicyType(tt.wantFailurePolicy), *mapol.Spec.FailurePolicy)
			assert.Equal(t, sourceSnapshot, mpol, "reconciliation must not mutate the source MutatingPolicy")
		})
	}
}

// testMutatingPolicy is a pods-only MutatingPolicy with MAP generation enabled and pod-controller
// autogen turned off, so generation is not skipped.
func testMutatingPolicy() *policiesv1beta1.MutatingPolicy {
	cfg := mapGenEnabled()
	cfg.PodControllers = &policiesv1beta1.PodControllersGenerationConfiguration{Controllers: []string{}}
	return &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-mutate-pods"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			AutogenConfiguration: cfg,
			MatchConstraints:     podsMatchConstraints(),
		},
	}
}
