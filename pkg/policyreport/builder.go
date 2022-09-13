package policyreport

import (
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// Builder builds report change request struct
// this is base type of namespaced and cluster policy report
type Builder interface {
	build(info Info) (*kyvernov1alpha2.ClusterReportChangeRequest, *kyvernov1alpha2.ReportChangeRequest, error)
}

type requestBuilder struct {
	cpolLister kyvernov1listers.ClusterPolicyLister
	polLister  kyvernov1listers.PolicyLister
}

// NewBuilder ...
func NewBuilder(cpolLister kyvernov1listers.ClusterPolicyLister, polLister kyvernov1listers.PolicyLister) Builder {
	return &requestBuilder{cpolLister: cpolLister, polLister: polLister}
}

func (builder *requestBuilder) build(info Info) (*kyvernov1alpha2.ClusterReportChangeRequest, *kyvernov1alpha2.ReportChangeRequest, error) {
	results := builder.buildResults(info)
	summary := calculateSummary(results)

	if info.Namespace != "" {
		rr := &kyvernov1alpha2.ReportChangeRequest{
			Results: results,
			Summary: summary,
		}
		set(rr, info)
		rr.SetCreationTimestamp(metav1.Now())
		if !setRequestDeletionLabels(rr, info) {
			if len(results) == 0 {
				// return nil on empty result without a deletion
				return nil, nil, nil
			}
		}
		return nil, rr, nil
	} else {
		rr := &kyvernov1alpha2.ClusterReportChangeRequest{
			Results: results,
			Summary: summary,
		}
		set(rr, info)
		rr.SetCreationTimestamp(metav1.Now())
		if !setRequestDeletionLabels(rr, info) {
			if len(results) == 0 {
				// return nil on empty result without a deletion
				return nil, nil, nil
			}
		}
		return rr, nil, nil
	}
}

func (builder *requestBuilder) buildResults(info Info) []policyreportv1alpha2.PolicyReportResult {
	var results []policyreportv1alpha2.PolicyReportResult
	for _, infoResult := range info.Results {
		for _, rule := range infoResult.Rules {
			if rule.Type != string(response.Validation) && rule.Type != string(response.ImageVerify) {
				continue
			}
			results = append(results, builder.buildRCRResult(info.PolicyName, infoResult.Resource, rule))
		}
	}
	return results
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

func set(obj metav1.Object, info Info) {
	if info.Namespace == "" {
		obj.SetGenerateName("crcr-")
	} else {
		obj.SetGenerateName("rcr-")
		obj.SetNamespace(config.KyvernoNamespace())
	}
	obj.SetLabels(map[string]string{
		ResourceLabelNamespace: info.Namespace,
		policyLabel:            trimmedName(info.PolicyName),
		appVersion:             version.BuildVersion,
	})
}

func setRequestDeletionLabels(obj metav1.Object, info Info) bool {
	switch {
	case info.isResourceDeletion():
		obj.SetAnnotations(map[string]string{
			deletedAnnotationResourceName: info.Results[0].Resource.Name,
			deletedAnnotationResourceKind: info.Results[0].Resource.Kind,
		})
		labels := obj.GetLabels()
		labels[ResourceLabelNamespace] = info.Results[0].Resource.Namespace
		obj.SetLabels(labels)
		return true

	case info.isPolicyDeletion():
		labels := obj.GetLabels()
		labels[deletedLabelPolicy] = info.PolicyName
		obj.SetLabels(labels)
		return true

	case info.isRuleDeletion():
		labels := obj.GetLabels()
		labels[deletedLabelPolicy] = info.PolicyName
		labels[deletedLabelRule] = info.Results[0].Rules[0].Name
		obj.SetLabels(labels)
		return true
	}

	return false
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
