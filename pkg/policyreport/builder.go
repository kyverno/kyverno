package policyreport

import (
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	request "github.com/kyverno/kyverno/pkg/api/kyverno/v1alpha1"
	report "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// the following labels are used to list rcr / crcr
	resourceLabelNamespace string = "kyverno.io/resource.namespace"
	deletedLabelPolicy     string = "kyverno.io/delete.policy"
	deletedLabelRule       string = "kyverno.io/delete.rule"

	// the following annotations are used to remove entries from polr / cpolr
	// there would be a problem if use labels as the value could exceed 63 chars
	deletedAnnotationResourceName string = "kyverno.io/delete.resource.name"
	deletedAnnotationResourceKind string = "kyverno.io/delete.resource.kind"
)

func generatePolicyReportName(ns string) string {
	if ns == "" {
		return clusterpolicyreport
	}

	name := fmt.Sprintf("polr-ns-%s", ns)
	if len(name) > 63 {
		return name[:63]
	}

	return name
}

//GeneratePRsFromEngineResponse generate Violations from engine responses
func GeneratePRsFromEngineResponse(ers []*response.EngineResponse, log logr.Logger) (pvInfos []Info) {
	for _, er := range ers {
		// ignore creation of PV for resources that are yet to be assigned a name
		if er.PolicyResponse.Resource.Name == "" {
			log.V(4).Info("resource does no have a name assigned yet, not creating a policy violation", "resource", er.PolicyResponse.Resource)
			continue
		}

		if len(er.PolicyResponse.Rules) == 0 {
			continue
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

type requestBuilder struct {
	cpolLister kyvernolister.ClusterPolicyLister
	polLister  kyvernolister.PolicyLister
}

// NewBuilder ...
func NewBuilder(cpolLister kyvernolister.ClusterPolicyLister, polLister kyvernolister.PolicyLister) Builder {
	return &requestBuilder{cpolLister: cpolLister, polLister: polLister}
}

func (builder *requestBuilder) build(info Info) (req *unstructured.Unstructured, err error) {
	results := []*report.PolicyReportResult{}
	for _, infoResult := range info.Results {
		for _, rule := range infoResult.Rules {
			if rule.Type != utils.Validation.String() {
				continue
			}

			result := builder.buildRCRResult(info.PolicyName, infoResult.Resource, rule)
			results = append(results, result)
		}
	}

	if info.Namespace != "" {
		rr := &request.ReportChangeRequest{
			Summary: calculateSummary(results),
			Results: results,
		}

		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(rr)
		if err != nil {
			return nil, err
		}

		req = &unstructured.Unstructured{Object: obj}
		set(req, info)
	} else {
		rr := &request.ClusterReportChangeRequest{
			Summary: calculateSummary(results),
			Results: results,
		}

		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(rr)
		if err != nil {
			return nil, err
		}
		req = &unstructured.Unstructured{Object: obj}
		set(req, info)
	}

	if !setRequestLabels(req, info) {
		if len(results) == 0 {
			// return nil on empty result without a deletion
			return nil, nil
		}
	}

	req.SetCreationTimestamp(metav1.Now())
	return req, nil
}

func (builder *requestBuilder) buildRCRResult(policy string, resource response.ResourceSpec, rule kyverno.ViolatedRule) *report.PolicyReportResult {
	result := &report.PolicyReportResult{
		Policy: policy,
		Resources: []*v1.ObjectReference{
			{
				Kind:       resource.Kind,
				Namespace:  resource.Namespace,
				APIVersion: resource.APIVersion,
				Name:       resource.Name,
				UID:        types.UID(resource.UID),
			},
		},
		Scored:   true,
		Category: builder.fetchCategory(policy, resource.Namespace),
	}

	result.Rule = rule.Name
	result.Message = rule.Message
	result.Status = report.PolicyStatus(rule.Check)
	return result
}

func set(obj *unstructured.Unstructured, info Info) {
	obj.SetAPIVersion(request.SchemeGroupVersion.Group + "/" + request.SchemeGroupVersion.Version)

	if info.Namespace == "" {
		obj.SetGenerateName("crcr-")
		obj.SetKind("ClusterReportChangeRequest")
	} else {
		obj.SetGenerateName("rcr-")
		obj.SetKind("ReportChangeRequest")
		obj.SetNamespace(config.KyvernoNamespace)
	}

	obj.SetLabels(map[string]string{
		resourceLabelNamespace: info.Namespace,
	})
}

func setRequestLabels(req *unstructured.Unstructured, info Info) bool {
	switch {
	case isResourceDeletion(info):
		req.SetAnnotations(map[string]string{
			deletedAnnotationResourceName: info.Results[0].Resource.Name,
			deletedAnnotationResourceKind: info.Results[0].Resource.Kind,
		})

		req.SetLabels(map[string]string{
			resourceLabelNamespace: info.Results[0].Resource.Namespace,
		})
		return true

	case isPolicyDeletion(info):
		req.SetKind("ReportChangeRequest")
		req.SetGenerateName("rcr-")
		req.SetLabels(map[string]string{
			deletedLabelPolicy: info.PolicyName},
		)
		return true

	case isRuleDeletion(info):
		req.SetKind("ReportChangeRequest")
		req.SetGenerateName("rcr-")
		req.SetLabels(map[string]string{
			deletedLabelPolicy: info.PolicyName,
			deletedLabelRule:   info.Results[0].Rules[0].Name},
		)
		return true
	}

	return false
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

func buildPVInfo(er *response.EngineResponse) Info {
	info := Info{
		PolicyName: er.PolicyResponse.Policy,
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

func buildViolatedRules(er *response.EngineResponse) []kyverno.ViolatedRule {
	var violatedRules []kyverno.ViolatedRule
	for _, rule := range er.PolicyResponse.Rules {
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

const categoryLabel string = "policies.kyverno.io/category"

func (builder *requestBuilder) fetchCategory(policy, ns string) string {
	cpol, err := builder.cpolLister.Get(policy)
	if err == nil {
		if ann := cpol.GetAnnotations(); ann != nil {
			return ann[categoryLabel]
		}
	}

	pol, err := builder.polLister.Policies(ns).Get(policy)
	if err == nil {
		if ann := pol.GetAnnotations(); ann != nil {
			return ann[categoryLabel]
		}
	}

	return ""
}

func isResourceDeletion(info Info) bool {
	return info.PolicyName == "" && len(info.Results) == 1 && info.GetRuleLength() == 0
}

func isPolicyDeletion(info Info) bool {
	return info.PolicyName != "" && len(info.Results) == 0
}

func isRuleDeletion(info Info) bool {
	if info.PolicyName != "" && len(info.Results) == 1 {
		result := info.Results[0]
		if len(result.Rules) == 1 && reflect.DeepEqual(result.Resource, response.ResourceSpec{}) {
			return true
		}
	}
	return false
}
