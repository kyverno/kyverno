package background

import (
	"strings"

	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
)

// keepResultForEffectiveRevision retains a result only when the policy that
// produced it, every named binding, and its known exceptions have not changed.
// Old resourceVersion labels do not equal the new semantic revision, so results
// are replaced on the first migration scan instead of being duplicated.
func keepResultForEffectiveRevision(result openreportsv1alpha1.ReportResult, labelsByName, expected, actual map[string]string) bool {
	label := labelsByName[result.Policy]
	if label == "" || expected[label] == "" || expected[label] != actual[label] {
		return false
	}
	for _, property := range []string{"binding", "mapBinding"} {
		if name := result.Properties[property]; name != "" {
			bindingLabel := labelsByName[name]
			if bindingLabel == "" || expected[bindingLabel] == "" || expected[bindingLabel] != actual[bindingLabel] {
				return false
			}
		}
	}
	for _, exception := range strings.Split(result.Properties["exceptions"], ",") {
		exceptionLabel := labelsByName[exception]
		if exceptionLabel != "" && expected[exceptionLabel] != actual[exceptionLabel] {
			return false
		}
	}
	return true
}
