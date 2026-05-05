package report

import (
	"sync"
	"testing"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/openreports"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_CalculateSummary_all_pass(t *testing.T) {
	t.Parallel()
	results := []openreportsv1alpha1.ReportResult{
		{Result: openreports.StatusPass},
		{Result: openreports.StatusPass},
		{Result: openreports.StatusPass},
	}

	summary := CalculateSummary(results)

	assert.Equal(t, 3, summary.Pass)
	assert.Equal(t, 0, summary.Fail)
	assert.Equal(t, 0, summary.Warn)
	assert.Equal(t, 0, summary.Error)
	assert.Equal(t, 0, summary.Skip)
}

func Test_CalculateSummary_mixed_results(t *testing.T) {
	t.Parallel()
	results := []openreportsv1alpha1.ReportResult{
		{Result: openreports.StatusPass},
		{Result: openreports.StatusFail},
		{Result: openreports.StatusWarn},
		{Result: openreports.StatusError},
		{Result: openreports.StatusSkip},
		{Result: openreports.StatusFail},
	}

	summary := CalculateSummary(results)

	assert.Equal(t, 1, summary.Pass)
	assert.Equal(t, 2, summary.Fail)
	assert.Equal(t, 1, summary.Warn)
	assert.Equal(t, 1, summary.Error)
	assert.Equal(t, 1, summary.Skip)
}

func Test_CalculateSummary_empty(t *testing.T) {
	t.Parallel()
	results := []openreportsv1alpha1.ReportResult{}

	summary := CalculateSummary(results)

	assert.Equal(t, 0, summary.Pass)
	assert.Equal(t, 0, summary.Fail)
}

func Test_SeverityFromString_valid_values(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected openreportsv1alpha1.ResultSeverity
	}{
		{"critical", openreports.SeverityCritical},
		{"high", openreports.SeverityHigh},
		{"medium", openreports.SeverityMedium},
		{"low", openreports.SeverityLow},
		{"info", openreports.SeverityInfo},
	}

	for _, tc := range tests {
		got := SeverityFromString(tc.input)
		assert.Equal(t, tc.expected, got, "input: %s", tc.input)
	}
}

func Test_SeverityFromString_unknown(t *testing.T) {
	t.Parallel()
	got := SeverityFromString("unknown")
	assert.Empty(t, got)
}

func Test_SeverityFromString_empty(t *testing.T) {
	t.Parallel()
	got := SeverityFromString("")
	assert.Empty(t, got)
}

func Test_toPolicyResult_status_mapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status   engineapi.RuleStatus
		expected openreportsv1alpha1.Result
	}{
		{engineapi.RuleStatusPass, openreports.StatusPass},
		{engineapi.RuleStatusFail, openreports.StatusFail},
		{engineapi.RuleStatusError, openreports.StatusError},
		{engineapi.RuleStatusWarn, openreports.StatusWarn},
		{engineapi.RuleStatusSkip, openreports.StatusSkip},
	}

	for _, tc := range tests {
		got := toPolicyResult(tc.status)
		assert.Equal(t, tc.expected, got)
	}
}

func Test_selectProcess_background(t *testing.T) {
	t.Parallel()
	got := selectProcess(true, false)
	assert.Equal(t, "background scan", got)
}

func Test_selectProcess_admission(t *testing.T) {
	t.Parallel()
	got := selectProcess(false, true)
	assert.Equal(t, "admission review", got)
}

func Test_selectProcess_neither(t *testing.T) {
	t.Parallel()
	got := selectProcess(false, false)
	assert.Empty(t, got)
}

func Test_selectProcess_both_prefers_background(t *testing.T) {
	t.Parallel()
	// when both are true, background takes precedence
	got := selectProcess(true, true)
	assert.Equal(t, "background scan", got)
}

func Test_addProperty_creates_map(t *testing.T) {
	t.Parallel()
	result := &openreportsv1alpha1.ReportResult{}

	addProperty("key", "value", result)

	assert.NotNil(t, result.Properties)
	assert.Equal(t, "value", result.Properties["key"])
}

func Test_addProperty_appends_to_existing(t *testing.T) {
	t.Parallel()
	result := &openreportsv1alpha1.ReportResult{
		Properties: map[string]string{"existing": "data"},
	}

	addProperty("new", "value", result)

	assert.Equal(t, "data", result.Properties["existing"])
	assert.Equal(t, "value", result.Properties["new"])
}

func Test_getResourceInfo_namespaced(t *testing.T) {
	t.Parallel()
	gvk := schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}

	info := getResourceInfo(gvk, "nginx", "prod")

	assert.Contains(t, info, "apps/v1, Kind=Deployment")
	assert.Contains(t, info, "Name=nginx")
	assert.Contains(t, info, "Namespace=prod")
}

