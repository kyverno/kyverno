//go:build integration

package gpol_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	gpol "github.com/kyverno/kyverno/pkg/webhooks/resource/gpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

// TestGenerateFullFlow_NamespaceSelector_GeneratesDefaultDenyOnlyInIsolatedTenants covers tenant
// isolation done with a GeneratingPolicy: the first workload in a namespace labelled for strict
// isolation gets a default-deny NetworkPolicy, named after the namespace the policy reads through
// namespaceObject (the shape of the data-sync conformance policies). Namespaces without the label
// are left alone.
func TestGenerateFullFlow_NamespaceSelector_GeneratesDefaultDenyOnlyInIsolatedTenants(t *testing.T) {
	isolated := "tenant-acme-" + rand.String(5)
	shared := "shared-tools-" + rand.String(5)
	framework.CreateNamespaceWithLabels(t, testEnv.KubeClient, isolated, map[string]string{"tenant.example.com/isolation": "strict"})
	framework.CreateNamespaceWithLabels(t, testEnv.KubeClient, shared, nil)

	match := framework.PodMatchRules()
	match.NamespaceSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"tenant.example.com/isolation": "strict"}}
	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "default-deny-for-isolated-tenants"},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			MatchConstraints: match,
			Variables: []admissionregistrationv1.Variable{
				{Name: "nsName", Expression: "namespaceObject.metadata.name"},
				{
					Name: "networkpolicy",
					Expression: `[{
						"kind": dyn("NetworkPolicy"),
						"apiVersion": dyn("networking.k8s.io/v1"),
						"metadata": dyn({"name": "default-deny", "namespace": string(variables.nsName)}),
						"spec": dyn({"podSelector": dyn({}), "policyTypes": dyn(["Ingress", "Egress"])})
					}]`,
				},
			},
			Generation: []policiesv1beta1.Generation{
				{Expression: "generator.Apply(variables.nsName, variables.networkpolicy)"},
			},
		},
	}
	createGpolWithCleanup(t, policy)
	waitForGpolInLister(t, policy.Name)

	processor := framework.NewURProcessor(gpolEngine, gpolProvider, testEnv.ContextProvider)
	mock := framework.NewProcessingURGenerator(processor)
	h := gpol.New(mock, gpolLister, ngpolLister, "")
	ctx := framework.ContextWithPolicies(context.Background(), policy.Name)

	trigger := func(ns string) {
		t.Helper()
		raw := []byte(fmt.Sprintf(`{
			"apiVersion": "v1", "kind": "Pod",
			"metadata": {"name": "web", "namespace": %q, "uid": "uid-web-%s"},
			"spec": {"containers": [{"name": "app", "image": "nginx:1.27"}]}
		}`, ns, ns))
		before := len(mock.GetSpecs())
		resp := h.Generate(ctx, logr.Discard(), framework.PodAdmissionRequestWithOp("web", ns, admissionv1.Create, raw), "", time.Now())
		require.True(t, resp.Allowed)
		require.Eventually(t, func() bool { return len(mock.GetSpecs()) > before }, 5*time.Second, 100*time.Millisecond, "UR not processed in time")
	}

	trigger(isolated)
	require.Empty(t, mock.ProcessingErrors(), "processing should succeed without errors")
	np, err := testEnv.KubeClient.NetworkingV1().NetworkPolicies(isolated).Get(context.Background(), "default-deny", metav1.GetOptions{})
	require.NoError(t, err, "a default-deny NetworkPolicy must be generated in the isolated tenant namespace")
	t.Cleanup(func() {
		_ = testEnv.KubeClient.NetworkingV1().NetworkPolicies(isolated).Delete(context.Background(), np.Name, metav1.DeleteOptions{})
	})
	assert.Len(t, np.Spec.PolicyTypes, 2)

	trigger(shared)
	require.Empty(t, mock.ProcessingErrors(), "processing should succeed without errors")
	_, err = testEnv.KubeClient.NetworkingV1().NetworkPolicies(shared).Get(context.Background(), "default-deny", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "no NetworkPolicy may be generated in a namespace without the isolation label, got %v", err)
}
