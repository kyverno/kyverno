package background

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
)

// celExceptionRevisionLabel is the legacy, report-wide exception revision.
// Refresh CEL results once when migrating it, since its previous membership is unknown.
const celExceptionRevisionLabel = reportutils.LabelPrefixPolicyException + "background-revision"

// celExceptionPolicyLabel identifies a CEL policy by kind, namespace and name.
// Hashing keeps the label name within Kubernetes' 63-character limit.
func celExceptionPolicyLabel(policy engineapi.GenericPolicy) string {
	if policy.AsValidatingPolicyLike() == nil && policy.AsMutatingPolicyLike() == nil && policy.AsImageValidatingPolicyLike() == nil {
		return ""
	}
	sum := sha256.Sum256([]byte(policy.GetKind() + "/" + policy.GetNamespace() + "/" + policy.GetName()))
	return celExceptionRevisionLabel + fmt.Sprintf("-%x", sum[:12])
}

// policyRevisions uses the same exception scope as the scanner's CEL providers.
// Native admission policies do not consume CEL exceptions.
func (c *controller) policyRevisions(policies []engineapi.GenericPolicy, exceptions []*policiesv1beta1.PolicyException) map[string]string {
	revisions := make(map[string]string)
	scope := celengine.ExceptionScope(c.exceptionNamespace)
	for _, policy := range policies {
		revisions[reportutils.PolicyLabel(policy)] = reportutils.PolicyRevision(policy)
		if label := celExceptionPolicyLabel(policy); label != "" {
			var matching []*policiesv1beta1.PolicyException
			for _, exception := range exceptions {
				if exception != nil && scope.Applies(exception, policy.GetKind(), policy.GetName(), policy.GetNamespace()) {
					matching = append(matching, exception)
				}
			}
			if revision := celExceptionRevision(matching); revision != "" {
				revisions[label] = revision
			}
		}
	}
	return revisions
}

func policyExceptionRevisionChanged(policy engineapi.GenericPolicy, expected, actual map[string]string) bool {
	label := celExceptionPolicyLabel(policy)
	return label != "" && (expected[label] != actual[label] || actual[celExceptionRevisionLabel] != "")
}

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
