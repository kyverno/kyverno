package utils

import (
	"testing"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/openreports"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReportsAreIdentical_EmptyReports(t *testing.T) {
	before := &reportsv1.EphemeralReport{}
	after := &reportsv1.EphemeralReport{}

	result := ReportsAreIdentical(before, after)
	assert.True(t, result, "empty reports should be identical")
}

func TestReportsAreIdentical_SameResults(t *testing.T) {
	results := []openreportsv1alpha1.ReportResult{
		{
			Policy: "test-policy",
			Rule:   "test-rule",
			Result: openreportsv1alpha1.Result(openreports.StatusPass),
		},
	}
	before := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "report1",
			Labels:      map[string]string{"app": "test"},
			Annotations: map[string]string{"key": "value"},
		},
	}
	before.SetResults(results)

	after := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "report1",
			Labels:      map[string]string{"app": "test"},
			Annotations: map[string]string{"key": "value"},
		},
	}
	after.SetResults(results)

	result := ReportsAreIdentical(before, after)
	assert.True(t, result, "reports with same results should be identical")
}

func TestReportsAreIdentical_DifferentResults(t *testing.T) {
	before := &reportsv1.EphemeralReport{}
	before.SetResults([]openreportsv1alpha1.ReportResult{
		{Policy: "policy1", Result: openreportsv1alpha1.Result(openreports.StatusPass)},
	})

	after := &reportsv1.EphemeralReport{}
	after.SetResults([]openreportsv1alpha1.ReportResult{
		{Policy: "policy1", Result: openreportsv1alpha1.Result(openreports.StatusFail)},
	})

	result := ReportsAreIdentical(before, after)
	assert.False(t, result, "reports with different results should not be identical")
}

func TestReportsAreIdentical_DifferentResultCount(t *testing.T) {
	before := &reportsv1.EphemeralReport{}
	before.SetResults([]openreportsv1alpha1.ReportResult{
		{Policy: "policy1", Result: openreportsv1alpha1.Result(openreports.StatusPass)},
	})

	after := &reportsv1.EphemeralReport{}
	after.SetResults([]openreportsv1alpha1.ReportResult{
		{Policy: "policy1", Result: openreportsv1alpha1.Result(openreports.StatusPass)},
		{Policy: "policy2", Result: openreportsv1alpha1.Result(openreports.StatusPass)},
	})

	result := ReportsAreIdentical(before, after)
	assert.False(t, result, "reports with different result count should not be identical")
}

func TestReportsAreIdentical_DifferentLabels(t *testing.T) {
	before := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"app": "test"},
		},
	}
	after := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"app": "different"},
		},
	}

	result := ReportsAreIdentical(before, after)
	assert.False(t, result, "reports with different labels should not be identical")
}

func TestReportsAreIdentical_DifferentAnnotations(t *testing.T) {
	before := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"key": "value1"},
		},
	}
	after := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"key": "value2"},
		},
	}

	result := ReportsAreIdentical(before, after)
	assert.False(t, result, "reports with different annotations should not be identical")
}

func TestReportsAreIdentical_TimestampIgnored(t *testing.T) {
	before := &reportsv1.EphemeralReport{}
	before.SetResults([]openreportsv1alpha1.ReportResult{
		{
			Policy:    "policy1",
			Result:    openreportsv1alpha1.Result(openreports.StatusPass),
			Timestamp: metav1.Timestamp{Seconds: 1000},
		},
	})

	after := &reportsv1.EphemeralReport{}
	after.SetResults([]openreportsv1alpha1.ReportResult{
		{
			Policy:    "policy1",
			Result:    openreportsv1alpha1.Result(openreports.StatusPass),
			Timestamp: metav1.Timestamp{Seconds: 2000},
		},
	})

	result := ReportsAreIdentical(before, after)
	assert.True(t, result, "reports with same content but different timestamps should be identical")
}

func TestGetExcludeReportingLabelRequirement(t *testing.T) {
	req, err := getExcludeReportingLabelRequirement()
	assert.NoError(t, err)
	assert.NotNil(t, req)
}

func TestGetIncludeReportingLabelRequirement(t *testing.T) {
	req, err := getIncludeReportingLabelRequirement()
	assert.NoError(t, err)
	assert.NotNil(t, req)
}
