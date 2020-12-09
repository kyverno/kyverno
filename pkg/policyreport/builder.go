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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	clusterreportchangerequest string = "clusterreportchangerequest"
	resourceLabelName          string = "kyverno.io/resource.name"
	resourceLabelKind          string = "kyverno.io/resource.kind"
	resourceLabelNamespace     string = "kyverno.io/resource.namespace"
	policyLabel                string = "kyverno.io/policy"
	deletedLabelResource       string = "kyverno.io/delete.resource"
	deletedLabelResourceKind   string = "kyverno.io/delete.resource.kind"
	deletedLabelPolicy         string = "kyverno.io/delete.policy"
	deletedLabelRule           string = "kyverno.io/delete.rule"
)

func generatePolicyReportName(ns string) string {
	if ns == "" {
		return clusterpolicyreport
	}

	name := fmt.Sprintf("pr-ns-%s", ns)
	if len(name) > 63 {
		return name[:63]
	}

	return name
}

//GeneratePRsFromEngineResponse generate Violations from engine responses
func GeneratePRsFromEngineResponse(ers []response.EngineResponse, log logr.Logger) (pvInfos []Info) {
	for _, er := range ers {
		// ignore creation of PV for resources that are yet to be assigned a name
		if er.PolicyResponse.Resource.Name == "" {
			log.V(4).Info("resource does no have a name assigned yet, not creating a policy violation", "resource", er.PolicyResponse.Resource)
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
func NewBuilder(cpolLister kyvernolister.ClusterPolicyLister, polLister kyvernolister.PolicyLister) *requestBuilder {
	return &requestBuilder{cpolLister: cpolLister, polLister: polLister}
}

func (builder *requestBuilder) build(info Info) (req *unstructured.Unstructured, err error) {
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
			Scored:   true,
			Category: builder.fetchCategory(info.PolicyName, info.Resource.GetNamespace()),
		}

		result.Rule = rule.Name
		result.Message = rule.Message
		result.Status = report.PolicyStatus(rule.Check)
		results = append(results, result)
	}

	if info.Resource.GetNamespace() != "" {
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

	// deletion of a result entry
	if len(info.Rules) == 0 && info.PolicyName == "" { // on resource deleteion
		req.SetLabels(map[string]string{
			resourceLabelNamespace:   info.Resource.GetNamespace(),
			deletedLabelResource:     info.Resource.GetName(),
			deletedLabelResourceKind: info.Resource.GetKind()})
	} else if info.PolicyName != "" && reflect.DeepEqual(info.Resource, unstructured.Unstructured{}) { // on policy deleteion
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

func set(obj *unstructured.Unstructured, info Info) {
	resource := info.Resource
	obj.SetNamespace(config.KyvernoNamespace)
	obj.SetAPIVersion(request.SchemeGroupVersion.Group + "/" + request.SchemeGroupVersion.Version)
	if resource.GetNamespace() == "" {
		obj.SetGenerateName(clusterreportchangerequest + "-")
		obj.SetKind("ClusterReportChangeRequest")
	} else {
		obj.SetGenerateName("reportchangerequest-")
		obj.SetKind("ReportChangeRequest")
	}

	obj.SetLabels(map[string]string{
		resourceLabelNamespace: resource.GetNamespace(),
		resourceLabelName:      resource.GetName(),
		resourceLabelKind:      resource.GetKind(),
		policyLabel:            info.PolicyName,
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
