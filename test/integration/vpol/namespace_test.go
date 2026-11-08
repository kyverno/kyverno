//go:build integration

package vpol_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	vpol "github.com/kyverno/kyverno/pkg/webhooks/resource/vpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

// podJSON returns a minimal Pod manifest in the given namespace, with optional labels and an
// optional pod-level runAsNonRoot setting.
func podJSON(name, namespace string, labels map[string]string, runAsNonRoot bool) []byte {
	if labels == nil {
		labels = map[string]string{}
	}
	spec := map[string]any{
		"containers": []any{map[string]any{"name": "app", "image": "nginx:1.27"}},
	}
	if runAsNonRoot {
		spec["securityContext"] = map[string]any{"runAsNonRoot": true}
	}
	raw, err := json.Marshal(map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"name": name, "namespace": namespace, "labels": labels},
		"spec":       spec,
	})
	if err != nil {
		panic(err)
	}
	return raw
}

func TestValidate_NamespaceSelector_EnforcesOnlyInProductionNamespaces(t *testing.T) {
	// A platform team requires an owning team label on every pod, but only in namespaces
	// labelled environment=production. Development namespaces are left alone.
	prod := "payments-prod-" + rand.String(5)
	dev := "payments-dev-" + rand.String(5)
	framework.CreateNamespaceWithLabels(t, testEnv.KubeClient, prod, map[string]string{"environment": "production"})
	framework.CreateNamespaceWithLabels(t, testEnv.KubeClient, dev, map[string]string{"environment": "development"})

	match := framework.PodMatchRules()
	match.NamespaceSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"environment": "production"}}
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "require-team-label-in-production"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: match,
			Validations: []admissionregistrationv1.Validation{{
				Expression: "has(object.metadata.labels) && 'team' in object.metadata.labels",
				Message:    "pods in production namespaces must have a team label",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
	createPolicyWithCleanup(t, policy)
	framework.WaitForVpols(t, provider, policy.Name)

	h := vpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	resp := h.ValidateClustered(context.Background(), logr.Discard(),
		framework.PodAdmissionRequest("api", prod, podJSON("api", prod, nil, false)), "", time.Now())
	assert.False(t, resp.Allowed, "a pod without a team label must be denied in a production namespace")

	resp = h.ValidateClustered(context.Background(), logr.Discard(),
		framework.PodAdmissionRequest("api", prod, podJSON("api", prod, map[string]string{"team": "payments"}, false)), "", time.Now())
	assert.True(t, resp.Allowed, "a pod with a team label must be admitted in a production namespace: %v", resp.Result)

	resp = h.ValidateClustered(context.Background(), logr.Discard(),
		framework.PodAdmissionRequest("api", dev, podJSON("api", dev, nil, false)), "", time.Now())
	assert.True(t, resp.Allowed, "the policy must not apply in a development namespace")
}

func TestValidate_NamespaceObject_PodMustCarryItsNamespaceCostCenter(t *testing.T) {
	// Chargeback: every pod must carry the cost-center label of the namespace it runs in. The
	// policy reads the namespace label through namespaceObject.
	ns := "analytics-" + rand.String(5)
	framework.CreateNamespaceWithLabels(t, testEnv.KubeClient, ns, map[string]string{"cost-center": "cc-4521"})

	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-cost-center-matches-namespace"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "has(object.metadata.labels) && 'cost-center' in object.metadata.labels && " +
					"object.metadata.labels['cost-center'] == namespaceObject.metadata.labels['cost-center']",
				Message: "pods must carry their namespace's cost-center label",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
	createPolicyWithCleanup(t, policy)
	framework.WaitForVpols(t, provider, policy.Name)

	h := vpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	resp := h.ValidateClustered(context.Background(), logr.Discard(),
		framework.PodAdmissionRequest("etl", ns, podJSON("etl", ns, map[string]string{"cost-center": "cc-4521"}, false)), "", time.Now())
	assert.True(t, resp.Allowed, "a pod with its namespace's cost-center must be admitted: %v", resp.Result)

	resp = h.ValidateClustered(context.Background(), logr.Discard(),
		framework.PodAdmissionRequest("etl", ns, podJSON("etl", ns, map[string]string{"cost-center": "cc-9999"}, false)), "", time.Now())
	assert.False(t, resp.Allowed, "a pod with a different cost-center must be denied")
}

func TestValidate_PolicyException_ExemptsNamespacesByLabel(t *testing.T) {
	// The cluster requires runAsNonRoot, but a legacy namespace labelled for it is exempt through a
	// PolicyException whose matchCondition reads the namespace's labels.
	legacy := "legacy-billing-" + rand.String(5)
	regular := "orders-" + rand.String(5)
	revoked := "legacy-reports-" + rand.String(5)
	framework.CreateNamespaceWithLabels(t, testEnv.KubeClient, legacy, map[string]string{"security.example.com/run-as-root": "allowed"})
	framework.CreateNamespaceWithLabels(t, testEnv.KubeClient, regular, nil)
	framework.CreateNamespaceWithLabels(t, testEnv.KubeClient, revoked, map[string]string{"security.example.com/run-as-root": "denied"})

	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "require-run-as-non-root-user"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "has(object.spec.securityContext) && has(object.spec.securityContext.runAsNonRoot) && object.spec.securityContext.runAsNonRoot == true",
				Message:    "pods must set securityContext.runAsNonRoot to true",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "legacy-namespaces-may-run-as-root", Namespace: "default"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1alpha1.PolicyRef{{Name: "require-run-as-non-root-user", Kind: "ValidatingPolicy"}},
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name: "namespace-allows-root",
				Expression: "has(namespaceObject.metadata.labels) && 'security.example.com/run-as-root' in namespaceObject.metadata.labels && " +
					"namespaceObject.metadata.labels['security.example.com/run-as-root'] == 'allowed'",
			}},
		},
	}

	ctx := context.Background()
	require.NoError(t, testEnv.Client.Create(ctx, policy))
	require.NoError(t, testEnv.Client.Create(ctx, exception))
	t.Cleanup(func() {
		_ = testEnv.Client.Delete(context.Background(), exception)
		time.Sleep(500 * time.Millisecond)
		_ = testEnv.Client.Delete(context.Background(), policy)
		waitForPolicyGone(t)
	})
	framework.WaitForVpols(t, provider, policy.Name)

	h := vpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})

	require.Eventually(t, func() bool {
		resp := h.ValidateClustered(ctx, logr.Discard(),
			framework.PodAdmissionRequest("batch", legacy, podJSON("batch", legacy, nil, false)), "", time.Now())
		return resp.Allowed
	}, 5*time.Second, 200*time.Millisecond, "the exception must exempt a root pod in the labelled legacy namespace")

	resp := h.ValidateClustered(ctx, logr.Discard(),
		framework.PodAdmissionRequest("batch", regular, podJSON("batch", regular, nil, false)), "", time.Now())
	assert.False(t, resp.Allowed, "the exception must not exempt a namespace without the label")
	require.NotNil(t, resp.Result)
	assert.Contains(t, resp.Result.Message, "pods must set securityContext.runAsNonRoot to true",
		"the pod must be denied by the policy's validation, not by an error in the exception")

	resp = h.ValidateClustered(ctx, logr.Discard(),
		framework.PodAdmissionRequest("batch", revoked, podJSON("batch", revoked, nil, false)), "", time.Now())
	assert.False(t, resp.Allowed, "the exception must not exempt a namespace whose label has another value")
	require.NotNil(t, resp.Result)
	assert.Contains(t, resp.Result.Message, "pods must set securityContext.runAsNonRoot to true")
}
