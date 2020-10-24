package policyreport

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/engine/response"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	clusterreportrequest = "clusterreportrequest"
)

//GeneratePRsFromEngineResponse generate Violations from engine responses
func GeneratePRsFromEngineResponse(ers []response.EngineResponse, log logr.Logger) (pvInfos []Info) {
	for _, er := range ers {
		// ignore creation of PV for resources that are yet to be assigned a name
		if er.PolicyResponse.Resource.Name == "" {
			log.V(4).Info("resource does no have a name assigned yet, not creating a policy violation", "resource", er.PolicyResponse.Resource)
			continue
		}
		// skip when response succeed
		if os.Getenv("POLICY-TYPE") != common.PolicyReport {
			if er.IsSuccessful() {
				continue
			}
		}
		// build policy violation info
		pvInfos = append(pvInfos, buildPVInfo(er))
	}

	return pvInfos
}

// Builder builds report request struct
// this is base type of namespaced and cluster policy report
type Builder interface {
	build(info Info) (*unstructured.Unstructured, error)
}

type requestBuilder struct{}

func NewBuilder() *requestBuilder {
	return &requestBuilder{}
}
func (pvb *requestBuilder) build(info Info) (*unstructured.Unstructured, error) {
	results := []*report.PolicyReportResult{}
	for _, rule := range info.Rules {
		result := &report.PolicyReportResult{
			Policy: info.PolicyName,
			Resources: []*v1.ObjectReference{
				{
					Kind:       info.Resource.GetKind(),
					Namespace:  info.Resource.GetNamespace(),
					APIVersion: info.Resource.GetAPIVersion(),
					Name:       info.Resource.GetName(),
					UID:        info.Resource.GetUID(),
				},
			},
			Scored: true,
		}

		result.Rule = rule.Name
		result.Message = rule.Message
		result.Status = report.PolicyStatus(rule.Check)
		results = append(results, result)
	}

	ns := info.Resource.GetNamespace()
	if ns != "" {
		rr := &report.ReportRequest{
			Summary: calculateSummary(results),
			Results: results,
		}

		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(rr)
		if err != nil {
			return nil, err
		}

		req := &unstructured.Unstructured{Object: obj}
		kind, apiversion := rr.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
		set(req, kind, apiversion, fmt.Sprintf("reportrequest-%s-%s", info.PolicyName, info.Resource.GetName()), info)
		return req, nil
	}

	rr := &report.ClusterPolicyReport{
		Summary: calculateSummary(results),
		Results: results,
	}

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(rr)
	if err != nil {
		return nil, err
	}
	req := &unstructured.Unstructured{Object: obj}
	kind, apiversion := rr.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	set(req, kind, apiversion, fmt.Sprintf("%s-%s", clusterreportrequest, info.Resource.GetName()), info)
	return req, nil
}

func set(obj *unstructured.Unstructured, kind, apiversion, name string, info Info) {
	resource := info.Resource
	obj.SetName(name)
	obj.SetNamespace(resource.GetNamespace())
	obj.SetKind(kind)
	obj.SetAPIVersion(apiversion)

	obj.SetLabels(map[string]string{
		"policy":   info.PolicyName,
		"resource": resource.GetKind() + "-" + resource.GetName(),
	})

	if info.FromSync {
		obj.SetAnnotations(map[string]string{
			"fromSync": "true",
		})
	}

	controllerFlag := true
	blockOwnerDeletionFlag := true
	obj.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion:         resource.GetAPIVersion(),
			Kind:               resource.GetKind(),
			Name:               resource.GetName(),
			UID:                resource.GetUID(),
			Controller:         &controllerFlag,
			BlockOwnerDeletion: &blockOwnerDeletionFlag,
		},
	})
}

func calculateSummary(results []*report.PolicyReportResult) (summary report.PolicyReportSummary) {
	for _, res := range results {
		switch string(res.Status) {
		case report.StatusPass:
			summary.Pass++
		case report.StatusFail:
			summary.Fail++
		case "warn":
			summary.Warn++
		case "error":
			summary.Error++
		case "skip":
			summary.Skip++
		}
	}
	return
}

func buildPVInfo(er response.EngineResponse) Info {
	info := Info{
		PolicyName: er.PolicyResponse.Policy,
		Resource:   er.PatchedResource,
		Rules:      buildViolatedRules(er),
	}
	return info
}

func buildViolatedRules(er response.EngineResponse) []kyverno.ViolatedRule {
	var violatedRules []kyverno.ViolatedRule
	for _, rule := range er.PolicyResponse.Rules {
		if os.Getenv("POLICY-TYPE") != common.PolicyReport {
			if rule.Success {
				continue
			}
		}
		vrule := kyverno.ViolatedRule{
			Name:    rule.Name,
			Type:    rule.Type,
			Message: rule.Message,
		}
		vrule.Check = report.StatusFail
		if rule.Success {
			vrule.Check = report.StatusPass
		}
		violatedRules = append(violatedRules, vrule)
	}
	return violatedRules
}
