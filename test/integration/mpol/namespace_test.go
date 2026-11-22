//go:build integration

package mpol_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	mpol "github.com/kyverno/kyverno/pkg/webhooks/resource/mpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

func unlabelledPod(name, namespace string) []byte {
	return []byte(fmt.Sprintf(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": %q, "namespace": %q, "labels": {}},
		"spec": {"containers": [{"name": "app", "image": "nginx:1.27"}]}
	}`, name, namespace))
}

func TestMutate_NamespaceSelector_MutatesOnlyInOptedInNamespaces(t *testing.T) {
	// Sidecar-style opt-in: pods are marked for mesh injection only in namespaces that opted in
	// with a label, the way istio-injection=enabled works.
	meshNS := "checkout-" + rand.String(5)
	plainNS := "batch-" + rand.String(5)
	framework.CreateNamespaceWithLabels(t, testEnv.KubeClient, meshNS, map[string]string{"mesh.example.com/inject": "enabled"})
	framework.CreateNamespaceWithLabels(t, testEnv.KubeClient, plainNS, nil)

	match := framework.PodMatchRules()
	match.NamespaceSelector = &metav1.LabelSelector{MatchLabels: map[string]string{"mesh.example.com/inject": "enabled"}}
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mark-pods-for-mesh-injection"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: match,
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: `[JSONPatch{op: "add", path: "/metadata/labels/mesh-injected", value: "true"}]`,
				},
			}},
		},
	}
	createPolicyWithCleanup(t, policy)
	framework.WaitForMpols(t, provider, policy.Name)

	h := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, nil, "", &framework.MockEventGen{})
	ctx := framework.ContextWithPolicies(context.Background(), "mark-pods-for-mesh-injection")

	resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("web", meshNS, unlabelledPod("web", meshNS)), "", time.Now())
	require.True(t, resp.Allowed)
	require.NotNil(t, resp.Patch, "a pod in an opted-in namespace must be mutated")
	p := findPatch(decodePatches(t, resp.Patch), "/metadata/labels/mesh-injected")
	require.NotNil(t, p, "patch for /metadata/labels/mesh-injected should exist")
	assert.Equal(t, "true", p.Value)

	resp = h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("job", plainNS, unlabelledPod("job", plainNS)), "", time.Now())
	assert.True(t, resp.Allowed)
	assert.Nil(t, resp.Patch, "a pod in a namespace that did not opt in must not be mutated")
}

func TestMutate_NamespaceObject_CopiesNamespaceTeamLabelToPods(t *testing.T) {
	// Cost allocation: pods inherit the owning team label of their namespace, read through
	// namespaceObject.
	ns := "search-" + rand.String(5)
	framework.CreateNamespaceWithLabels(t, testEnv.KubeClient, ns, map[string]string{"team": "search-relevance"})

	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "inherit-namespace-team-label"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
					Expression: `[JSONPatch{op: "add", path: "/metadata/labels/team", value: namespaceObject.metadata.labels["team"]}]`,
				},
			}},
		},
	}
	createPolicyWithCleanup(t, policy)
	framework.WaitForMpols(t, provider, policy.Name)

	h := mpol.New(testEnv.ContextProvider, engine, nil, reportutils.ReportingCfg, nil, "", &framework.MockEventGen{})
	ctx := framework.ContextWithPolicies(context.Background(), "inherit-namespace-team-label")

	resp := h.MutateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("indexer", ns, unlabelledPod("indexer", ns)), "", time.Now())
	require.True(t, resp.Allowed, "mutation should not fail: %v", resp.Result)
	require.NotNil(t, resp.Patch, "the pod must get its namespace's team label")
	p := findPatch(decodePatches(t, resp.Patch), "/metadata/labels/team")
	require.NotNil(t, p, "patch for /metadata/labels/team should exist")
	assert.Equal(t, "search-relevance", p.Value)
}
