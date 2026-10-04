//go:build integration

package vap_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	vpol "github.com/kyverno/kyverno/pkg/webhooks/resource/vpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testEnv  *framework.TestEnv
	engine   vpolengine.Engine
	provider vpolengine.Provider
)

func TestMain(m *testing.M) {
	var err error
	testEnv, err = framework.NewTestEnv("../../../config/crds/policies.kyverno.io")
	if err != nil {
		panic(err)
	}
	engine, provider, err = framework.NewVpolEngine(testEnv.Mgr)
	if err != nil {
		testEnv.Stop()
		panic(err)
	}
	if err := testEnv.Start(); err != nil {
		testEnv.Stop()
		panic(err)
	}
	code := m.Run()
	testEnv.Stop()
	os.Exit(code)
}

func waitForPolicyReady(t *testing.T, count int) {
	t.Helper()
	require.Eventually(t, func() bool {
		policies, err := provider.Fetch(context.Background())
		return err == nil && len(policies) >= count
	}, 5*time.Second, 100*time.Millisecond, "policy not reconciled in time")
}

// makePod builds a Pod; when env != "" it carries label env=<env>.
func makePod(name, env string) *corev1.Pod {
	labels := map[string]string{}
	if env != "" {
		labels["env"] = env
	}
	return &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: labels},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx"}}},
	}
}

// TestVAP_GeneratedFromVpol_EnforcesEquivalentlyToKyverno is the migration-parity
// flagship. A single ValidatingPolicy is enforced two ways: by Kyverno's own CEL
// engine (the handler) and by the native ValidatingAdmissionPolicy that Kyverno
// generates from that same policy (enforced by the API server). The two decisions
// must agree. If Kyverno's VAP conversion ever diverges from its engine, migrating
// a policy to native enforcement would silently change what is allowed or denied;
// this test fails in exactly that case.
func TestVAP_GeneratedFromVpol_EnforcesEquivalentlyToKyverno(t *testing.T) {
	ctx := context.Background()

	// A realistic hardening policy: deny pods labeled env=production.
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-production"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "!has(object.metadata.labels) || !('env' in object.metadata.labels) || object.metadata.labels.env != 'production'",
				Message:    "production pods are not allowed",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
	require.NoError(t, testEnv.Client.Create(ctx, policy))
	t.Cleanup(func() { _ = testEnv.Client.Delete(context.Background(), policy) })
	waitForPolicyReady(t, 1)

	// Path B setup: generate the native VAP from the SAME policy via Kyverno's real
	// conversion path, then apply it so the API server enforces it natively.
	vapObj, binding, err := framework.GenerateVAP(testEnv.DClient.Discovery(), engineapi.NewValidatingPolicy(policy), nil)
	require.NoError(t, err, "generate VAP from vpol")
	cleanup, err := framework.ApplyVAP(ctx, testEnv.KubeClient, vapObj, binding)
	require.NoError(t, err, "apply generated VAP + binding")
	t.Cleanup(cleanup)

	// Path A: Kyverno's own engine decision, via the handler (denied == !Allowed).
	kyvernoDenies := func(pod *corev1.Pod) bool {
		raw, err := json.Marshal(pod)
		require.NoError(t, err)
		h := vpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
		resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest(pod.Name, pod.Namespace, raw), "", time.Now())
		return !resp.Allowed
	}

	violating := makePod("prod-pod", "production")
	compliant := makePod("staging-pod", "staging")

	// Kyverno-engine decisions (the reference).
	assert.True(t, kyvernoDenies(violating), "Kyverno engine should deny the production pod")
	assert.False(t, kyvernoDenies(compliant), "Kyverno engine should admit the staging pod")

	// Native-VAP decisions must match. The VAP activates asynchronously, so poll
	// with a dry-run create (which runs admission, including VAP enforcement, but
	// persists nothing) until the API server denies the violating pod.
	dryRun := metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}}
	require.Eventually(t, func() bool {
		_, createErr := testEnv.KubeClient.CoreV1().Pods(violating.Namespace).Create(ctx, violating, dryRun)
		return createErr != nil && strings.Contains(createErr.Error(), "production pods are not allowed")
	}, 30*time.Second, 500*time.Millisecond, "generated VAP should natively deny the production pod (equivalent to Kyverno)")

	// A compliant pod must be admitted by the (now active) VAP, matching Kyverno.
	_, err = testEnv.KubeClient.CoreV1().Pods(compliant.Namespace).Create(ctx, compliant, dryRun)
	require.NoError(t, err, "generated VAP should admit the staging pod (equivalent to Kyverno)")
}
