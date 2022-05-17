package policyreport

import (
	"encoding/json"

	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func convertToRCR(request *unstructured.Unstructured) (*kyvernov1alpha2.ReportChangeRequest, error) {
	rcr := kyvernov1alpha2.ReportChangeRequest{}
	raw, err := request.MarshalJSON()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(raw, &rcr)
	rcr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   kyvernov1alpha2.SchemeGroupVersion.Group,
		Version: kyvernov1alpha2.SchemeGroupVersion.Version,
		Kind:    "ReportChangeRequest",
	})

	return &rcr, err
}

func convertToCRCR(request *unstructured.Unstructured) (*kyvernov1alpha2.ClusterReportChangeRequest, error) {
	rcr := kyvernov1alpha2.ClusterReportChangeRequest{}
	raw, err := request.MarshalJSON()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(raw, &rcr)
	rcr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   kyvernov1alpha2.SchemeGroupVersion.Group,
		Version: kyvernov1alpha2.SchemeGroupVersion.Version,
		Kind:    "ClusterReportChangeRequest",
	})

	return &rcr, err
}

func convertToPolr(request *unstructured.Unstructured) (*policyreportv1alpha2.PolicyReport, error) {
	polr := policyreportv1alpha2.PolicyReport{}
	raw, err := request.MarshalJSON()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(raw, &polr)
	polr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   policyreportv1alpha2.SchemeGroupVersion.Group,
		Version: policyreportv1alpha2.SchemeGroupVersion.Version,
		Kind:    "PolicyReport",
	})

	return &polr, err
}

func convertToCpolr(request *unstructured.Unstructured) (*policyreportv1alpha2.ClusterPolicyReport, error) {
	cpolr := policyreportv1alpha2.ClusterPolicyReport{}
	raw, err := request.MarshalJSON()
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(raw, &cpolr)
	cpolr.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   policyreportv1alpha2.SchemeGroupVersion.Group,
		Version: policyreportv1alpha2.SchemeGroupVersion.Version,
		Kind:    "ClusterPolicyReport",
	})

	return &cpolr, err
}
