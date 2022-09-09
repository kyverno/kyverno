package utils

import (
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func BuildDeletionPrInfo(oldR unstructured.Unstructured) policyreport.Info {
	return policyreport.Info{
		Namespace: oldR.GetNamespace(),
		Results: []policyreport.EngineResponseResult{
			{Resource: response.ResourceSpec{
				Kind:       oldR.GetKind(),
				APIVersion: oldR.GetAPIVersion(),
				Namespace:  oldR.GetNamespace(),
				Name:       oldR.GetName(),
				UID:        string(oldR.GetUID()),
			}},
		},
	}
}
