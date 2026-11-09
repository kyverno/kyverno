package admissionpolicygenerator

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/toggle"
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

// disabledVAPGenerationToggle simulates the global feature flag being disabled,
// while retaining the rest of the current feature-toggle configuration.
type disabledVAPGenerationToggle struct{ toggle.Toggles }

func (disabledVAPGenerationToggle) GenerateValidatingAdmissionPolicy() bool { return false }

// TestReconcileVAPExplicitOptIn tests the per-policy override at the controller
// reconcile entrypoint, where the global generation toggle is checked.
func TestReconcileVAPExplicitOptIn(t *testing.T) {
	for _, tt := range []struct {
		name      string
		perPolicy bool
	}{
		{name: "explicitly enabled with global flag off", perPolicy: true},
		{name: "disabled with global flag off", perPolicy: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			policy := disallowPrivilegeEscalationVpol(nil)
			policy.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
			policy.Spec.AutogenConfiguration.ValidatingAdmissionPolicy.Enabled = ptr.To(tt.perPolicy)

			c, kubeClient, _ := newVAPTestController(t, policy)
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			require.NoError(t, indexer.Add(policy))
			c.vpolLister = policiesv1beta1listers.NewValidatingPolicyLister(indexer)

			baseCtx := context.Background()
			ctx := toggle.NewContext(baseCtx, disabledVAPGenerationToggle{Toggles: toggle.FromContext(baseCtx)})
			require.NoError(t, c.reconcile(ctx, logr.Discard(), "ValidatingPolicy/"+policy.Name, "", policy.Name))
			_, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, "vpol-"+policy.Name, metav1.GetOptions{})
			if tt.perPolicy {
				require.NoError(t, err, "explicit per-policy opt-in must override the global default")
			} else {
				assert.True(t, apierrors.IsNotFound(err), "disabled policy must not generate a VAP: %v", err)
			}
		})
	}
}

// TestReconcileVAPDisabledCleanup checks the persisted-resource cleanup path
// when the global default and the policy-specific VAP opt-in are both disabled.
// It also covers resources left after a restart, incomplete cleanup, stale
// generated status, and no-op reconciliation of an idle policy.
func TestReconcileVAPDisabledCleanup(t *testing.T) {
	tests := []struct {
		name            string
		seedVAP         bool
		seedBinding     bool
		generatedStatus bool
		cacheOnly       bool
	}{
		{name: "nothing to clean up"},
		{name: "stale generated status", generatedStatus: true},
		{name: "orphaned VAP", seedVAP: true},
		{name: "orphaned binding", seedBinding: true},
		{name: "both resources after restart", seedVAP: true, seedBinding: true},
		{name: "stale informer cache after deletion", seedVAP: true, seedBinding: true, cacheOnly: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseCtx := context.Background()
			ctx := toggle.NewContext(baseCtx, disabledVAPGenerationToggle{Toggles: toggle.FromContext(baseCtx)})
			policy := disallowPrivilegeEscalationVpol(nil)
			policy.Spec.AutogenConfiguration.ValidatingAdmissionPolicy.Enabled = ptr.To(false)
			policy.Status.Generated = tt.generatedStatus

			c, kubeClient, kyvernoClient := newVAPTestController(t, policy)
			policyIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			require.NoError(t, policyIndexer.Add(policy))
			c.vpolLister = policiesv1beta1listers.NewValidatingPolicyLister(policyIndexer)

			vapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			bindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			vapName := "vpol-" + policy.Name
			bindingName := vapName + "-binding"
			if tt.seedVAP {
				vap := &admissionregistrationv1.ValidatingAdmissionPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: vapName},
				}
				if !tt.cacheOnly {
					_, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, vap, metav1.CreateOptions{})
					require.NoError(t, err)
				}
				require.NoError(t, vapIndexer.Add(vap))
			}
			if tt.seedBinding {
				binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
					ObjectMeta: metav1.ObjectMeta{Name: bindingName},
				}
				if !tt.cacheOnly {
					_, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{})
					require.NoError(t, err)
				}
				require.NoError(t, bindingIndexer.Add(binding))
			}
			c.vapLister = admissionregistrationv1listers.NewValidatingAdmissionPolicyLister(vapIndexer)
			c.vapbindingLister = admissionregistrationv1listers.NewValidatingAdmissionPolicyBindingLister(bindingIndexer)

			require.NoError(t, c.reconcile(ctx, logr.Discard(), "ValidatingPolicy/"+policy.Name, "", policy.Name))
			_, vapErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, vapName, metav1.GetOptions{})
			_, bindingErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, bindingName, metav1.GetOptions{})
			assert.True(t, apierrors.IsNotFound(vapErr), "stale VAP must be removed: %v", vapErr)
			assert.True(t, apierrors.IsNotFound(bindingErr), "stale binding must be removed: %v", bindingErr)

			// An untouched, ungenerated policy must not incur status writes.
			if !tt.seedVAP && !tt.seedBinding && !tt.generatedStatus {
				assert.Empty(t, kyvernoClient.Actions(), "idle disabled policy should be skipped")
			} else {
				assert.NotEmpty(t, kyvernoClient.Actions(), "cleanup must update the generated status")
			}
			stored, err := kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(ctx, policy.Name, metav1.GetOptions{})
			require.NoError(t, err)
			assert.False(t, stored.Status.Generated, "native generation must be marked disabled")
		})
	}
}

