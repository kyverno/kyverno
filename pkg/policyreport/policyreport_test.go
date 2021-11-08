package policyreport

import (
	"testing"
)

var validReportStatuses = []string{"pass", "fail", "error", "skip", "warn"}

func TestUpdateSummary_Successful(t *testing.T) {
	results := []interface{}{}
	for _, status := range validReportStatuses {
		results = append(results, map[string]interface{}{"result": status})
	}

	summary := updateSummary(results)

	for _, status := range validReportStatuses {
		if summary[status] != int64(1) {
			t.Errorf("Was expecting status %q to have a count of 1", status)
		}
	}
}
func TestUpdateSummary_MissingResultField(t *testing.T) {
	results := []interface{}{
		map[string]interface{}{"name": "test"},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Error("Function should not cause a panic")
		}
	}()

	summary := updateSummary(results)

	for _, status := range validReportStatuses {
		if summary[status] != int64(0) {
			t.Errorf("Was expecting status %q to have a count of 0", status)
		}
	}
}
