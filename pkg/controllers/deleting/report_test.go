package deleting

import (
	"context"
	"errors"
	"testing"

	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
)

func makeResource(kind, namespace, name, apiVersion string) unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetKind(kind)
	u.SetNamespace(namespace)
	u.SetName(name)
	u.SetAPIVersion(apiVersion)
	return u
}

func makeDeletingPolicy(name, namespace string) *policiesv1beta1.DeletingPolicy {
	return &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// ---------------------------------------------------------------------------
// reportWriter.Write — cluster-scoped DeletingPolicy → ClusterPolicyReport
// ---------------------------------------------------------------------------

func TestReportWriter_Write_ClusterScope_Creates(t *testing.T) {
	ctx := context.Background()
	kyvernoClient := versionedfake.NewSimpleClientset()
	rw := &reportWriter{kyvernoClient: kyvernoClient}

	pol := makeDeletingPolicy("my-dpol", "")
	res := makeResource("Pod", "default", "pod-1", "v1")

	records := []deletionRecord{
		{resource: res, err: nil},
	}
	rw.Write(ctx, pol, records)

	reportName := "dpol-my-dpol"
	report, err := kyvernoClient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Get(ctx, reportName, metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, report.Results, 1)
	assert.Equal(t, policyreportv1alpha2.StatusPass, report.Results[0].Result)
	assert.Equal(t, reportutils.SourceDeletingPolicy, report.Results[0].Source)
	assert.Equal(t, "my-dpol", report.Results[0].Policy)
	assert.Equal(t, 1, report.Summary.Pass)
}

// ---------------------------------------------------------------------------
// reportWriter.Write — cluster-scoped DeletingPolicy with a deletion error
// ---------------------------------------------------------------------------

func TestReportWriter_Write_ClusterScope_Error(t *testing.T) {
	ctx := context.Background()
	kyvernoClient := versionedfake.NewSimpleClientset()
	rw := &reportWriter{kyvernoClient: kyvernoClient}

	pol := makeDeletingPolicy("my-dpol", "")
	res := makeResource("Pod", "default", "pod-1", "v1")
	delErr := errors.New("permission denied")

	records := []deletionRecord{
		{resource: res, err: delErr},
	}
	rw.Write(ctx, pol, records)

	report, err := kyvernoClient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Get(ctx, "dpol-my-dpol", metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, report.Results, 1)
	assert.Equal(t, policyreportv1alpha2.StatusError, report.Results[0].Result)
	assert.Contains(t, report.Results[0].Message, "permission denied")
	assert.Equal(t, 1, report.Summary.Error)
}

// ---------------------------------------------------------------------------
// reportWriter.Write — second run updates existing ClusterPolicyReport
// ---------------------------------------------------------------------------

func TestReportWriter_Write_ClusterScope_Updates(t *testing.T) {
	ctx := context.Background()
	kyvernoClient := versionedfake.NewSimpleClientset()
	rw := &reportWriter{kyvernoClient: kyvernoClient}

	pol := makeDeletingPolicy("my-dpol", "")
	res1 := makeResource("Pod", "default", "pod-1", "v1")
	res2 := makeResource("Pod", "default", "pod-2", "v1")

	// First run: 1 deleted
	rw.Write(ctx, pol, []deletionRecord{{resource: res1}})

	// Second run: 2 deleted
	rw.Write(ctx, pol, []deletionRecord{{resource: res1}, {resource: res2}})

	report, err := kyvernoClient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Get(ctx, "dpol-my-dpol", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Len(t, report.Results, 2, "second Write should replace, not append")
	assert.Equal(t, 2, report.Summary.Pass)
}

// ---------------------------------------------------------------------------
// reportWriter.Write — namespaced NamespacedDeletingPolicy → PolicyReport
// ---------------------------------------------------------------------------

func TestReportWriter_Write_Namespaced_Creates(t *testing.T) {
	ctx := context.Background()
	kyvernoClient := versionedfake.NewSimpleClientset()
	rw := &reportWriter{kyvernoClient: kyvernoClient}

	pol := &policiesv1beta1.NamespacedDeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ns-dpol",
			Namespace: "team-a",
		},
	}
	res := makeResource("ConfigMap", "team-a", "cm-1", "v1")

	rw.Write(ctx, pol, []deletionRecord{{resource: res}})

	report, err := kyvernoClient.Wgpolicyk8sV1alpha2().PolicyReports("team-a").Get(ctx, "dpol-ns-dpol", metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, report.Results, 1)
	assert.Equal(t, policyreportv1alpha2.StatusPass, report.Results[0].Result)
}

// ---------------------------------------------------------------------------
// reportWriter.Write — empty records produces a report with zero results
// ---------------------------------------------------------------------------

func TestReportWriter_Write_EmptyRecords(t *testing.T) {
	ctx := context.Background()
	kyvernoClient := versionedfake.NewSimpleClientset()
	rw := &reportWriter{kyvernoClient: kyvernoClient}

	pol := makeDeletingPolicy("empty-dpol", "")
	rw.Write(ctx, pol, []deletionRecord{})

	report, err := kyvernoClient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Get(ctx, "dpol-empty-dpol", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Empty(t, report.Results)
	assert.Zero(t, report.Summary.Pass)
}

// ---------------------------------------------------------------------------
// DeletingPolicyToReportResult helper
// ---------------------------------------------------------------------------

func TestDeletingPolicyToReportResult_Pass(t *testing.T) {
	res := makeResource("Deployment", "prod", "old-deploy", "apps/v1")
	result := reportutils.DeletingPolicyToReportResult("my-policy", res, nil)

	assert.Equal(t, reportutils.SourceDeletingPolicy, result.Source)
	assert.Equal(t, "my-policy", result.Policy)
	assert.Equal(t, "pass", string(result.Result))
	assert.Contains(t, result.Description, "old-deploy")
	require.Len(t, result.Subjects, 1)
	assert.Equal(t, "Deployment", result.Subjects[0].Kind)
}

func TestDeletingPolicyToReportResult_Error(t *testing.T) {
	res := makeResource("Deployment", "prod", "old-deploy", "apps/v1")
	delErr := errors.New("some transient error")
	result := reportutils.DeletingPolicyToReportResult("my-policy", res, delErr)

	assert.Equal(t, "error", string(result.Result))
	assert.Contains(t, result.Description, "some transient error")
}
