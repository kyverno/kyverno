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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubefake "k8s.io/client-go/kubernetes/fake"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	clienttesting "k8s.io/client-go/testing"
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

			require.NoError(t, c.handleVAPGeneration(ctx, engineapi.NewValidatingPolicy(vpol)))

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

func newVAPTestControllerWithPolex(t *testing.T, vpol *policiesv1beta1.ValidatingPolicy, seedExistingVAP bool, polexs ...*policiesv1beta1.PolicyException) (*controller, *kubefake.Clientset, *versionedfake.Clientset) {
	t.Helper()
	var initObjs []runtime.Object
	vapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	vapBindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	if seedExistingVAP {
		existingVAP := &admissionregistrationv1.ValidatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vpol-" + vpol.GetName(),
			},
		}
		existingBinding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "vpol-" + vpol.GetName() + "-binding",
			},
		}
		initObjs = append(initObjs, existingVAP, existingBinding)
		vapIndexer.Add(existingVAP)
		vapBindingIndexer.Add(existingBinding)
	}
	kubeClient := kubefake.NewSimpleClientset(initObjs...)
	kyvernoClient := versionedfake.NewSimpleClientset(vpol)
	celpolexIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, p := range polexs {
		if p != nil {
			celpolexIndexer.Add(p)
		}
	}
	c := &controller{
		client:           kubeClient,
		kyvernoClient:    kyvernoClient,
		discoveryClient:  dclient.NewEmptyFakeClient().Discovery(),
		eventGen:         event.NewFake(),
		checker:          permissiveAuthChecker{},
		polexEnabled:     true,
		celpolexLister:   policiesv1beta1listers.NewPolicyExceptionLister(celpolexIndexer),
		vapLister:        admissionregistrationv1listers.NewValidatingAdmissionPolicyLister(vapIndexer),
		vapbindingLister: admissionregistrationv1listers.NewValidatingAdmissionPolicyBindingLister(vapBindingIndexer),
	}
	return c, kubeClient, kyvernoClient
}

func TestHandleVAPGeneration_FineGrainedExceptions(t *testing.T) {
	tests := []struct {
		name            string
		seedExistingVAP bool
		customVpol      func(base *policiesv1beta1.ValidatingPolicy)
		polex           *policiesv1beta1.PolicyException
		wantGenerated   bool
		wantReasonSub   string
		validateVAP     func(t *testing.T, vap *admissionregistrationv1.ValidatingAdmissionPolicy)
	}{
		{
			name: "unscoped fine-grained exception allows VAP generation and populates allowedImages variable",
			polex: &policiesv1beta1.PolicyException{
				ObjectMeta: metav1.ObjectMeta{
					Name: "allow-busybox",
				},
				Spec: policiesv1beta1.PolicyExceptionSpec{
					PolicyRefs: []policiesv1beta1.PolicyRef{{
						Name: "disallow-privilege-escalation",
						Kind: "ValidatingPolicy",
					}},
					Images: []string{"busybox:latest"},
				},
			},
			wantGenerated: true,
			validateVAP: func(t *testing.T, vap *admissionregistrationv1.ValidatingAdmissionPolicy) {
				require.Len(t, vap.Spec.Variables, 1)
				assert.Equal(t, "allowedImages", vap.Spec.Variables[0].Name)
				assert.Equal(t, `["busybox:latest"]`, vap.Spec.Variables[0].Expression)
			},
		},
		{
			name: "scoped fine-grained exception with matchConditions skips VAP generation to protect cluster security",
			polex: &policiesv1beta1.PolicyException{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "dev",
					Name:      "scoped-debug-image",
				},
				Spec: policiesv1beta1.PolicyExceptionSpec{
					PolicyRefs: []policiesv1beta1.PolicyRef{{
						Name: "disallow-privilege-escalation",
						Kind: "ValidatingPolicy",
					}},
					Images: []string{"debug:latest"},
					MatchConditions: []admissionregistrationv1.MatchCondition{{
						Name:       "in-dev",
						Expression: "request.namespace == 'dev'",
					}},
				},
			},
			wantGenerated: false,
			wantReasonSub: "skip generating ValidatingAdmissionPolicy: policy exception(s) dev/scoped-debug-image define fine-grained exceptions (spec.images / spec.allowedValues) scoped with spec.matchConditions, which a generated ValidatingAdmissionPolicy cannot evaluate safely",
		},
		{
			name:            "policy becoming ineligible deletes previously seeded VAP and binding and sets status.generated to false",
			seedExistingVAP: true,
			polex: &policiesv1beta1.PolicyException{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "dev",
					Name:      "scoped-debug-image",
				},
				Spec: policiesv1beta1.PolicyExceptionSpec{
					PolicyRefs: []policiesv1beta1.PolicyRef{{
						Name: "disallow-privilege-escalation",
						Kind: "ValidatingPolicy",
					}},
					Images: []string{"debug:latest"},
					MatchConditions: []admissionregistrationv1.MatchCondition{{
						Name:       "in-dev",
						Expression: "request.namespace == 'dev'",
					}},
				},
			},
			wantGenerated: false,
			wantReasonSub: "skip generating ValidatingAdmissionPolicy: policy exception(s) dev/scoped-debug-image define fine-grained exceptions (spec.images / spec.allowedValues) scoped with spec.matchConditions, which a generated ValidatingAdmissionPolicy cannot evaluate safely",
		},
		{
			name: "policy with matchConditions referencing exceptions skips VAP generation leaving evaluation to webhook",
			customVpol: func(base *policiesv1beta1.ValidatingPolicy) {
				base.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{
					Name:       "match-polex-images",
					Expression: "request.image in exceptions.allowedImages",
				}}
			},
			polex: &policiesv1beta1.PolicyException{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "polex-img",
				},
				Spec: policiesv1beta1.PolicyExceptionSpec{
					PolicyRefs: []policiesv1beta1.PolicyRef{{
						Name: "disallow-privilege-escalation",
						Kind: "ValidatingPolicy",
					}},
					Images: []string{"nginx:latest"},
				},
			},
			wantGenerated: false,
			wantReasonSub: "skip generating ValidatingAdmissionPolicy: policy spec.matchConditions reference exception variables (allowedImages/allowedValues), which cannot be evaluated in native ValidatingAdmissionPolicy matchConditions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			vpol := disallowPrivilegeEscalationVpol(nil)
			vpol.Spec.AutogenConfiguration.PodControllers.Controllers = []string{} // disable autogen so VAP is allowed
			if tt.customVpol != nil {
				tt.customVpol(vpol)
			}
			c, kubeClient, kyvernoClient := newVAPTestControllerWithPolex(t, vpol, tt.seedExistingVAP, tt.polex)

			require.NoError(t, c.handleVAPGeneration(ctx, engineapi.NewValidatingPolicy(vpol)))

			vap, vapErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, "vpol-disallow-privilege-escalation", metav1.GetOptions{})
			binding, bindingErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, "vpol-disallow-privilege-escalation-binding", metav1.GetOptions{})
			updated, err := kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(ctx, "disallow-privilege-escalation", metav1.GetOptions{})
			require.NoError(t, err)

			assert.Equal(t, tt.wantGenerated, updated.Status.Generated)
			if tt.wantGenerated {
				require.NoError(t, vapErr)
				require.NoError(t, bindingErr)
				assert.NotNil(t, binding)
				if tt.validateVAP != nil {
					tt.validateVAP(t, vap)
				}
			} else {
				assert.True(t, apierrors.IsNotFound(vapErr))
				assert.True(t, apierrors.IsNotFound(bindingErr))
				assert.Contains(t, updated.Status.GetConditionStatus().Message, tt.wantReasonSub)
			}
		})
	}
}

