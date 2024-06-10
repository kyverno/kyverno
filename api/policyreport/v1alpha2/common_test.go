package v1alpha2

import (
	"reflect"
	"testing"
)

func Test_PolicyReportSummary_ToMap(t *testing.T) {
	prs := PolicyReportSummary{
		Pass:  10,
		Fail:  5,
		Warn:  3,
		Error: 2,
		Skip:  1,
	}

	expectedMap := map[string]interface{}{
		"Pass":  10,
		"Fail":  5,
		"Warn":  3,
		"Error": 2,
		"Skip":  1,
	}

	actualMap := prs.ToMap()

	if !reflect.DeepEqual(actualMap, expectedMap) {
		t.Errorf("Expected map %v, but got %v", expectedMap, actualMap)
	}
}
