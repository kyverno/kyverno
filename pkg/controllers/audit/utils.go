package audit

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/policy"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
)

const (
	categoryLabel string = "policies.kyverno.io/category"
	severityLabel string = "policies.kyverno.io/severity"
	ScoredLabel   string = "policies.kyverno.io/scored"
)

func canBackgroundProcess(logger logr.Logger, p kyvernov1.PolicyInterface) bool {
	if !p.BackgroundProcessingEnabled() {
		return false
	}
	if err := policy.ValidateVariables(p, true); err != nil {
		return false
	}
	return true
}

func buildKindSet(logger logr.Logger, policies ...kyvernov1.PolicyInterface) sets.String {
	kinds := sets.NewString()
	for _, policy := range policies {
		for _, rule := range autogen.ComputeRules(policy) {
			if rule.HasValidate() || rule.HasVerifyImages() {
				kinds.Insert(rule.MatchResources.GetKinds()...)
			}
		}
	}
	return kinds
}

func removeNonBackgroundPolicies(logger logr.Logger, policies ...kyvernov1.PolicyInterface) []kyvernov1.PolicyInterface {
	var backgroundPolicies []kyvernov1.PolicyInterface
	for _, pol := range policies {
		if canBackgroundProcess(logger, pol) {
			backgroundPolicies = append(backgroundPolicies, pol)
		}
	}
	return backgroundPolicies
}

func CalculateSummary(results []policyreportv1alpha2.PolicyReportResult) (summary policyreportv1alpha2.PolicyReportSummary) {
	for _, res := range results {
		switch string(res.Result) {
		case policyreportv1alpha2.StatusPass:
			summary.Pass++
		case policyreportv1alpha2.StatusFail:
			summary.Fail++
		case policyreportv1alpha2.StatusWarn:
			summary.Warn++
		case policyreportv1alpha2.StatusError:
			summary.Error++
		case policyreportv1alpha2.StatusSkip:
			summary.Skip++
		}
	}
	return
}

func SortReportResults(results []policyreportv1alpha2.PolicyReportResult) {
	slices.SortFunc(results, func(a policyreportv1alpha2.PolicyReportResult, b policyreportv1alpha2.PolicyReportResult) bool {
		if a.Policy != b.Policy {
			return a.Policy < b.Policy
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		if len(a.Resources) != len(b.Resources) {
			return len(a.Resources) < len(b.Resources)
		}
		for i := range a.Resources {
			if a.Resources[i].UID != b.Resources[i].UID {
				return a.Resources[i].UID < b.Resources[i].UID
			}
		}
		return false
	})
}

func SplitResultsByPolicy(results []policyreportv1alpha2.PolicyReportResult) map[string][]policyreportv1alpha2.PolicyReportResult {
	resultsMap := map[string][]policyreportv1alpha2.PolicyReportResult{}
	keysMap := map[string]string{}
	for _, result := range results {
		if keysMap[result.Policy] == "" {
			// TODO error checking
			ns, n, _ := cache.SplitMetaNamespaceKey(result.Policy)
			if ns == "" {
				keysMap[result.Policy] = "cpol-" + n
			} else {
				keysMap[result.Policy] = "pol-" + n
			}
		}
	}
	for _, result := range results {
		key := keysMap[result.Policy]
		resultsMap[key] = append(resultsMap[key], result)
	}
	for _, result := range resultsMap {
		SortReportResults(result)
	}
	return resultsMap
}

func toPolicyResult(status response.RuleStatus) policyreportv1alpha2.PolicyResult {
	switch status {
	case response.RuleStatusPass:
		return policyreportv1alpha2.StatusPass
	case response.RuleStatusFail:
		return policyreportv1alpha2.StatusFail
	case response.RuleStatusError:
		return policyreportv1alpha2.StatusError
	case response.RuleStatusWarn:
		return policyreportv1alpha2.StatusWarn
	case response.RuleStatusSkip:
		return policyreportv1alpha2.StatusSkip
	}
	return ""
}

func severityFromString(severity string) policyreportv1alpha2.PolicySeverity {
	switch severity {
	case policyreportv1alpha2.SeverityHigh:
		return policyreportv1alpha2.SeverityHigh
	case policyreportv1alpha2.SeverityMedium:
		return policyreportv1alpha2.SeverityMedium
	case policyreportv1alpha2.SeverityLow:
		return policyreportv1alpha2.SeverityLow
	}
	return ""
}

func toReportResults(scanResult ScanResult) []policyreportv1alpha2.PolicyReportResult {
	if scanResult.Error != nil {
		return nil
	}
	key, _ := cache.MetaNamespaceKeyFunc(scanResult.EngineResponse.Policy)
	var results []policyreportv1alpha2.PolicyReportResult
	for _, ruleResult := range scanResult.EngineResponse.PolicyResponse.Rules {
		annotations := scanResult.EngineResponse.Policy.GetAnnotations()
		result := policyreportv1alpha2.PolicyReportResult{
			Source: kyvernov1.KyvernoAppValue,
			Policy: key,
			Rule:   ruleResult.Name,
			Resources: []corev1.ObjectReference{
				{
					Kind:       scanResult.EngineResponse.PatchedResource.GetKind(),
					Namespace:  scanResult.EngineResponse.PatchedResource.GetNamespace(),
					APIVersion: scanResult.EngineResponse.PatchedResource.GetAPIVersion(),
					Name:       scanResult.EngineResponse.PatchedResource.GetName(),
					UID:        scanResult.EngineResponse.PatchedResource.GetUID(),
				},
			},
			Message: ruleResult.Message,
			Result:  toPolicyResult(ruleResult.Status),
			Scored:  annotations[categoryLabel] != "false",
			// TODO this is going to tigger updates
			// Timestamp: metav1.Timestamp{
			// 	Seconds: time.Now().Unix(),
			// },
			Category: annotations[categoryLabel],
			Severity: severityFromString(annotations[categoryLabel]),
		}
		if result.Result == "fail" && !result.Scored {
			result.Result = "warn"
		}
		results = append(results, result)
	}
	return results
}

func isPolicyLabel(label string) bool {
	return strings.HasPrefix(label, "pol.kyverno.io/") || strings.HasPrefix(label, "cpol.kyverno.io/")
}

func policyNameFromLabel(namespace, label string) (string, error) {
	names := strings.Split(label, "/")
	if len(names) == 2 {
		if names[0] == "cpol.kyverno.io" {
			return names[1], nil
		} else if names[0] == "pol.kyverno.io" {
			return namespace + "/" + names[1], nil
		}
	}
	return "", fmt.Errorf("cannot get policy name from label, incorrect format: %s", label)
}

func policyLabelPrefix(policy kyvernov1.PolicyInterface) string {
	if policy.IsNamespaced() {
		return "pol.kyverno.io"
	}
	return "cpol.kyverno.io"
}

func policyLabel(policy kyvernov1.PolicyInterface) string {
	return policyLabelPrefix(policy) + "/" + policy.GetName()
}

func policyLabelRequirementNotEquals(policy kyvernov1.PolicyInterface) (*labels.Requirement, error) {
	return labels.NewRequirement(policyLabel(policy), selection.NotEquals, []string{policy.GetResourceVersion()})
}

func policyLabelRequirementExists(policy kyvernov1.PolicyInterface) (*labels.Requirement, error) {
	return labels.NewRequirement(policyLabel(policy), selection.Exists, nil)
}

func policyLabelRequirementDoesNotExist(policy kyvernov1.PolicyInterface) (*labels.Requirement, error) {
	return labels.NewRequirement(policyLabel(policy), selection.DoesNotExist, nil)
}
