package policyreport

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/kyverno/pkg/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// appVersion represents which version of Kyverno manages rcr / crcr
	appVersion string = "app.kubernetes.io/version"

	// the following labels are used to list rcr / crcr
	ResourceLabelNamespace string = "kyverno.io/resource.namespace"
	policyLabel            string = "kyverno.io/policy-name"
	deletedLabelPolicy     string = "kyverno.io/delete.policy"
	deletedLabelRule       string = "kyverno.io/delete.rule"

	// the following annotations are used to remove entries from polr / cpolr
	// there would be a problem if use labels as the value could exceed 63 chars
	deletedAnnotationResourceName string = "kyverno.io/delete.resource.name"
	deletedAnnotationResourceKind string = "kyverno.io/delete.resource.kind"

	inactiveLabelKey string = "kyverno.io/report.status"
	inactiveLabelVal string = "inactive"

	// SourceValue is the static value for PolicyReportResult.Source
	SourceValue = "Kyverno"
)

func GeneratePolicyReportName(ns, policyName string) string {
	if ns == "" {
		if toggle.SplitPolicyReport() {
			return TrimmedName(clusterpolicyreport + "-" + policyName)
		}
		return clusterpolicyreport
	}

	var name string
	if toggle.SplitPolicyReport() {
		name = fmt.Sprintf("polr-ns-%s-%s", ns, policyName)
	} else {
		name = fmt.Sprintf("polr-ns-%s", ns)
	}
	if len(name) > 63 {
		return name[:63]
	}

	return name
}

func TrimmedName(s string) string {
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

// Builder builds report change request struct
// this is base type of namespaced and cluster policy report
type Builder interface {
	build(info Info) (*unstructured.Unstructured, error)
}

type requestBuilder struct {
	cpolLister kyvernov1listers.ClusterPolicyLister
	polLister  kyvernov1listers.PolicyLister
}

// NewBuilder ...
func NewBuilder(cpolLister kyvernov1listers.ClusterPolicyLister, polLister kyvernov1listers.PolicyLister) Builder {
	return &requestBuilder{cpolLister: cpolLister, polLister: polLister}
}

func (builder *requestBuilder) build(info Info) (req *unstructured.Unstructured, err error) {
	results := []policyreportv1alpha2.PolicyReportResult{}
	req = new(unstructured.Unstructured)
	for _, infoResult := range info.Results {
		for _, rule := range infoResult.Rules {
			if rule.Type != string(response.Validation) && rule.Type != string(response.ImageVerify) {
				continue
			}

			result := builder.buildRCRResult(info.PolicyName, infoResult.Resource, rule)
			results = append(results, result)
		}
	}

	if info.Namespace != "" {
		rr := &kyvernov1alpha2.ReportChangeRequest{
			Summary: calculateSummary(results),
			Results: results,
		}

		gv := policyreportv1alpha2.SchemeGroupVersion
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
		rr := &kyvernov1alpha2.ClusterReportChangeRequest{
			Summary: calculateSummary(results),
			Results: results,
		}

		gv := policyreportv1alpha2.SchemeGroupVersion
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

func (builder *requestBuilder) buildRCRResult(policy string, resource response.ResourceSpec, rule kyvernov1.ViolatedRule) policyreportv1alpha2.PolicyReportResult {
	av := builder.fetchAnnotationValues(policy, resource.Namespace)

	result := policyreportv1alpha2.PolicyReportResult{
		Policy: policy,
		Resources: []corev1.ObjectReference{
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
	result.Result = policyreportv1alpha2.PolicyResult(rule.Status)
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
	obj.SetAPIVersion(kyvernov1alpha2.SchemeGroupVersion.Group + "/" + kyvernov1alpha2.SchemeGroupVersion.Version)

	if info.Namespace == "" {
		obj.SetGenerateName("crcr-")
		obj.SetKind("ClusterReportChangeRequest")
	} else {
		obj.SetGenerateName("rcr-")
		obj.SetKind("ReportChangeRequest")
		obj.SetNamespace(config.KyvernoNamespace())
	}

	obj.SetLabels(map[string]string{
		ResourceLabelNamespace: info.Namespace,
		policyLabel:            TrimmedName(info.PolicyName),
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
		labels[ResourceLabelNamespace] = info.Results[0].Resource.Namespace
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

const (
	categoryLabel string = "policies.kyverno.io/category"
	severityLabel string = "policies.kyverno.io/severity"
	ScoredLabel   string = "policies.kyverno.io/scored"
)

type annotationValues struct {
	category string
	severity policyreportv1alpha2.PolicySeverity
	scored   bool
}

func (av *annotationValues) setSeverityFromString(severity string) {
	switch severity {
	case policyreportv1alpha2.SeverityHigh:
		av.severity = policyreportv1alpha2.SeverityHigh
	case policyreportv1alpha2.SeverityMedium:
		av.severity = policyreportv1alpha2.SeverityMedium
	case policyreportv1alpha2.SeverityLow:
		av.severity = policyreportv1alpha2.SeverityLow
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
	if scored, ok := ann[ScoredLabel]; ok {
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
