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

// TestHandleVAPGeneration_PodControllerAutogen checks that no ValidatingAdmissionPolicy is
// generated for a policy with pod-controller autogen, including right after the policy is
// created, before the policystatus controller has written status.autogen.
func TestHandleVAPGeneration_PodControllerAutogen(t *testing.T) {
	tests := []struct {
		name           string
		statusConfigs  map[string]policiesv1beta1.ValidatingPolicyAutogen
		disableAutogen bool
		wantGenerated  bool
	}{{
		name: "autogen in the spec, status.autogen not written yet",
	}, {
		name:          "autogen in the spec and in status.autogen",
		statusConfigs: map[string]policiesv1beta1.ValidatingPolicyAutogen{"deployments": {}},
	}, {
		// autogen is on by default for pod-shaped policies, so an empty controllers list turns it off
		name:           "autogen turned off",
		disableAutogen: true,
		wantGenerated:  true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			vpol := disallowPrivilegeEscalationVpol(tt.statusConfigs)
			if tt.disableAutogen {
				vpol.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
			}
			c, kubeClient, kyvernoClient := newVAPTestController(t, vpol)

			require.NoError(t, c.handleVAPGeneration(ctx, "ValidatingPolicy", engineapi.NewValidatingPolicy(vpol)))

			_, vapErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, "vpol-disallow-privilege-escalation", metav1.GetOptions{})
			_, bindingErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, "vpol-disallow-privilege-escalation-binding", metav1.GetOptions{})
			updated, err := kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(ctx, "disallow-privilege-escalation", metav1.GetOptions{})
			require.NoError(t, err)
			if tt.wantGenerated {
				assert.NoError(t, vapErr, "a ValidatingAdmissionPolicy is expected")
				assert.NoError(t, bindingErr, "a ValidatingAdmissionPolicyBinding is expected")
			} else {
				assert.True(t, apierrors.IsNotFound(vapErr), "no ValidatingAdmissionPolicy expected: got err=%v", vapErr)
				assert.True(t, apierrors.IsNotFound(bindingErr), "no ValidatingAdmissionPolicyBinding expected: got err=%v", bindingErr)
			}
			assert.Equal(t, tt.wantGenerated, updated.Status.Generated)
		})
	}
}
