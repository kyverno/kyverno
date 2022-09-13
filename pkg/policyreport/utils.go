package policyreport

import (
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/toggle"
)

func GeneratePolicyReportName(ns, policyName string) string {
	if ns == "" {
		if toggle.SplitPolicyReport.Enabled() {
			return trimmedName(clusterpolicyreport + "-" + policyName)
		}
		return clusterpolicyreport
	}
	var name string
	if toggle.SplitPolicyReport.Enabled() {
		name = fmt.Sprintf("polr-ns-%s-%s", ns, policyName)
	} else {
		name = fmt.Sprintf("polr-ns-%s", ns)
	}
	return trimmedName(name)
}

func trimmedName(s string) string {
	if len(s) > 63 {
		return s[:63]
	}
	return s
}

// GeneratePRsFromEngineResponse generate Violations from engine responses
func GeneratePRsFromEngineResponse(ers []*response.EngineResponse, log logr.Logger) (pvInfos []Info) {
	for _, er := range ers {
		// ignore creation of PV for resources that are yet to be assigned a name
		if er.PolicyResponse.Resource.Name == "" {
			log.V(4).Info("skipping resource with no name", "resource", er.PolicyResponse.Resource)
			continue
		}

		if len(er.PolicyResponse.Rules) == 0 {
			continue
		}

		if er.Policy != nil && engine.ManagedPodResource(er.Policy, er.PatchedResource) {
			continue
		}

		// build policy violation info
		pvInfos = append(pvInfos, buildPVInfo(er))
	}

	return pvInfos
}

func buildPVInfo(er *response.EngineResponse) Info {
	info := Info{
		PolicyName: er.PolicyResponse.Policy.Name,
		Namespace:  er.PatchedResource.GetNamespace(),
		Results: []EngineResponseResult{
			{
				Resource: er.GetResourceSpec(),
				Rules:    buildViolatedRules(er),
			},
		},
	}
	return info
}

func buildViolatedRules(er *response.EngineResponse) []kyvernov1.ViolatedRule {
	var violatedRules []kyvernov1.ViolatedRule
	for _, rule := range er.PolicyResponse.Rules {
		vrule := kyvernov1.ViolatedRule{
			Name:    rule.Name,
			Type:    string(rule.Type),
			Message: rule.Message,
		}

		vrule.Status = toPolicyResult(rule.Status)
		violatedRules = append(violatedRules, vrule)
	}

	return violatedRules
}

func calculateSummary(results []policyreportv1alpha2.PolicyReportResult) (summary policyreportv1alpha2.PolicyReportSummary) {
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

func toPolicyResult(status response.RuleStatus) string {
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
