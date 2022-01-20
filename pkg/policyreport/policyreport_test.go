package policyreport

import (
	"testing"

	report "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
)

var validReportStatuses = []string{"pass", "fail", "error", "skip", "warn"}

func TestUpdateSummary_Successful(t *testing.T) {
	results := []report.PolicyReportResult{}
	for _, status := range validReportStatuses {
		results = append(results, report.PolicyReportResult{
			Result: report.PolicyResult(status),
			Policy: "TestUpdateSummary_Successful",
			Source: "Kyverno",
		})
	}

	summary := updateSummary(results)

	if summary.Pass != 1 {
		t.Errorf("Was expecting status pass to have a count of 1")
	}
	if summary.Error != 1 {
		t.Errorf("Was expecting status error to have a count of 1")
	}
	if summary.Fail != 1 {
		t.Errorf("Was expecting status fail to have a count of 1")
	}
	if summary.Skip != 1 {
		t.Errorf("Was expecting status skip to have a count of 1")
	}
	if summary.Warn != 1 {
		t.Errorf("Was expecting status warn to have a count of 1")
	}
}
func TestUpdateSummary_MissingResultField(t *testing.T) {
	results := []report.PolicyReportResult{
		{
			Policy: "TestUpdateSummary_MissingResultField",
			Source: "Kyverno",
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Error("Function should not cause a panic")
		}
	}()

	summary := updateSummary(results)

	if summary.Pass != 0 {
		t.Errorf("Was expecting status pass to have a count of 0")
	}
	if summary.Error != 0 {
		t.Errorf("Was expecting status error to have a count of 0")
	}
	if summary.Fail != 0 {
		t.Errorf("Was expecting status fail to have a count of 0")
	}
	if summary.Skip != 0 {
		t.Errorf("Was expecting status skip to have a count of 0")
	}
	if summary.Warn != 0 {
		t.Errorf("Was expecting status warn to have a count of 0")
	}
}