// TestReconcileVAPExplicitOptOutAfterGeneration verifies that disabling the
// per-policy opt-in deletes an already-created VAP and binding, even when the
// global generation switch is disabled for the entire lifecycle.
func TestReconcileVAPExplicitOptOutAfterGeneration(t *testing.T) {
	baseCtx := context.Background()
	ctx := toggle.NewContext(baseCtx, disabledVAPGenerationToggle{Toggles: toggle.FromContext(baseCtx)})
	policy := disallowPrivilegeEscalationVpol(nil)
	policy.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
	policy.Spec.AutogenConfiguration.ValidatingAdmissionPolicy.Enabled = ptr.To(true)
	c, kubeClient, kyvernoClient := newVAPTestController(t, policy)

	policyIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, policyIndexer.Add(policy))
	c.vpolLister = policiesv1beta1listers.NewValidatingPolicyLister(policyIndexer)
	key := "ValidatingPolicy/" + policy.Name

	require.NoError(t, c.reconcile(ctx, logr.Discard(), key, "", policy.Name))
	vapName := "vpol-" + policy.Name
	bindingName := vapName + "-binding"
	vap, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, vapName, metav1.GetOptions{})
	require.NoError(t, err)
	binding, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, bindingName, metav1.GetOptions{})
	require.NoError(t, err)
	stored, err := kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(ctx, policy.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.True(t, stored.Status.Generated)

	// Simulate informer updates after the native objects have been persisted.
	vapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	bindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, vapIndexer.Add(vap))
	require.NoError(t, bindingIndexer.Add(binding))
	c.vapLister = admissionregistrationv1listers.NewValidatingAdmissionPolicyLister(vapIndexer)
	c.vapbindingLister = admissionregistrationv1listers.NewValidatingAdmissionPolicyBindingLister(bindingIndexer)

	disabled := stored.DeepCopy()
	disabled.Spec.AutogenConfiguration.ValidatingAdmissionPolicy.Enabled = ptr.To(false)
	_, err = kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Update(ctx, disabled, metav1.UpdateOptions{})
	require.NoError(t, err)
	require.NoError(t, policyIndexer.Update(disabled))

	require.NoError(t, c.reconcile(ctx, logr.Discard(), key, "", policy.Name))
	_, vapErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, vapName, metav1.GetOptions{})
	_, bindingErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, bindingName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(vapErr), "previously generated VAP must be removed: %v", vapErr)
	assert.True(t, apierrors.IsNotFound(bindingErr), "previously generated binding must be removed: %v", bindingErr)
	stored, err = kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(ctx, policy.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.False(t, stored.Status.Generated)
}
