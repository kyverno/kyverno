package report

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/openreports"
	"github.com/kyverno/kyverno/pkg/pss/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"
)

func SortReportResults(results []openreportsv1alpha1.ReportResult) {
	slices.SortFunc(results, func(a openreportsv1alpha1.ReportResult, b openreportsv1alpha1.ReportResult) int {
		if x := cmp.Compare(a.Policy, b.Policy); x != 0 {
			return x
		}
		if x := cmp.Compare(a.Rule, b.Rule); x != 0 {
			return x
		}
		if x := cmp.Compare(a.Source, b.Source); x != 0 {
			return x
		}
		if x := cmp.Compare(len(a.Subjects), len(b.Subjects)); x != 0 {
			return x
		}
		for i := range a.Subjects {
			if x := cmp.Compare(a.Subjects[i].UID, b.Subjects[i].UID); x != 0 {
				return x
			}
		}
		return cmp.Compare(a.Timestamp.String(), b.Timestamp.String())
	})
}

func CalculateSummary(results []openreportsv1alpha1.ReportResult) (summary openreportsv1alpha1.ReportSummary) {
	for _, res := range results {
		switch res.Result {
		case openreports.StatusPass:
			summary.Pass++
		case openreports.StatusFail:
			summary.Fail++
		case openreports.StatusWarn:
			summary.Warn++
		case openreports.StatusError:
			summary.Error++
		case openreports.StatusSkip:
			summary.Skip++
		}
	}
	return
}

func toPolicyResult(status engineapi.RuleStatus) openreportsv1alpha1.Result {
	switch status {
	case engineapi.RuleStatusPass:
		return openreports.StatusPass
	case engineapi.RuleStatusFail:
		return openreports.StatusFail
	case engineapi.RuleStatusError:
		return openreports.StatusError
	case engineapi.RuleStatusWarn:
		return openreports.StatusWarn
	case engineapi.RuleStatusSkip:
		return openreports.StatusSkip
	}
	return ""
}

func SeverityFromString(severity string) openreportsv1alpha1.ResultSeverity {
	switch severity {
	case "critical":
		return openreports.SeverityCritical
	case "high":
		return openreports.SeverityHigh
	case "medium":
		return openreports.SeverityMedium
	case "low":
		return openreports.SeverityLow
	case "info":
		return openreports.SeverityInfo
	}
	return ""
}

func ToPolicyReportResult(pol engineapi.GenericPolicy, ruleResult engineapi.RuleResponse, resource *corev1.ObjectReference) openreportsv1alpha1.ReportResult {
	policyName, _ := cache.MetaNamespaceKeyFunc(pol)
	annotations := pol.GetAnnotations()

	result := openreportsv1alpha1.ReportResult{
		Source:      SourceKyverno,
		Policy:      policyName,
		Rule:        ruleResult.Name(),
		Description: ruleResult.Message(),
		Properties:  ruleResult.Properties(),
		Result:      toPolicyResult(ruleResult.Status()),
		Scored:      annotations[kyverno.AnnotationPolicyScored] != "false",
		Timestamp: metav1.Timestamp{
			Seconds: time.Now().Unix(),
		},
		Category: annotations[kyverno.AnnotationPolicyCategory],
		Severity: SeverityFromString(annotations[kyverno.AnnotationPolicySeverity]),
	}

	var process string

	switch {
	case pol.AsValidatingAdmissionPolicy() != nil:
		result.Source = SourceValidatingAdmissionPolicy
		result.Policy = ruleResult.Name()
		process = "background scan"
		if binding := ruleResult.ValidatingAdmissionPolicyBinding(); binding != nil {
			addProperty("binding", binding.Name, &result)
		}

	case pol.AsMutatingAdmissionPolicy() != nil:
		result.Source = SourceMutatingAdmissionPolicy
		result.Policy = ruleResult.Name()
		process = "background scan"
		if binding := ruleResult.MutatingAdmissionPolicyBinding(); binding != nil {
			addProperty("mapBinding", binding.Name, &result)
		}

	case pol.AsValidatingPolicy() != nil:
		vp := pol.AsValidatingPolicy()
		result.Source = SourceValidatingPolicy
		process = selectProcess(vp.Spec.BackgroundEnabled(), vp.Spec.AdmissionEnabled())

	case pol.AsMutatingPolicy() != nil:
		mpol := pol.AsMutatingPolicy()
		result.Source = SourceMutatingPolicy
		process = selectProcess(mpol.Spec.BackgroundEnabled(), mpol.Spec.AdmissionEnabled())

	case pol.AsImageValidatingPolicy() != nil:
		ivp := pol.AsImageValidatingPolicy()
		result.Source = SourceImageValidatingPolicy
		process = selectProcess(ivp.Spec.BackgroundEnabled(), ivp.Spec.AdmissionEnabled())

	case pol.AsGeneratingPolicy() != nil:
		result.Source = SourceGeneratingPolicy
		process = "admission review"

	case pol.AsKyvernoPolicy() != nil:
		kyvernoPolicy := pol.AsKyvernoPolicy()
		result.Source = SourceKyverno
		process = selectProcess(kyvernoPolicy.BackgroundProcessingEnabled(), kyvernoPolicy.AdmissionProcessingEnabled())
	}
	addProperty("process", process, &result)

	if result.Result == "fail" && !result.Scored {
		result.Result = "warn"
	}

	if resource != nil {
		result.Subjects = []corev1.ObjectReference{*resource}
	}

	if exceptions := ruleResult.Exceptions(); len(exceptions) > 0 {
		var names []string
		for _, e := range exceptions {
			names = append(names, e.GetName())
		}
		addProperty("exceptions", strings.Join(names, ","), &result)
	}

	if pss := ruleResult.PodSecurityChecks(); pss != nil && len(pss.Checks) > 0 {
		addPodSecurityProperties(pss, &result)
	}
	return result
}

