//go:build integration

package reports_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	"github.com/kyverno/kyverno/pkg/controllers/report/aggregate"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	vpol "github.com/kyverno/kyverno/pkg/webhooks/resource/vpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
)

var (
	testEnv  *framework.TestEnv
	engine   vpolengine.Engine
	provider vpolengine.Provider
)

func TestMain(m *testing.M) {
	// Enable reporting BEFORE NewTestEnv: the reporting config is a process-wide
	// singleton that NewTestEnv otherwise locks to disabled, and the admission
	// handler needs a non-nil reports breaker.
	framework.EnableReporting()

	var err error
	testEnv, err = framework.NewTestEnv(
		"../../../config/crds/policies.kyverno.io",
		"../../../config/crds/reports",      // EphemeralReport (admission emits these)
		"../../../config/crds/policyreport", // PolicyReport (aggregation persists these)
	)
	if err != nil {
		panic(err)
	}

	engine, provider, err = framework.NewVpolEngineWithExceptions(testEnv.Mgr)
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
	}, 5*time.Second, 100*time.Millisecond, "policies not reconciled in time")
}

func createPolicyWithCleanup(t *testing.T, policy *policiesv1beta1.ValidatingPolicy) {
	t.Helper()
	require.NoError(t, testEnv.Client.Create(context.Background(), policy))
	t.Cleanup(func() {
		_ = testEnv.Client.Delete(context.Background(), policy)
	})
}

// TestPolicyReport_VpolAuditEmitsFailResult drives a real Audit ValidatingPolicy
// through the real vpol admission handler: a violating Pod is admitted (Audit
// does not block), the handler emits a real EphemeralReport with a Fail result,
// and the framework aggregates it into a PolicyReport in a single pass (no
// controllers, no queues, the production merge from pkg/controllers/report/
// aggregate). The PolicyReport is persisted and read back, asserting Fail=1.
//
// User scenario: a platform team runs an audit-mode policy and reads the
// resulting PolicyReport to see which workloads violate it, without the policy
// blocking anything.
func TestPolicyReport_VpolAuditEmitsFailResult(t *testing.T) {
	const (
		policyName = "audit-require-team"
		ns         = "reports-flagship"
		podName    = "no-team-pod"
		podUID     = "pod-uid-reports-1"
		reportName = "polr-reports-flagship"
	)
	ctx := context.Background()

	framework.CreateNamespace(t, testEnv.KubeClient, ns)

	// Audit (not Deny): the request is allowed but a Fail result is reported.
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "has(object.metadata.labels) && 'team' in object.metadata.labels",
				Message:    "pods must have a team label",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit},
		},
	}
	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, testEnv.KyvernoClient, true, eventGen)

	podJSON := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "` + podName + `", "namespace": "` + ns + `", "uid": "` + podUID + `"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)

	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest(podName, ns, podJSON), "", time.Now())
	require.True(t, resp.Allowed, "audit policy must not block the request")

	// The handler emits the ephemeral report asynchronously; wait until the
	// single-pass aggregation sees exactly one Fail result.
	require.Eventually(t, func() bool {
		report, err := framework.AggregateEphemeralReports(ctx, testEnv.KyvernoClient, ns, reportName, types.UID(podUID), podScope(ns, podName, podUID), vpolMaps(policy))
		return err == nil && reportutils.CalculateSummary(report.GetResults()).Fail == 1
	}, 10*time.Second, 200*time.Millisecond, "expected an aggregated Fail result from the emitted ephemeral report")

	// Aggregate once more for the persisted assertion, then persist + read back.
	report, err := framework.AggregateEphemeralReports(ctx, testEnv.KyvernoClient, ns, reportName, types.UID(podUID), podScope(ns, podName, podUID), vpolMaps(policy))
	require.NoError(t, err)

	_, err = reportutils.CreatePermanentReport(ctx, report, testEnv.KyvernoClient, nil)
	require.NoError(t, err, "persisting the aggregated PolicyReport should succeed")
	t.Cleanup(func() {
		_ = testEnv.KyvernoClient.Wgpolicyk8sV1alpha2().PolicyReports(ns).Delete(context.Background(), reportName, metav1.DeleteOptions{})
	})

	persisted, err := testEnv.KyvernoClient.Wgpolicyk8sV1alpha2().PolicyReports(ns).Get(ctx, reportName, metav1.GetOptions{})
	require.NoError(t, err, "the persisted PolicyReport should be readable")
	assert.Equal(t, 1, persisted.Summary.Fail, "exactly one Fail result expected")
	assert.Equal(t, 0, persisted.Summary.Pass, "no Pass result expected")
}

// vpolMaps builds the active-policy filter for the aggregator. The key matches
// what the reports controller uses (cache.MetaObjectToName), so the real merge
// keeps results for this policy.
func vpolMaps(policy *policiesv1beta1.ValidatingPolicy) aggregate.Maps {
	return aggregate.Maps{Vpol: sets.New(cache.MetaObjectToName(policy).String())}
}

func podScope(ns, name, uid string) *corev1.ObjectReference {
	return &corev1.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: ns, Name: name, UID: types.UID(uid)}
}
