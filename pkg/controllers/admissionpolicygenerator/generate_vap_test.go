package admissionpolicygenerator

import (
	"context"
	"errors"
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
	kubefake "k8s.io/client-go/kubernetes/fake"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	k8stesting "k8s.io/client-go/testing"
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

// TestHandleVAPGeneration_PodControllerAutogen checks that ValidatingAdmissionPolicies are
// generated for a policy with pod-controller autogen, including right after the policy is
// created, before the policystatus controller has written status.autogen.
func TestHandleVAPGeneration_PodControllerAutogen(t *testing.T) {
	tests := []struct {
		name           string
		statusConfigs  map[string]policiesv1beta1.ValidatingPolicyAutogen
		disableAutogen bool
	}{{
		name: "autogen in the spec, status.autogen not written yet",
	}, {
		name:          "autogen in the spec and in status.autogen",
		statusConfigs: map[string]policiesv1beta1.ValidatingPolicyAutogen{"defaults": {}},
	}, {
		// autogen is on by default for pod-shaped policies, so an empty controllers list turns it off
		name:           "autogen turned off",
		disableAutogen: true,
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
			assert.NoError(t, vapErr, "a ValidatingAdmissionPolicy is expected")
			assert.NoError(t, bindingErr, "a ValidatingAdmissionPolicyBinding is expected")

			if !tt.disableAutogen {
				// The test policy spec lists ["deployments","cronjobs"] as controllers.
				// "deployments" resolves to the "defaults" replacements bucket and
				// "cronjobs" resolves to the "cronjobs" bucket, so both autogen VAPs
				// must be created regardless of what status.autogen already contains.
				for _, configKey := range []string{"defaults", "cronjobs"} {
					autogenName := autogenVAPName("disallow-privilege-escalation", configKey)
					_, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, autogenName, metav1.GetOptions{})
					assert.NoError(t, err, "an autogen ValidatingAdmissionPolicy is expected for %s", configKey)
				}
			}
			assert.True(t, updated.Status.Generated)
		})
	}
}

// TestHandleVAPGeneration_StaleAutogenCleanup verifies that handleVAPGeneration deletes
// stale autogen VAPs (and their bindings) when pod-controller autogen is disabled.
func TestHandleVAPGeneration_StaleAutogenCleanup(t *testing.T) {
	ctx := context.Background()

	vpol := disallowPrivilegeEscalationVpol(nil)
	vpol.UID = "test-vpol-uid"
	// Disable autogen so the handler must prune any previously generated autogen VAPs.
	vpol.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}

	// Pre-seed stale autogen VAPs that the handler is expected to delete.
	staleDefaultsName := autogenVAPName("disallow-privilege-escalation", "defaults")
	staleCronjobsName := autogenVAPName("disallow-privilege-escalation", "cronjobs")
	staleLabels := map[string]string{autogenSourceLabel: labelValue("disallow-privilege-escalation")}

	staleVAP1 := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: staleDefaultsName, Labels: staleLabels, OwnerReferences: []metav1.OwnerReference{{UID: "test-vpol-uid"}}},
	}
	staleVAP2 := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: staleCronjobsName, Labels: staleLabels, OwnerReferences: []metav1.OwnerReference{{UID: "test-vpol-uid"}}},
	}
	staleBinding1 := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: constructBindingName(staleDefaultsName), Labels: staleLabels, OwnerReferences: []metav1.OwnerReference{{UID: "test-vpol-uid"}}},
		Spec:       admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{PolicyName: staleDefaultsName},
	}
	staleBinding2 := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: constructBindingName(staleCronjobsName), Labels: staleLabels, OwnerReferences: []metav1.OwnerReference{{UID: "test-vpol-uid"}}},
		Spec:       admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{PolicyName: staleCronjobsName},
	}

	kubeClient := kubefake.NewSimpleClientset(staleVAP1, staleVAP2, staleBinding1, staleBinding2)
	kyvernoClient := versionedfake.NewSimpleClientset(vpol)

	// Populate the lister indexers so pruneStaleAutogenVAPs can discover the stale resources.
	vapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	_ = vapIndexer.Add(staleVAP1)
	_ = vapIndexer.Add(staleVAP2)
	vapBindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	_ = vapBindingIndexer.Add(staleBinding1)
	_ = vapBindingIndexer.Add(staleBinding2)
	celpolexIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})

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

	require.NoError(t, c.handleVAPGeneration(ctx, "ValidatingPolicy", engineapi.NewValidatingPolicy(vpol)))

	// Stale autogen VAPs and their bindings must be gone.
	_, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, staleDefaultsName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "stale defaults autogen VAP should be deleted")
	_, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, staleCronjobsName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "stale cronjobs autogen VAP should be deleted")
	_, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, constructBindingName(staleDefaultsName), metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "stale defaults autogen binding should be deleted")
	_, err = kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, constructBindingName(staleCronjobsName), metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "stale cronjobs autogen binding should be deleted")
}

// TestHandleVAPGeneration_AutogenFailureKeepsGeneratedFalse verifies that when autogen VAP
// creation fails, Status.Generated is not set to true.
func TestHandleVAPGeneration_AutogenFailureKeepsGeneratedFalse(t *testing.T) {
	ctx := context.Background()

	vpol := disallowPrivilegeEscalationVpol(nil)
	kubeClient := kubefake.NewSimpleClientset()
	kyvernoClient := versionedfake.NewSimpleClientset(vpol)

	vapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	vapBindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	celpolexIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})

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

	// Inject a reactor that fails creation of autogen VAPs only (not the base vpol- VAP).
	createErr := errors.New("injected autogen VAP create failure")
	kubeClient.PrependReactor("create", "validatingadmissionpolicies", func(action k8stesting.Action) (bool, runtime.Object, error) {
		ca := action.(k8stesting.CreateAction)
		if obj, ok := ca.GetObject().(*admissionregistrationv1.ValidatingAdmissionPolicy); ok {
			if _, hasLabel := obj.Labels[autogenSourceLabel]; hasLabel {
				return true, nil, createErr
			}
		}
		return false, nil, nil // let the base VAP creation proceed normally
	})

	err := c.handleVAPGeneration(ctx, "ValidatingPolicy", engineapi.NewValidatingPolicy(vpol))
	assert.Error(t, err, "expected an error when autogen VAP creation fails")

	// Status.Generated must remain false so the webhook is not prematurely dropped.
	updated, getErr := kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(ctx, "disallow-privilege-escalation", metav1.GetOptions{})
	require.NoError(t, getErr)
	assert.False(t, updated.Status.Generated, "Status.Generated must not be true when autogen VAP creation fails")
}
