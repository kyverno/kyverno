package background

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
)

const celExceptionRevisionLabel = reportutils.LabelPrefixPolicyException + "background-revision"

// celExceptionRevision fingerprints the active exception set, including
// additions and removals. A stable ordering prevents informer-list order from
// changing the value. PolicyException resourceVersions remain deliberately
// sensitive to exception edits, unlike non-semantic CEL policy status changes.
func celExceptionRevision(exceptions []*policiesv1beta1.PolicyException) string {
	var revisions []string
	for _, exception := range exceptions {
		if exception != nil {
			revisions = append(revisions, fmt.Sprintf("%s/%s:%s:%s",
				exception.GetNamespace(), exception.GetName(), exception.GetUID(), exception.GetResourceVersion()))
		}
	}
	if len(revisions) == 0 {
		return ""
	}
	sort.Strings(revisions)
	sum := sha256.Sum256([]byte(strings.Join(revisions, "\n")))
	return fmt.Sprintf("e1-%x", sum[:12])
}

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
