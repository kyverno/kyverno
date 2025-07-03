package openreports

import (
	"reflect"
	"testing"

	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewWGPolAdapter(t *testing.T) {
	rep := policyreportv1alpha2.PolicyReport{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-polr",
		},
	}

	adapter := NewWGPolAdapter(&rep)

	if adapter.PolicyReport.Name != "test-polr" {
		t.Errorf("Expected adapter PolicyReport name to be 'test-polr', got %s", adapter.PolicyReport.Name)
	}

	if adapter.or.Report.Name != "test-polr" {
		t.Errorf("Expected adapter openreports Report name to be 'test-polr', got %s", adapter.or.Report.Name)
	}
}

func TestNewWGCpolAdapter(t *testing.T) {
	cr := policyreportv1alpha2.ClusterPolicyReport{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cpolr",
		},
	}

	adapter := NewWGCpolAdapter(&cr)

	if adapter.ClusterPolicyReport.Name != "test-cpolr" {
		t.Errorf("Expected adapter ClusterPolicyReport name to be 'test-cpolr', got %s", adapter.ClusterPolicyReport.Name)
	}

	if adapter.or.ClusterReport.Name != "test-cpolr" {
		t.Errorf("Expected adapter openreports ClusterReport name to be 'test-cpolr', got %s", adapter.or.ClusterReport.Name)
	}
}

func TestWgpolicyReportAdapter_SetResults(t *testing.T) {
	initialReport := &policyreportv1alpha2.PolicyReport{}
	adapter := &WgpolicyReportAdapter{
		PolicyReport: initialReport,
		or:           &ReportAdapter{Report: &openreportsv1alpha1.Report{}},
	}

	res := []openreportsv1alpha1.ReportResult{
		{
			Source:      "test-source",
			Policy:      "test-policy",
			Subjects:    []corev1.ObjectReference{{APIVersion: "v1", Kind: "Pod", Name: "test-pod", Namespace: "test-ns"}},
			Rule:        "test-rule",
			Description: "test-description",
			Severity:    "medium",
			Result:      "fail",
			Scored:      true,
			Category:    "test-category",
		},
	}

	adapter.SetResults(res)

	if len(adapter.PolicyReport.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(adapter.PolicyReport.Results))
	}

	expectedWgRes := policyreportv1alpha2.PolicyReportResult{
		Source:    "test-source",
		Policy:    "test-policy",
		Resources: []corev1.ObjectReference{{APIVersion: "v1", Kind: "Pod", Name: "test-pod", Namespace: "test-ns"}},
		Rule:      "test-rule",
		Message:   "test-description",
		Severity:  policyreportv1alpha2.PolicySeverity("medium"),
		Result:    policyreportv1alpha2.PolicyResult("fail"),
		Scored:    true,
		Category:  "test-category",
	}

	if !reflect.DeepEqual(adapter.PolicyReport.Results[0], expectedWgRes) {
		t.Errorf("SetResults did not set expected PolicyReportResult.\nExpected: %+v\nGot: %+v", expectedWgRes, adapter.PolicyReport.Results[0])
	}
}

func TestWgpolicyReportAdapter_SetSummary(t *testing.T) {
	initialReport := &policyreportv1alpha2.PolicyReport{}
	adapter := &WgpolicyReportAdapter{
		PolicyReport: initialReport,
		or:           &ReportAdapter{Report: &openreportsv1alpha1.Report{}},
	}

	summary := openreportsv1alpha1.ReportSummary{
		Pass:  1,
		Fail:  2,
		Warn:  3,
		Skip:  4,
		Error: 5,
	}

	adapter.SetSummary(summary)

	expectedSummary := policyreportv1alpha2.PolicyReportSummary{
		Pass:  1,
		Fail:  2,
		Warn:  3,
		Skip:  4,
		Error: 5,
	}

	if !reflect.DeepEqual(adapter.PolicyReport.Summary, expectedSummary) {
		t.Errorf("SetSummary did not set expected PolicyReportSummary.\nExpected: %+v\nGot: %+v", expectedSummary, adapter.PolicyReport.Summary)
	}
}

func TestWgpolicyClusterReportAdapter_SetResults(t *testing.T) {
	initialClusterReport := &policyreportv1alpha2.ClusterPolicyReport{}
	adapter := &WgpolicyClusterReportAdapter{
		ClusterPolicyReport: initialClusterReport,
		or:                  &ClusterReportAdapter{ClusterReport: &openreportsv1alpha1.ClusterReport{}},
	}

	res := []openreportsv1alpha1.ReportResult{
		{
			Source:      "test-source-cluster",
			Policy:      "test-policy-cluster",
			Subjects:    []corev1.ObjectReference{{APIVersion: "v1", Kind: "Namespace", Name: "test-ns"}},
			Rule:        "test-rule-cluster",
			Description: "test-description-cluster",
			Severity:    "high",
			Scored:      false,
			Category:    "test-category-cluster",
		},
	}

	adapter.SetResults(res)

	if len(adapter.ClusterPolicyReport.Results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(adapter.ClusterPolicyReport.Results))
	}

	expectedWgRes := policyreportv1alpha2.PolicyReportResult{
		Source:    "test-source-cluster",
		Policy:    "test-policy-cluster",
		Resources: []corev1.ObjectReference{{APIVersion: "v1", Kind: "Namespace", Name: "test-ns"}},
		Rule:      "test-rule-cluster",
		Message:   "test-description-cluster",
		Severity:  policyreportv1alpha2.PolicySeverity("high"),
		Scored:    false,
		Category:  "test-category-cluster",
	}

	if !reflect.DeepEqual(adapter.ClusterPolicyReport.Results[0], expectedWgRes) {
		t.Errorf("SetResults did not set expected ClusterPolicyReportResult.\nExpected: %+v\nGot: %+v", expectedWgRes, adapter.ClusterPolicyReport.Results[0])
	}
}

func TestWgpolicyClusterReportAdapter_SetSummary(t *testing.T) {
	initialClusterReport := &policyreportv1alpha2.ClusterPolicyReport{}
	adapter := &WgpolicyClusterReportAdapter{
		ClusterPolicyReport: initialClusterReport,
		or:                  &ClusterReportAdapter{ClusterReport: &openreportsv1alpha1.ClusterReport{}},
	}

	summary := openreportsv1alpha1.ReportSummary{
		Pass:  10,
		Fail:  20,
		Warn:  30,
		Skip:  40,
		Error: 50,
	}

	adapter.SetSummary(summary)

	expectedSummary := policyreportv1alpha2.PolicyReportSummary{
		Pass:  10,
		Fail:  20,
		Warn:  30,
		Skip:  40,
		Error: 50,
	}

	if !reflect.DeepEqual(adapter.ClusterPolicyReport.Summary, expectedSummary) {
		t.Errorf("SetSummary did not set expected ClusterPolicyReportSummary.\nExpected: %+v\nGot: %+v", expectedSummary, adapter.ClusterPolicyReport.Summary)
	}
}
