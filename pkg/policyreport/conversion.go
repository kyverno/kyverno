package policyreport

import (
	"encoding/json"

	typercr "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	report "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func convertToRCR(request *unstructured.Unstructured) (*typercr.ReportChangeRequest, error) {
	rcr := typercr.ReportChangeRequest{}
	raw, err := request.MarshalJSON()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(raw, &rcr)
	rcr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   typercr.SchemeGroupVersion.Group,
		Version: typercr.SchemeGroupVersion.Version,
		Kind:    "ReportChangeRequest",
	})

	return &rcr, err
}

func convertToCRCR(request *unstructured.Unstructured) (*typercr.ClusterReportChangeRequest, error) {
	rcr := typercr.ClusterReportChangeRequest{}
	raw, err := request.MarshalJSON()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(raw, &rcr)
	rcr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   typercr.SchemeGroupVersion.Group,
		Version: typercr.SchemeGroupVersion.Version,
		Kind:    "ClusterReportChangeRequest",
	})

	return &rcr, err
}

func convertToPolr(request *unstructured.Unstructured) (*report.PolicyReport, error) {
	polr := report.PolicyReport{}
	raw, err := request.MarshalJSON()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(raw, &polr)
	polr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   report.SchemeGroupVersion.Group,
		Version: report.SchemeGroupVersion.Version,
		Kind:    "PolicyReport",
	})

	return &polr, err
}

func convertToCpolr(request *unstructured.Unstructured) (*report.ClusterPolicyReport, error) {
	cpolr := report.ClusterPolicyReport{}
	raw, err := request.MarshalJSON()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(raw, &cpolr)
	cpolr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   report.SchemeGroupVersion.Group,
		Version: report.SchemeGroupVersion.Version,
		Kind:    "ClusterPolicyReport",
	})

	return &cpolr, err
}
