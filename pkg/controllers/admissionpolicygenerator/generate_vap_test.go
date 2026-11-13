package admissionpolicygenerator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-logr/logr"
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
	"k8s.io/apimachinery/pkg/types"
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

func policyOwnerRef(kind, name, uid string) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: policiesv1beta1.GroupVersion.String(),
		Kind:       kind,
		Name:       name,
		UID:        types.UID(uid),
	}
}

func preservedGeneratedStatus() policiesv1beta1.ConditionStatus {
	return policiesv1beta1.ConditionStatus{
		Message: "left alone",
		Conditions: []metav1.Condition{{
			Type:               string(policiesv1beta1.PolicyConditionTypeWebhookConfigured),
			Status:             metav1.ConditionTrue,
			Reason:             "Succeeded",
			Message:            "Admission is enforced by the generated ValidatingAdmissionPolicy.",
			LastTransitionTime: metav1.NewTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)),
		}},
	}
}

func envoyValidatingPolicy() *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "envoy-pol", UID: "policy-uid"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: "Envoy"},
		},
		Status: policiesv1beta1.ValidatingPolicyStatus{
			Generated:       true,
			ConditionStatus: preservedGeneratedStatus(),
		},
	}
}

func TestReconcile_ForeignModeDeletesGeneratedVAPWithoutStatus(t *testing.T) {
	vpol := envoyValidatingPolicy()
	c, kubeClient, kyvernoClient := newVAPTestController(t, vpol)
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, indexer.Add(vpol))
	c.vpolLister = policiesv1beta1listers.NewValidatingPolicyLister(indexer)

	vapName := "vpol-" + vpol.Name
	bindingName := constructBindingName(vapName)
	owner := policyOwnerRef("ValidatingPolicy", vpol.Name, string(vpol.UID))
	_, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(context.Background(), &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            vapName,
			UID:             "vap-uid",
			ResourceVersion: "11",
			OwnerReferences: []metav1.OwnerReference{owner},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(context.Background(), &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            bindingName,
			UID:             "binding-uid",
			ResourceVersion: "22",
			OwnerReferences: []metav1.OwnerReference{owner},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	storedVAP, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(context.Background(), vapName, metav1.GetOptions{})
	require.NoError(t, err)
	storedBinding, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(context.Background(), bindingName, metav1.GetOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, string(storedVAP.UID))
	require.NotEmpty(t, storedVAP.ResourceVersion)
	require.NotEqual(t, storedVAP.UID, storedBinding.UID)
	require.NotEqual(t, storedVAP.ResourceVersion, storedBinding.ResourceVersion)
	var vapPreconditions, bindingPreconditions *metav1.Preconditions
	kubeClient.PrependReactor("delete", "validatingadmissionpolicies", func(action clienttesting.Action) (bool, runtime.Object, error) {
		del, ok := action.(clienttesting.DeleteAction)
		if ok {
			vapPreconditions = del.GetDeleteOptions().Preconditions
		}
		return false, nil, nil
	})
	kubeClient.PrependReactor("delete", "validatingadmissionpolicybindings", func(action clienttesting.Action) (bool, runtime.Object, error) {
		del, ok := action.(clienttesting.DeleteAction)
		if ok {
			bindingPreconditions = del.GetDeleteOptions().Preconditions
		}
		return false, nil, nil
	})

	require.NoError(t, c.reconcile(context.Background(), logr.Discard(), "ValidatingPolicy/"+vpol.Name, "ValidatingPolicy", vpol.Name))
	require.NotNil(t, vapPreconditions)
	require.NotNil(t, vapPreconditions.UID)
	require.NotNil(t, vapPreconditions.ResourceVersion)
	assert.Equal(t, storedVAP.UID, *vapPreconditions.UID)
	assert.Equal(t, storedVAP.ResourceVersion, *vapPreconditions.ResourceVersion)
	require.NotNil(t, bindingPreconditions)
	require.NotNil(t, bindingPreconditions.UID)
	require.NotNil(t, bindingPreconditions.ResourceVersion)
	assert.Equal(t, storedBinding.UID, *bindingPreconditions.UID)
	assert.Equal(t, storedBinding.ResourceVersion, *bindingPreconditions.ResourceVersion)

	_, vapErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(context.Background(), vapName, metav1.GetOptions{})
	_, bindingErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(context.Background(), bindingName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(vapErr))
	assert.True(t, apierrors.IsNotFound(bindingErr))

	stored, err := kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(context.Background(), vpol.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, vpol.Status, stored.Status)
}

func TestReconcile_ForeignModeLeavesUnownedVAP(t *testing.T) {
	vpol := envoyValidatingPolicy()
	c, kubeClient, _ := newVAPTestController(t, vpol)
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, indexer.Add(vpol))
	c.vpolLister = policiesv1beta1listers.NewValidatingPolicyLister(indexer)

	vapName := "vpol-" + vpol.Name
	bindingName := constructBindingName(vapName)
	_, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(context.Background(), &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: vapName},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(context.Background(), &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: bindingName},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	require.NoError(t, c.reconcile(context.Background(), logr.Discard(), "ValidatingPolicy/"+vpol.Name, "ValidatingPolicy", vpol.Name))

	_, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(context.Background(), vapName, metav1.GetOptions{})
	require.NoError(t, err)
	_, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(context.Background(), bindingName, metav1.GetOptions{})
	require.NoError(t, err)
}

