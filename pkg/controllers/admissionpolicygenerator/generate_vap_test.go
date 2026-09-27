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

// permissiveAuthChecker allows managing native admission policies and bindings.
type permissiveAuthChecker struct{}

func (permissiveAuthChecker) Check(ctx context.Context, group, version, resource, subresource, namespace, name, verb string) (*checker.AuthResult, error) {
	return &checker.AuthResult{Allowed: true}, nil
}

// newVAPTestController builds a controller backed by fake clientsets and empty listers.
func newVAPTestController(t *testing.T, vpol *policiesv1beta1.ValidatingPolicy) (*controller, *kubefake.Clientset, *versionedfake.Clientset) {
	t.Helper()
	kubeClient := kubefake.NewSimpleClientset()
	kyvernoClient := versionedfake.NewSimpleClientset(vpol)
	celpolexIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	// no VAP or binding exists yet
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

// disallowPrivilegeEscalationVpol is a pods-only ValidatingPolicy with pod-controller autogen
// and VAP generation enabled.
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

// TestHandleVAPGeneration_AutogenStatusRace covers a new policy whose status.autogen is not
// written yet: generation must still be skipped because autogen is set in the spec.
func TestHandleVAPGeneration_AutogenStatusRace(t *testing.T) {
	ctx := context.Background()

	// status.autogen is empty, as it is right after the policy is created
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

// TestHandleVAPGeneration_AutogenStatusPopulated checks generation is skipped once status.autogen is set.
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

// TestHandleVAPGeneration_NoAutogen checks a VAP is still generated when autogen is turned off.
// Autogen is on by default for pod-shaped policies, so it is disabled with an empty controllers list.
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
