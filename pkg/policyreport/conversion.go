package policyreport

import (
	"encoding/json"

	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

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