func TestReconcile_ForeignModeVAPDeleteErrors(t *testing.T) {
	vpol := envoyValidatingPolicy()
	c, kubeClient, _ := newVAPTestController(t, vpol)
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, indexer.Add(vpol))
	c.vpolLister = policiesv1beta1listers.NewValidatingPolicyLister(indexer)

	vapName := "vpol-" + vpol.Name
	bindingName := constructBindingName(vapName)
	owner := policyOwnerRef("ValidatingPolicy", vpol.Name, string(vpol.UID))
	ctx := context.Background()
	_, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: vapName, OwnerReferences: []metav1.OwnerReference{owner}},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: bindingName, OwnerReferences: []metav1.OwnerReference{owner}},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	failVAP := true
	failBinding := false
	kubeClient.PrependReactor("delete", "validatingadmissionpolicies", func(clienttesting.Action) (bool, runtime.Object, error) {
		if failVAP {
			return true, nil, errors.New("vap delete failed")
		}
		return false, nil, nil
	})
	kubeClient.PrependReactor("delete", "validatingadmissionpolicybindings", func(clienttesting.Action) (bool, runtime.Object, error) {
		if failBinding {
			return true, nil, errors.New("binding delete failed")
		}
		return false, nil, nil
	})

	err = c.reconcile(ctx, logr.Discard(), "ValidatingPolicy/"+vpol.Name, "ValidatingPolicy", vpol.Name)
	require.ErrorContains(t, err, "vap delete failed")
	_, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, vapName, metav1.GetOptions{})
	require.NoError(t, err)

	failVAP = false
	failBinding = true
	err = c.reconcile(ctx, logr.Discard(), "ValidatingPolicy/"+vpol.Name, "ValidatingPolicy", vpol.Name)
	require.ErrorContains(t, err, "binding delete failed")
	_, vapErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, vapName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(vapErr))
	_, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, bindingName, metav1.GetOptions{})
	require.NoError(t, err)

	failBinding = false
	require.NoError(t, c.reconcile(ctx, logr.Discard(), "ValidatingPolicy/"+vpol.Name, "ValidatingPolicy", vpol.Name))
	_, bindingErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, bindingName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(bindingErr))
}

func TestReconcile_ForeignModeSucceedsWhenVAPAlreadyMissing(t *testing.T) {
	vpol := envoyValidatingPolicy()
	c, _, _ := newVAPTestController(t, vpol)
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, indexer.Add(vpol))
	c.vpolLister = policiesv1beta1listers.NewValidatingPolicyLister(indexer)

	require.NoError(t, c.reconcile(context.Background(), logr.Discard(), "ValidatingPolicy/"+vpol.Name, "ValidatingPolicy", vpol.Name))
}
