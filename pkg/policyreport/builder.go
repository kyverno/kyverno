package policyreport

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	request "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	report "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/version"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// appVersion represents which version of Kyverno manages rcr / crcr
	appVersion string = "app.kubernetes.io/version"

	// the following labels are used to list rcr / crcr
	resourceLabelNamespace string = "kyverno.io/resource.namespace"
	deletedLabelPolicy     string = "kyverno.io/delete.policy"
	deletedLabelRule       string = "kyverno.io/delete.rule"

	// the following annotations are used to remove entries from polr / cpolr
	// there would be a problem if use labels as the value could exceed 63 chars
	deletedAnnotationResourceName string = "kyverno.io/delete.resource.name"
	deletedAnnotationResourceKind string = "kyverno.io/delete.resource.kind"

	// SourceValue is the static value for PolicyReportResult.Source
	SourceValue = "Kyverno"
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
	req = new(unstructured.Unstructured)
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

		gv := report.SchemeGroupVersion
		rr.SetGroupVersionKind(schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: "ReportChangeRequest"})

		rawRcr, err := json.Marshal(rr)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(rawRcr, req)
		if err != nil {
			return nil, err
		}

		set(req, info)
	} else {
		rr := &request.ClusterReportChangeRequest{
			Summary: calculateSummary(results),
			Results: results,
		}

		gv := report.SchemeGroupVersion
		rr.SetGroupVersionKind(schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: "ClusterReportChangeRequest"})

		rawRcr, err := json.Marshal(rr)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(rawRcr, req)
		if err != nil {
			return nil, err
		}

		set(req, info)
	}

	if !setRequestDeletionLabels(req, info) {
		if len(results) == 0 {
			// return nil on empty result without a deletion
			return nil, nil
		}
	}

	req.SetCreationTimestamp(metav1.Now())
	return req, nil
}

func (builder *requestBuilder) buildRCRResult(policy string, resource response.ResourceSpec, rule kyverno.ViolatedRule) *report.PolicyReportResult {
	av := builder.fetchAnnotationValues(policy, resource.Namespace)

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
		Scored:   av.scored,
		Category: av.category,
		Severity: av.severity,
	}

	result.Rule = rule.Name
	result.Message = rule.Message
	result.Result = report.PolicyResult(rule.Status)
	if result.Result == "fail" && !av.scored {
		result.Result = "warn"
	}
	result.Source = SourceValue
	result.Timestamp = metav1.Timestamp{
		Seconds: time.Now().Unix(),
	}
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
		appVersion:             version.BuildVersion,
	})
}

func setRequestDeletionLabels(req *unstructured.Unstructured, info Info) bool {
	switch {
	case isResourceDeletion(info):
		req.SetAnnotations(map[string]string{
			deletedAnnotationResourceName: info.Results[0].Resource.Name,
			deletedAnnotationResourceKind: info.Results[0].Resource.Kind,
		})

		labels := req.GetLabels()
		labels[resourceLabelNamespace] = info.Results[0].Resource.Namespace
		req.SetLabels(labels)
		return true

	case isPolicyDeletion(info):
		req.SetKind("ReportChangeRequest")
		req.SetGenerateName("rcr-")

		labels := req.GetLabels()
		labels[deletedLabelPolicy] = info.PolicyName
		req.SetLabels(labels)
		return true

	case isRuleDeletion(info):
		req.SetKind("ReportChangeRequest")
		req.SetGenerateName("rcr-")

		labels := req.GetLabels()
		labels[deletedLabelPolicy] = info.PolicyName
		labels[deletedLabelRule] = info.Results[0].Rules[0].Name
		req.SetLabels(labels)
		return true
	}

	return false
}

func calculateSummary(results []*report.PolicyReportResult) (summary report.PolicyReportSummary) {
	for _, res := range results {
		switch string(res.Result) {
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

func buildViolatedRules(er *response.EngineResponse) []kyverno.ViolatedRule {
	var violatedRules []kyverno.ViolatedRule
	for _, rule := range er.PolicyResponse.Rules {
		vrule := kyverno.ViolatedRule{
			Name:    rule.Name,
			Type:    rule.Type,
			Message: rule.Message,
		}

		vrule.Status = toPolicyResult(rule.Status)
		violatedRules = append(violatedRules, vrule)
	}

	return violatedRules
}

func toPolicyResult(status response.RuleStatus) string {
	switch status {
	case response.RuleStatusPass:
		return report.StatusPass
	case response.RuleStatusFail:
		return report.StatusFail
	case response.RuleStatusError:
		return report.StatusError
	case response.RuleStatusWarn:
		return report.StatusWarn
	case response.RuleStatusSkip:
		return report.StatusSkip
	}

	return ""
}

const categoryLabel string = "policies.kyverno.io/category"
const severityLabel string = "policies.kyverno.io/severity"
const scoredLabel string = "policies.kyverno.io/scored"

type annotationValues struct {
	category string
	severity report.PolicySeverity
	scored   bool
}

func (av *annotationValues) setSeverityFromString(severity string) {
	switch severity {
	case report.SeverityHigh:
		av.severity = report.SeverityHigh
	case report.SeverityMedium:
		av.severity = report.SeverityMedium
	case report.SeverityLow:
		av.severity = report.SeverityLow
	}
}

func (builder *requestBuilder) fetchAnnotationValues(policy, ns string) annotationValues {
	av := annotationValues{}
	ann := builder.fetchAnnotations(policy, ns)

	if category, ok := ann[categoryLabel]; ok {
		av.category = category
	}
	if severity, ok := ann[severityLabel]; ok {
		av.setSeverityFromString(severity)
	}
	if scored, ok := ann[scoredLabel]; ok {
		if scored == "false" {
			av.scored = false
		} else {
			av.scored = true
		}
	} else {
		av.scored = true
	}

	return av
}

func (builder *requestBuilder) fetchAnnotations(policy, ns string) map[string]string {
	cpol, err := builder.cpolLister.Get(policy)
	if err == nil {
		if ann := cpol.GetAnnotations(); ann != nil {
			return ann
		}
	}

	pol, err := builder.polLister.Policies(ns).Get(policy)
	if err == nil {
		if ann := pol.GetAnnotations(); ann != nil {
			return ann
		}
	}

	return make(map[string]string)
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
