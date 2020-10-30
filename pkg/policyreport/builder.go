package policyreport

import (
	"fmt"
	"os"
	"reflect"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	clusterreportchangerequest string = "clusterreportchangerequest"
	deletedLabelResource       string = "kyverno.io/delete.resource"
	deletedLabelResourceKind   string = "kyverno.io/delete.resource.kind"
	deletedLabelPolicy         string = "kyverno.io/delete.policy"
	deletedLabelRule           string = "kyverno.io/delete.rule"
)

func generatePolicyReportName(ns string) string {
	if ns == "" {
		return clusterpolicyreport
	}
	return fmt.Sprintf("policyreport-ns-%s", ns)
}

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

// Builder builds report change request struct
// this is base type of namespaced and cluster policy report
type Builder interface {
	build(info Info) (*unstructured.Unstructured, error)
}

type requestBuilder struct{}

func NewBuilder() *requestBuilder {
	return &requestBuilder{}
}
func (pvb *requestBuilder) build(info Info) (req *unstructured.Unstructured, err error) {
	results := []*report.PolicyReportResult{}
	for _, rule := range info.Rules {
		if rule.Type != utils.Validation.String() {
			continue
		}

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

	if info.Resource.GetNamespace() != "" {
		rr := &report.ReportChangeRequest{
			Summary: calculateSummary(results),
			Results: results,
		}

		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(rr)
		if err != nil {
			return nil, err
		}

		req = &unstructured.Unstructured{Object: obj}
		set(req, fmt.Sprintf("reportchangerequest-%s-%s-%s", info.PolicyName, info.Resource.GetNamespace(), info.Resource.GetName()), info)
	} else {
		rr := &report.ClusterReportChangeRequest{
			Summary: calculateSummary(results),
			Results: results,
		}

		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(rr)
		if err != nil {
			return nil, err
		}
		req = &unstructured.Unstructured{Object: obj}
		set(req, fmt.Sprintf("%s-%s", clusterreportchangerequest, info.Resource.GetName()), info)
	}

	// deletion of a result entry
	// - on resource deleteion:
	//   - info.Rules == 0 && info.PolicyName == ""
	//   - set label delete.resource=resourceKind-resourceNamespace-resourceName
	// - on policy deleteion:
	//   - info.PolicyName != "" && info.Resource == {}
	//   - set label delete.policy=policyName
	if len(info.Rules) == 0 && info.PolicyName == "" {
		req.SetLabels(map[string]string{
			"namespace":              info.Resource.GetNamespace(),
			deletedLabelResource:     info.Resource.GetName(),
			deletedLabelResourceKind: info.Resource.GetKind()})
	} else if info.PolicyName != "" && reflect.DeepEqual(info.Resource, unstructured.Unstructured{}) {
		req.SetKind("ReportChangeRequest")

		if len(info.Rules) == 0 {
			req.SetLabels(map[string]string{
				deletedLabelPolicy: info.PolicyName})

			req.SetName(fmt.Sprintf("reportchangerequest-%s", info.PolicyName))
		} else {
			req.SetLabels(map[string]string{
				deletedLabelPolicy: info.PolicyName,
				deletedLabelRule:   info.Rules[0].Name})
			req.SetName(fmt.Sprintf("reportchangerequest-%s-%s", info.PolicyName, info.Rules[0].Name))
		}
	} else if len(results) == 0 {
		// return nil on empty result without a deletion
		return nil, nil
	}

	return req, nil
}

func set(obj *unstructured.Unstructured, name string, info Info) {
	resource := info.Resource
	obj.SetName(name)
	obj.SetNamespace(config.KubePolicyNamespace)
	obj.SetAPIVersion("policy.kubernetes.io/v1alpha1")
	if resource.GetNamespace() == "" {
		obj.SetKind("ClusterReportChangeRequest")
	} else {
		obj.SetKind("ReportChangeRequest")
	}

	obj.SetLabels(map[string]string{
		"namespace": resource.GetNamespace(),
		"policy":    info.PolicyName,
		"resource":  resource.GetKind() + "-" + resource.GetNamespace() + "-" + resource.GetName(),
	})

	if info.FromSync {
		obj.SetAnnotations(map[string]string{
			"fromSync": "true",
		})
	}
}

func calculateSummary(results []*report.PolicyReportResult) (summary report.PolicyReportSummary) {
	for _, res := range results {
		switch string(res.Status) {
		case report.StatusPass:
			summary.Pass++
		case report.StatusFail:
			summary.Fail++
		case report.StatusWarn:
			summary.Warn++
		case report.StatusError:
			summary.Error++
		case report.StatusSkip:
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
