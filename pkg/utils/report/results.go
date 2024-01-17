package report

import (
	"cmp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func SortReportResults(results []policyreportv1alpha2.PolicyReportResult) {
	slices.SortFunc(results, func(a policyreportv1alpha2.PolicyReportResult, b policyreportv1alpha2.PolicyReportResult) int {
		if x := cmp.Compare(a.Policy, b.Policy); x != 0 {
			return x
		}
		if x := cmp.Compare(a.Rule, b.Rule); x != 0 {
			return x
		}
		if x := cmp.Compare(len(a.Resources), len(b.Resources)); x != 0 {
			return x
		}
		for i := range a.Resources {
			if x := cmp.Compare(a.Resources[i].UID, b.Resources[i].UID); x != 0 {
				return x
			}
		}
		return cmp.Compare(a.Timestamp.String(), b.Timestamp.String())
	})
}

func CalculateSummary(results []policyreportv1alpha2.PolicyReportResult) (summary policyreportv1alpha2.PolicyReportSummary) {
	for _, res := range results {
		switch res.Result {
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

func toPolicyResult(status engineapi.RuleStatus) policyreportv1alpha2.PolicyResult {
	switch status {
	case engineapi.RuleStatusPass:
		return policyreportv1alpha2.StatusPass
	case engineapi.RuleStatusFail:
		return policyreportv1alpha2.StatusFail
	case engineapi.RuleStatusError:
		return policyreportv1alpha2.StatusError
	case engineapi.RuleStatusWarn:
		return policyreportv1alpha2.StatusWarn
	case engineapi.RuleStatusSkip:
		return policyreportv1alpha2.StatusSkip
	}
	return ""
}

func SeverityFromString(severity string) policyreportv1alpha2.PolicySeverity {
	switch severity {
	case "critical":
		return policyreportv1alpha2.SeverityCritical
	case "high":
		return policyreportv1alpha2.SeverityHigh
	case "medium":
		return policyreportv1alpha2.SeverityMedium
	case "low":
		return policyreportv1alpha2.SeverityLow
	case "info":
		return policyreportv1alpha2.SeverityInfo
	}
	return ""
}

func EngineResponseToReportResults(response engineapi.EngineResponse) []policyreportv1alpha2.PolicyReportResult {
	pol := response.Policy()
	var results []policyreportv1alpha2.PolicyReportResult
	if pol.GetType() == engineapi.KyvernoPolicyType {
		key, _ := cache.MetaNamespaceKeyFunc(pol.GetPolicy().(kyvernov1.PolicyInterface))
		for _, ruleResult := range response.PolicyResponse.Rules {
			annotations := pol.GetAnnotations()
			result := policyreportv1alpha2.PolicyReportResult{
				Source:  kyverno.ValueKyvernoApp,
				Policy:  key,
				Rule:    ruleResult.Name(),
				Message: ruleResult.Message(),
				Result:  toPolicyResult(ruleResult.Status()),
				Scored:  annotations[kyverno.AnnotationPolicyScored] != "false",
				Timestamp: metav1.Timestamp{
					Seconds: time.Now().Unix(),
				},
				Category: annotations[kyverno.AnnotationPolicyCategory],
				Severity: SeverityFromString(annotations[kyverno.AnnotationPolicySeverity]),
			}
			pss := ruleResult.PodSecurityChecks()
			if pss != nil {
				var controls []string
				for _, check := range pss.Checks {
					if !check.CheckResult.Allowed {
						controls = append(controls, check.ID)
					}
				}
				if len(controls) > 0 {
					sort.Strings(controls)
					result.Properties = map[string]string{
						"standard": string(pss.Level),
						"version":  pss.Version,
						"controls": strings.Join(controls, ","),
					}
				}
			}
			if result.Result == "fail" && !result.Scored {
				result.Result = "warn"
			}
			results = append(results, result)
		}
	} else {
		for _, ruleResult := range response.PolicyResponse.Rules {
			result := policyreportv1alpha2.PolicyReportResult{
				Source:  "ValidatingAdmissionPolicy",
				Policy:  ruleResult.Name(),
				Message: ruleResult.Message(),
				Result:  toPolicyResult(ruleResult.Status()),
				Timestamp: metav1.Timestamp{
					Seconds: time.Now().Unix(),
				},
			}
			results = append(results, result)
		}
	}
	return results
}

func SplitResultsByPolicy(logger logr.Logger, results []policyreportv1alpha2.PolicyReportResult) map[string][]policyreportv1alpha2.PolicyReportResult {
	resultsMap := map[string][]policyreportv1alpha2.PolicyReportResult{}
	keysMap := map[string]string{}
	for _, result := range results {
		if keysMap[result.Policy] == "" {
			ns, n, err := cache.SplitMetaNamespaceKey(result.Policy)
			if err != nil {
				logger.Error(err, "failed to decode policy name", "key", result.Policy)
			} else {
				if ns == "" {
					keysMap[result.Policy] = "cpol-" + n
				} else {
					keysMap[result.Policy] = "pol-" + n
				}
			}
		}
	}
	for _, result := range results {
		key := keysMap[result.Policy]
		resultsMap[key] = append(resultsMap[key], result)
	}
	return resultsMap
}

func SetResults(report kyvernov1alpha2.ReportInterface, results ...policyreportv1alpha2.PolicyReportResult) {
	SortReportResults(results)
	report.SetResults(results)
	report.SetSummary(CalculateSummary(results))
}

func SetResponses(report kyvernov1alpha2.ReportInterface, engineResponses ...engineapi.EngineResponse) {
	var ruleResults []policyreportv1alpha2.PolicyReportResult
	for _, result := range engineResponses {
		pol := result.Policy()
		SetPolicyLabel(report, pol)
		ruleResults = append(ruleResults, EngineResponseToReportResults(result)...)
	}
	SetResults(report, ruleResults...)
}
