package admissionpolicygenerator

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

// permissiveAuthChecker always reports the caller as allowed to manage native
// admission policies/bindings, so handleVAPGeneration/handleMAPGeneration reach the
// generation decision under test instead of short-circuiting on RBAC.
type permissiveAuthChecker struct{}

func (permissiveAuthChecker) Check(ctx context.Context, group, version, resource, subresource, namespace, name, verb string) (*checker.AuthResult, error) {
	return &checker.AuthResult{Allowed: true}, nil
}

// newVAPTestController builds a controller wired with fakes sufficient to exercise
// handleVAPGeneration end to end: a real fake kube clientset (so Create/Get on the
// generated VAP/binding are observable), a real fake kyverno clientset seeded with the
// policy (so updatePolicyStatus's UpdateStatus call has something to update), an empty
// (but non-nil, matching the "no exceptions configured" real-world default) CEL
// PolicyException lister, and a permissive auth checker.
func newVAPTestController(t *testing.T, vpol *policiesv1beta1.ValidatingPolicy) (*controller, *kubefake.Clientset, *versionedfake.Clientset) {
	t.Helper()
	kubeClient := kubefake.NewSimpleClientset()
	kyvernoClient := versionedfake.NewSimpleClientset(vpol)
	celpolexIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	// vapLister/vapbindingLister mirror what NewController wires from a started,
	// synced informer factory (controller.go:152) - here backed by empty indexers
	// (no VAP/binding pre-exists), same as a fresh cluster.
	vapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	vapBindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	c := &controller{
		client:           kubeClient,
		kyvernoClient:    kyvernoClient,
		discoveryClient:  dclient.NewEmptyFakeClient().Discovery(),
		eventGen:         event.NewFake(),
		checker:          permissiveAuthChecker{},
		celpolexLister:   policiesv1beta1listers.NewPolicyExceptionLister(celpolexIndexer),
		vapLister:        admissionregistrationv1listers.NewValidatingAdmissionPolicyLister(vapIndexer),
		vapbindingLister: admissionregistrationv1listers.NewValidatingAdmissionPolicyBindingLister(vapBindingIndexer),
	}
	return c, kubeClient, kyvernoClient
}

// disallowPrivilegeEscalationVpol mirrors the real conformance fixture at
// test/conformance/chainsaw/generate-validating-admission-policy/validatingpolicy/autogen-enabled/policy.yaml:
// pod-controller autogen (deployments, cronjobs) + VAP generation both enabled on a
// pods-only ValidatingPolicy.
func disallowPrivilegeEscalationVpol(statusAutogenConfigs map[string]policiesv1beta1.ValidatingPolicyAutogen) *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "disallow-privilege-escalation",
		},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			AutogenConfiguration: &policiesv1beta1.ValidatingPolicyAutogenConfiguration{
				PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
					Controllers: []string{"deployments", "cronjobs"},
				},
				ValidatingAdmissionPolicy: &policiesv1beta1.VapGenerationConfiguration{
					Enabled: ptr.To(true),
				},
			},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit},
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "check-prod-label",
					Expression: `has(object.metadata.labels) && has(object.metadata.labels.prod) && object.metadata.labels.prod == 'true'`,
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: `object.spec.containers.all(container, has(container.securityContext) && has(container.securityContext.allowPrivilegeEscalation) && container.securityContext.allowPrivilegeEscalation == false)`,
					Message:    "Privilege escalation is disallowed. The field spec.containers[*].securityContext.allowPrivilegeEscalation must be set to `false`.",
				},
			},
		},
		Status: policiesv1beta1.ValidatingPolicyStatus{
			Autogen: policiesv1beta1.ValidatingPolicyAutogenStatus{
				Configs: statusAutogenConfigs,
			},
		},
	}
}

