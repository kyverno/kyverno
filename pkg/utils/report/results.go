package report

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/pss/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func ToPolicyReportResult(policyType engineapi.PolicyType, policyName string, ruleResult engineapi.RuleResponse, annotations map[string]string, resource *corev1.ObjectReference) policyreportv1alpha2.PolicyReportResult {
	result := policyreportv1alpha2.PolicyReportResult{
		Source:     kyverno.ValueKyvernoApp,
		Policy:     policyName,
		Rule:       ruleResult.Name(),
		Message:    ruleResult.Message(),
		Properties: ruleResult.Properties(),
		Result:     toPolicyResult(ruleResult.Status()),
		Scored:     annotations[kyverno.AnnotationPolicyScored] != "false",
		Timestamp: metav1.Timestamp{
			Seconds: time.Now().Unix(),
		},
		Category: annotations[kyverno.AnnotationPolicyCategory],
		Severity: SeverityFromString(annotations[kyverno.AnnotationPolicySeverity]),
	}
	if result.Result == "fail" && !result.Scored {
		result.Result = "warn"
	}
	if resource != nil {
		result.Resources = []corev1.ObjectReference{
			*resource,
		}
	}
	exceptions := ruleResult.Exceptions()
	if len(exceptions) > 0 {
		var names []string
		for _, exception := range exceptions {
			names = append(names, exception.Name)
		}
		addProperty("exceptions", strings.Join(names, ","), &result)
	}
	pss := ruleResult.PodSecurityChecks()
	if pss != nil && len(pss.Checks) > 0 {
		addPodSecurityProperties(pss, &result)
	}
	if policyType == engineapi.ValidatingAdmissionPolicyType {
		result.Source = "ValidatingAdmissionPolicy"
		result.Policy = ruleResult.Name()
		if ruleResult.ValidatingAdmissionPolicyBinding() != nil {
			addProperty("binding", ruleResult.ValidatingAdmissionPolicyBinding().Name, &result)
		}
	}
	return result
}

func addProperty(k, v string, result *policyreportv1alpha2.PolicyReportResult) {
	if result.Properties == nil {
		result.Properties = map[string]string{}
	}

	result.Properties[k] = v
}

type Control struct {
	ID     string
	Name   string
	Images []string
}

func addPodSecurityProperties(pss *engineapi.PodSecurityChecks, result *policyreportv1alpha2.PolicyReportResult) {
	if pss == nil {
		return
	}
	if result.Properties == nil {
		result.Properties = map[string]string{}
	}
	var controls []Control
	var controlIDs []string
	for _, check := range pss.Checks {
		if !check.CheckResult.Allowed {
			controlName := utils.PSSControlIDToName(check.ID)
			controlIDs = append(controlIDs, check.ID)
			controls = append(controls, Control{
				ID:     check.ID,
				Name:   controlName,
				Images: check.Images,
			})
		}
	}
	if len(controls) > 0 {
		controlsJson, _ := json.Marshal(controls)
		result.Properties["standard"] = string(pss.Level)
		result.Properties["version"] = pss.Version
		result.Properties["controls"] = strings.Join(controlIDs, ",")
		result.Properties["controlsJSON"] = string(controlsJson)
	}
}

func EngineResponseToReportResults(response engineapi.EngineResponse) []policyreportv1alpha2.PolicyReportResult {
	pol := response.Policy()
	policyName, _ := cache.MetaNamespaceKeyFunc(pol.AsKyvernoPolicy())
	policyType := pol.GetType()
	annotations := pol.GetAnnotations()

	results := make([]policyreportv1alpha2.PolicyReportResult, 0, len(response.PolicyResponse.Rules))
	for _, ruleResult := range response.PolicyResponse.Rules {
		result := ToPolicyReportResult(policyType, policyName, ruleResult, annotations, nil)
		results = append(results, result)
	}

	return results
}

func MutationEngineResponseToReportResults(response engineapi.EngineResponse) []policyreportv1alpha2.PolicyReportResult {
	pol := response.Policy()
	policyName, _ := cache.MetaNamespaceKeyFunc(pol.AsKyvernoPolicy())
	policyType := pol.GetType()
	annotations := pol.GetAnnotations()

	results := make([]policyreportv1alpha2.PolicyReportResult, 0, len(response.PolicyResponse.Rules))
	for _, ruleResult := range response.PolicyResponse.Rules {
		result := ToPolicyReportResult(policyType, policyName, ruleResult, annotations, nil)
		if target, _, _ := ruleResult.PatchedTarget(); target != nil {
			addProperty("patched-target", getResourceInfo(target.GroupVersionKind(), target.GetName(), target.GetNamespace()), &result)
		}
		results = append(results, result)
	}

	return results
}

func GenerationEngineResponseToReportResults(response engineapi.EngineResponse) []policyreportv1alpha2.PolicyReportResult {
	pol := response.Policy()
	policyName, _ := cache.MetaNamespaceKeyFunc(pol.AsKyvernoPolicy())
	policyType := pol.GetType()
	annotations := pol.GetAnnotations()

	results := make([]policyreportv1alpha2.PolicyReportResult, 0, len(response.PolicyResponse.Rules))
	for _, ruleResult := range response.PolicyResponse.Rules {
		result := ToPolicyReportResult(policyType, policyName, ruleResult, annotations, nil)
		if generatedResources := ruleResult.GeneratedResources(); len(generatedResources) != 0 {
			property := make([]string, 0)
			for _, r := range generatedResources {
				property = append(property, getResourceInfo(r.GroupVersionKind(), r.GetName(), r.GetNamespace()))
			}
			addProperty("generated-resources", strings.Join(property, "; "), &result)
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

func SetResults(report reportsv1.ReportInterface, results ...policyreportv1alpha2.PolicyReportResult) {
	SortReportResults(results)
	report.SetResults(results)
	report.SetSummary(CalculateSummary(results))
}

func SetResponses(report reportsv1.ReportInterface, engineResponses ...engineapi.EngineResponse) {
	var ruleResults []policyreportv1alpha2.PolicyReportResult
	for _, result := range engineResponses {
		pol := result.Policy()
		SetPolicyLabel(report, pol)
		ruleResults = append(ruleResults, EngineResponseToReportResults(result)...)
	}
	SetResults(report, ruleResults...)
}

func SetMutationResponses(report reportsv1.ReportInterface, engineResponses ...engineapi.EngineResponse) {
	var ruleResults []policyreportv1alpha2.PolicyReportResult
	for _, result := range engineResponses {
		pol := result.Policy()
		SetPolicyLabel(report, pol)
		ruleResults = append(ruleResults, MutationEngineResponseToReportResults(result)...)
	}
	SetResults(report, ruleResults...)
}

func SetGenerationResponses(report reportsv1.ReportInterface, engineResponses ...engineapi.EngineResponse) {
	var ruleResults []policyreportv1alpha2.PolicyReportResult
	for _, result := range engineResponses {
		pol := result.Policy()
		SetPolicyLabel(report, pol)
		ruleResults = append(ruleResults, GenerationEngineResponseToReportResults(result)...)
	}
	SetResults(report, ruleResults...)
}

func getResourceInfo(gvk schema.GroupVersionKind, name, namespace string) string {
	info := gvk.String() + " Name=" + name
	if len(namespace) != 0 {
		info = info + " Namespace=" + namespace
	}
	return info
}
