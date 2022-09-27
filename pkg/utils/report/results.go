package report

import (
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

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
		return a.Timestamp.String() < b.Timestamp.String()
	})
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

func EngineResponseToReportResults(response *response.EngineResponse) []policyreportv1alpha2.PolicyReportResult {
	key, _ := cache.MetaNamespaceKeyFunc(response.Policy)
	var results []policyreportv1alpha2.PolicyReportResult
	for _, ruleResult := range response.PolicyResponse.Rules {
		annotations := response.Policy.GetAnnotations()
		result := policyreportv1alpha2.PolicyReportResult{
			Source:  kyvernov1.ValueKyvernoApp,
			Policy:  key,
			Rule:    ruleResult.Name,
			Message: ruleResult.Message,
			Result:  toPolicyResult(ruleResult.Status),
			Scored:  annotations[kyvernov1.AnnotationPolicyScored] != "false",
			Timestamp: metav1.Timestamp{
				Seconds: time.Now().Unix(),
			},
			Category: annotations[kyvernov1.AnnotationPolicyCategory],
			Severity: severityFromString(annotations[kyvernov1.AnnotationPolicySeverity]),
		}
		if result.Result == "fail" && !result.Scored {
			result.Result = "warn"
		}
		results = append(results, result)
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

func SetResponses(report kyvernov1alpha2.ReportInterface, engineResponses ...*response.EngineResponse) {
	var ruleResults []policyreportv1alpha2.PolicyReportResult
	for _, result := range engineResponses {
		SetPolicyLabel(report, result.Policy)
		ruleResults = append(ruleResults, EngineResponseToReportResults(result)...)
	}
	SetResults(report, ruleResults...)
}