func TestHandleVAPGeneration_StatusConflictLeavesVAPIntact(t *testing.T) {
	ctx := context.Background()
	vpol := disallowPrivilegeEscalationVpol(nil)
	vpol.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
	vpol.Status.Generated = true

	polex := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "dev",
			Name:      "scoped-debug-image",
		},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{
				Name: "disallow-privilege-escalation",
				Kind: "ValidatingPolicy",
			}},
			Images: []string{"custom-debug-image:latest"},
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "is-debug-pod",
				Expression: "request.name.startsWith('debug-')",
			}},
		},
	}

	c, kubeClient, kyvernoClient := newVAPTestControllerWithPolex(t, vpol, true, polex)

	conflict := true
	kyvernoClient.PrependReactor("update", "validatingpolicies", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if conflict && action.GetSubresource() == "status" {
			conflict = false
			return true, nil, apierrors.NewConflict(schema.GroupResource{Group: "policies.kyverno.io", Resource: "validatingpolicies"}, vpol.GetName(), nil)
		}
		return false, nil, nil
	})

	// First run: status update fails with conflict.
	// Both VAP and binding MUST remain intact because status was not yet updated.
	err := c.handleVAPGeneration(ctx, engineapi.NewValidatingPolicy(vpol))
	require.Error(t, err)
	assert.True(t, apierrors.IsConflict(err))

	vap, vapErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, "vpol-disallow-privilege-escalation", metav1.GetOptions{})
	require.NoError(t, vapErr)
	assert.NotNil(t, vap)
	binding, bindingErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, "vpol-disallow-privilege-escalation-binding", metav1.GetOptions{})
	require.NoError(t, bindingErr)
	assert.NotNil(t, binding)

	// Second run (retry): conflict is resolved. Status is updated to false and VAP + binding are deleted.
	freshVpol, err := kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(ctx, vpol.GetName(), metav1.GetOptions{})
	require.NoError(t, err)
	require.NoError(t, c.handleVAPGeneration(ctx, engineapi.NewValidatingPolicy(freshVpol)))

	updated, err := kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(ctx, vpol.GetName(), metav1.GetOptions{})
	require.NoError(t, err)
	assert.False(t, updated.Status.Generated)

	_, vapErr = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, "vpol-disallow-privilege-escalation", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(vapErr))
	_, bindingErr = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, "vpol-disallow-privilege-escalation-binding", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(bindingErr))
}
