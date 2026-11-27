//go:build integration

package reports_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	vpol "github.com/kyverno/kyverno/pkg/webhooks/resource/vpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func teamLabelPolicy(name string, action admissionregistrationv1.ValidationAction) *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "has(object.metadata.labels) && 'team' in object.metadata.labels",
				Message:    "pods must have a team label",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{action},
		},
	}
}

func ephemeralReportsFor(t *testing.T, ns, uid string) int {
	t.Helper()
	selector, err := reportutils.SelectorResourceUidEquals(types.UID(uid))
	require.NoError(t, err)
	list, err := testEnv.KyvernoClient.ReportsV1().EphemeralReports(ns).List(context.Background(), metav1.ListOptions{LabelSelector: selector.String()})
	require.NoError(t, err)
	return len(list.Items)
}

// TestPolicyReport_VpolCompliantResourceEmitsPassResult is the counterpart of the Audit Fail test:
// a compliant pod must show up in the PolicyReport as a Pass, so a report with no failures means
// "checked and compliant", not "never checked".
func TestPolicyReport_VpolCompliantResourceEmitsPassResult(t *testing.T) {
	const (
		policyName = "audit-require-team-pass"
		ns         = "reports-pass"
		podName    = "team-pod"
		podUID     = "pod-uid-reports-pass"
		reportName = "polr-reports-pass"
	)
	ctx := context.Background()
	framework.CreateNamespace(t, testEnv.KubeClient, ns)

	policy := teamLabelPolicy(policyName, admissionregistrationv1.Audit)
	createPolicyWithCleanup(t, policy)
	framework.WaitForVpols(t, provider, policyName)

	h := vpol.New(engine, testEnv.ContextProvider, testEnv.KyvernoClient, true, &framework.MockEventGen{})
	podJSON := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "` + podName + `", "namespace": "` + ns + `", "uid": "` + podUID + `", "labels": {"team": "payments"}},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)

	policyCtx := framework.ContextWithPolicies(ctx, policyName)
	resp := h.ValidateClustered(policyCtx, logr.Discard(), framework.PodAdmissionRequest(podName, ns, podJSON), "", time.Now())
	require.True(t, resp.Allowed, "a compliant pod must be admitted")

	require.Eventually(t, func() bool {
		report, err := framework.AggregateEphemeralReports(ctx, testEnv.KyvernoClient, ns, reportName, types.UID(podUID), podScope(ns, podName, podUID), vpolMaps(policy))
		return err == nil && reportutils.CalculateSummary(report.GetResults()).Pass == 1
	}, 10*time.Second, 200*time.Millisecond, "expected an aggregated Pass result for the compliant pod")

	report, err := framework.AggregateEphemeralReports(ctx, testEnv.KyvernoClient, ns, reportName, types.UID(podUID), podScope(ns, podName, podUID), vpolMaps(policy))
	require.NoError(t, err)
	summary := reportutils.CalculateSummary(report.GetResults())
	assert.Equal(t, 1, summary.Pass, "exactly one Pass result expected")
	assert.Equal(t, 0, summary.Fail, "a compliant pod must not produce a Fail result")
	require.Len(t, report.GetResults(), 1)
	assert.Equal(t, policyName, report.GetResults()[0].Policy, "the result must be attributed to the evaluated policy")
}

// TestPolicyReport_VpolDenyBlockedRequestEmitsNoReport checks that a request a Deny policy
// rejects leaves no EphemeralReport behind. The resource was never created, so a report for it
// would show a violation against a pod that does not exist and would never be cleaned up by its
// owner's deletion.
func TestPolicyReport_VpolDenyBlockedRequestEmitsNoReport(t *testing.T) {
	const (
		policyName = "deny-require-team-noreport"
		ns         = "reports-blocked"
		podName    = "blocked-pod"
		podUID     = "pod-uid-reports-blocked"
	)
	framework.CreateNamespace(t, testEnv.KubeClient, ns)

	policy := teamLabelPolicy(policyName, admissionregistrationv1.Deny)
	createPolicyWithCleanup(t, policy)
	framework.WaitForVpols(t, provider, policyName)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, testEnv.KyvernoClient, true, eventGen)
	podJSON := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "` + podName + `", "namespace": "` + ns + `", "uid": "` + podUID + `"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)

	policyCtx := framework.ContextWithPolicies(context.Background(), policyName)
	resp := h.ValidateClustered(policyCtx, logr.Discard(), framework.PodAdmissionRequest(podName, ns, podJSON), "", time.Now())
	require.False(t, resp.Allowed, "the Deny policy must block the pod")

	// The handler waits for its audit goroutine before returning, so any report it was going to
	// write already exists. Keep checking for a moment anyway, so that a future change that makes
	// report writing fully asynchronous fails here instead of passing by accident.
	require.Never(t, func() bool {
		return ephemeralReportsFor(t, ns, podUID) > 0
	}, time.Second, 100*time.Millisecond, "a blocked request must not produce an EphemeralReport")

	// The block itself must still be visible: events are the audit trail for denied requests.
	assert.NotEmpty(t, eventGen.GetEvents(), "a blocked request must still generate events")
}

// TestPolicyReport_VpolDryRunEmitsNoReport checks that a server-side dry run (kubectl apply
// --dry-run=server) that would pass a policy does not write a report. The webhook is registered
// with sideEffects NoneOnDryRun, so writing one would break that contract.
func TestPolicyReport_VpolDryRunEmitsNoReport(t *testing.T) {
	const (
		policyName = "audit-require-team-dryrun"
		ns         = "reports-dryrun"
		podName    = "dryrun-pod"
		podUID     = "pod-uid-reports-dryrun"
	)
	framework.CreateNamespace(t, testEnv.KubeClient, ns)

	policy := teamLabelPolicy(policyName, admissionregistrationv1.Audit)
	createPolicyWithCleanup(t, policy)
	framework.WaitForVpols(t, provider, policyName)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, testEnv.KyvernoClient, true, eventGen)
	podJSON := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "` + podName + `", "namespace": "` + ns + `", "uid": "` + podUID + `"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)

	policyCtx := framework.ContextWithPolicies(context.Background(), policyName)
	resp := h.ValidateClustered(policyCtx, logr.Discard(), framework.PodAdmissionRequestDryRun(podName, ns, podJSON), "", time.Now())
	require.True(t, resp.Allowed, "an Audit policy must admit the dry run")

	require.Never(t, func() bool {
		return ephemeralReportsFor(t, ns, podUID) > 0
	}, time.Second, 100*time.Millisecond, "a dry run must not produce an EphemeralReport")
	assert.Empty(t, eventGen.GetEvents(), "a dry run must not generate events")
}