// TestHandleVAPGeneration_AutogenStatusRace reproduces the intermittent conformance
// failure at generate-validating-admission-policy/validatingpolicy/autogen-enabled
// (status.generated: true, expected false).
//
// The admissionpolicygenerator controller decides whether pod-controller autogen is in
// effect by reading status.autogen.configs from its lister cache (generate-vap.go), but
// that field is written asynchronously by the SEPARATE policystatus controller. If the
// admissionpolicygenerator reconciles a ValidatingPolicy whose spec already has autogen
// configured but whose status has not been populated yet, it wrongly concludes autogen is
// NOT in effect and generates a ValidatingAdmissionPolicy it should have skipped -
// dropping pod-controller enforcement entirely once status.generated flips to true
// (a generated policy is removed from Kyverno's own engine).
func TestHandleVAPGeneration_AutogenStatusRace(t *testing.T) {
	ctx := context.Background()

	// Status.Autogen.Configs is empty here - the exact state a lister cache holds the
	// instant a ValidatingPolicy is created, before the policystatus controller's own
	// reconcile has had a chance to run and populate it.
	vpol := disallowPrivilegeEscalationVpol(nil)
	c, kubeClient, kyvernoClient := newVAPTestController(t, vpol)

	err := c.handleVAPGeneration(ctx, "ValidatingPolicy", engineapi.NewValidatingPolicy(vpol))
	require.NoError(t, err)

	_, vapErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, "vpol-disallow-privilege-escalation", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(vapErr),
		"a ValidatingAdmissionPolicy must NOT be generated for a policy whose spec has pod-controller autogen"+
			" configured, even if status.autogen.configs has not been populated yet by the policystatus controller: got err=%v", vapErr)

	_, bindingErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, "vpol-disallow-privilege-escalation-binding", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(bindingErr), "a ValidatingAdmissionPolicyBinding must NOT be generated either: got err=%v", bindingErr)

	updated, err := kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(ctx, "disallow-privilege-escalation", metav1.GetOptions{})
	require.NoError(t, err)
	assert.False(t, updated.Status.Generated, "status.generated must stay false when autogen is configured in spec")
}

// TestHandleVAPGeneration_AutogenStatusPopulated is the non-racy control: once
// status.autogen.configs IS populated (the policystatus controller won the race, or ran
// first), generation must still correctly be skipped. Guards against a fix that only
// works by accident of the empty-map case.
func TestHandleVAPGeneration_AutogenStatusPopulated(t *testing.T) {
	ctx := context.Background()

	vpol := disallowPrivilegeEscalationVpol(map[string]policiesv1beta1.ValidatingPolicyAutogen{
		"deployments": {},
	})
	c, kubeClient, _ := newVAPTestController(t, vpol)

	err := c.handleVAPGeneration(ctx, "ValidatingPolicy", engineapi.NewValidatingPolicy(vpol))
	require.NoError(t, err)

	_, vapErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, "vpol-disallow-privilege-escalation", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(vapErr), "no ValidatingAdmissionPolicy expected: got err=%v", vapErr)
}

// TestHandleVAPGeneration_NoAutogen is the regression guard: a policy with VAP
// generation enabled and NO pod-controller autogen must still get a generated VAP.
//
// Pod-controller autogen is opt-OUT, not opt-in: vpolautogen.Autogen (which both
// policystatus and this fix call) defaults to autogen for ALL pod-controller kinds
// whenever spec.AutogenConfiguration.PodControllers.Controllers is nil - it is gated
// only on the policy's MatchConstraints being pod-shaped (autogen/support.go
// CanAutoGen), not on whether PodControllers is set. The real, documented way to
// disable it is an explicit empty list, exactly as issue #17088's own repro does:
// "autogen.podControllers.controllers: [] set in the values". A nil PodControllers
// (this test's original setup) is therefore NOT "no autogen" - it still triggers the
// skip and would have made this a false positive for the fix's coverage.
func TestHandleVAPGeneration_NoAutogen(t *testing.T) {
	ctx := context.Background()

	vpol := disallowPrivilegeEscalationVpol(nil)
	vpol.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
	c, kubeClient, _ := newVAPTestController(t, vpol)

	err := c.handleVAPGeneration(ctx, "ValidatingPolicy", engineapi.NewValidatingPolicy(vpol))
	require.NoError(t, err)

	_, vapErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, "vpol-disallow-privilege-escalation", metav1.GetOptions{})
	assert.NoError(t, vapErr, "a ValidatingAdmissionPolicy IS expected when pod-controller autogen is not configured")
}