func addProperty(k, v string, result *openreportsv1alpha1.ReportResult) {
	if result.Properties == nil {
		result.Properties = map[string]string{}
	}

	result.Properties[k] = v
}

func selectProcess(background, admission bool) string {
	switch {
	case background:
		return "background scan"
	case admission:
		return "admission review"
	default:
		return ""
	}
}

type Control struct {
	ID     string
	Name   string
	Images []string
}

func addPodSecurityProperties(pss *engineapi.PodSecurityChecks, result *openreportsv1alpha1.ReportResult) {
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

func EngineResponseToReportResults(response engineapi.EngineResponse) []openreportsv1alpha1.ReportResult {
	results := make([]openreportsv1alpha1.ReportResult, 0, len(response.PolicyResponse.Rules))
	for _, ruleResult := range response.PolicyResponse.Rules {
		result := ToPolicyReportResult(response.Policy(), ruleResult, nil)
		results = append(results, result)
	}

	return results
}

func MutationEngineResponseToReportResults(response engineapi.EngineResponse) []openreportsv1alpha1.ReportResult {
	results := make([]openreportsv1alpha1.ReportResult, 0, len(response.PolicyResponse.Rules))
	for _, ruleResult := range response.PolicyResponse.Rules {
		result := ToPolicyReportResult(response.Policy(), ruleResult, nil)
		if target, _, _ := ruleResult.PatchedTarget(); target != nil {
			addProperty("patched-target", getResourceInfo(target.GroupVersionKind(), target.GetName(), target.GetNamespace()), &result)
		}
		results = append(results, result)
	}

	return results
}

func GenerationEngineResponseToReportResults(response engineapi.EngineResponse) []openreportsv1alpha1.ReportResult {
	results := make([]openreportsv1alpha1.ReportResult, 0, len(response.PolicyResponse.Rules))
	for _, ruleResult := range response.PolicyResponse.Rules {
		result := ToPolicyReportResult(response.Policy(), ruleResult, nil)
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

func SplitResultsByPolicy(logger logr.Logger, results []openreportsv1alpha1.ReportResult) map[string][]openreportsv1alpha1.ReportResult {
	resultsMap := map[string][]openreportsv1alpha1.ReportResult{}
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

func SetResults(report reportsv1.ReportInterface, results ...openreportsv1alpha1.ReportResult) {
	SortReportResults(results)
	report.SetResults(results)
	report.SetSummary(CalculateSummary(results))
}

func SetResponses(report reportsv1.ReportInterface, engineResponses ...engineapi.EngineResponse) {
	var ruleResults []openreportsv1alpha1.ReportResult
	for _, result := range engineResponses {
		pol := result.Policy()
		SetPolicyLabel(report, pol)
		ruleResults = append(ruleResults, EngineResponseToReportResults(result)...)
	}
	SetResults(report, ruleResults...)
}

func SetMutationResponses(report reportsv1.ReportInterface, engineResponses ...engineapi.EngineResponse) {
	var ruleResults []openreportsv1alpha1.ReportResult
	for _, result := range engineResponses {
		pol := result.Policy()
		SetPolicyLabel(report, pol)
		ruleResults = append(ruleResults, MutationEngineResponseToReportResults(result)...)
	}
	SetResults(report, ruleResults...)
}

func SetGenerationResponses(report reportsv1.ReportInterface, engineResponses ...engineapi.EngineResponse) {
	var ruleResults []openreportsv1alpha1.ReportResult
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