func Test_getResourceInfo_cluster_scoped(t *testing.T) {
	t.Parallel()
	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Namespace",
	}

	info := getResourceInfo(gvk, "kube-system", "")

	assert.Contains(t, info, "Kind=Namespace")
	assert.Contains(t, info, "Name=kube-system")
	assert.NotContains(t, info, "Namespace=")
}

func Test_SortReportResults_by_policy(t *testing.T) {
	t.Parallel()
	results := []openreportsv1alpha1.ReportResult{
		{Policy: "z-policy", Rule: "rule1"},
		{Policy: "a-policy", Rule: "rule1"},
		{Policy: "m-policy", Rule: "rule1"},
	}

	SortReportResults(results)

	assert.Equal(t, "a-policy", results[0].Policy)
	assert.Equal(t, "m-policy", results[1].Policy)
	assert.Equal(t, "z-policy", results[2].Policy)
}

func Test_SortReportResults_by_rule_same_policy(t *testing.T) {
	t.Parallel()
	results := []openreportsv1alpha1.ReportResult{
		{Policy: "my-policy", Rule: "validate-z"},
		{Policy: "my-policy", Rule: "validate-a"},
		{Policy: "my-policy", Rule: "validate-m"},
	}

	SortReportResults(results)

	assert.Equal(t, "validate-a", results[0].Rule)
	assert.Equal(t, "validate-m", results[1].Rule)
	assert.Equal(t, "validate-z", results[2].Rule)
}

func Test_SortReportResults_by_source(t *testing.T) {
	t.Parallel()
	results := []openreportsv1alpha1.ReportResult{
		{Policy: "pol", Rule: "rule", Source: "Kyverno"},
		{Policy: "pol", Rule: "rule", Source: "Admission"},
	}

	SortReportResults(results)

	assert.Equal(t, "Admission", results[0].Source)
	assert.Equal(t, "Kyverno", results[1].Source)
}

func Test_SortReportResults_by_timestamp(t *testing.T) {
	t.Parallel()
	now := time.Now()
	results := []openreportsv1alpha1.ReportResult{
		{Policy: "pol", Rule: "rule", Timestamp: metav1.Timestamp{Seconds: now.Unix() + 100}},
		{Policy: "pol", Rule: "rule", Timestamp: metav1.Timestamp{Seconds: now.Unix()}},
		{Policy: "pol", Rule: "rule", Timestamp: metav1.Timestamp{Seconds: now.Unix() + 50}},
	}

	SortReportResults(results)

	assert.Equal(t, now.Unix(), results[0].Timestamp.Seconds)
	assert.Equal(t, now.Unix()+50, results[1].Timestamp.Seconds)
	assert.Equal(t, now.Unix()+100, results[2].Timestamp.Seconds)
}

func Test_SortReportResults_empty(t *testing.T) {
	t.Parallel()
	results := []openreportsv1alpha1.ReportResult{}

	// should not panic
	SortReportResults(results)

	assert.Empty(t, results)
}

func Test_SortReportResults_single(t *testing.T) {
	t.Parallel()
	results := []openreportsv1alpha1.ReportResult{
		{Policy: "only-one"},
	}

	SortReportResults(results)

	assert.Equal(t, "only-one", results[0].Policy)
}

func TestToPolicyReportResult_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	// Build a shared RuleResponse with properties -- this is the map that
	// caused the concurrent-write panic in issue #15617.
	sharedProps := map[string]string{
		"passMessage": "resource is compliant",
		"category":    "best-practices",
	}
	ruleResp := engineapi.NewRuleResponse(
		"validate-labels",
		engineapi.Validation,
		"labels are valid",
		engineapi.RuleStatusPass,
		sharedProps,
	)

	// Minimal ClusterPolicy to satisfy GenericPolicy interface.
	pol := engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "require-labels",
		},
	})

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	results := make([]openreportsv1alpha1.ReportResult, goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx] = ToPolicyReportResult(pol, *ruleResp, nil)
		}(i)
	}
	wg.Wait()

	// Every goroutine should have produced a valid result with the
	// overridden description from passMessage.
	for i, r := range results {
		assert.Equal(t, "require-labels", r.Policy, "goroutine %d", i)
		assert.Equal(t, "validate-labels", r.Rule, "goroutine %d", i)
		assert.Equal(t, openreportsv1alpha1.Result(openreports.StatusPass), r.Result, "goroutine %d", i)
		assert.Equal(t, "resource is compliant", r.Description, "goroutine %d", i)
	}
}
