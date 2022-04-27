package v1

import (
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type GenerateRequestListerExpansion interface{}

type GenerateRequestNamespaceListerExpansion interface {
	GetGenerateRequestsForClusterPolicy(policy string) ([]*v1.GenerateRequest, error)
	GetGenerateRequestsForResource(kind, namespace, name string) ([]*v1.GenerateRequest, error)
}

func (s generateRequestNamespaceLister) GetGenerateRequestsForResource(kind, namespace, name string) ([]*v1.GenerateRequest, error) {
	var list []*v1.GenerateRequest
	grs, err := s.List(labels.NewSelector())
	if err != nil {
		return nil, err
	}
	for idx, gr := range grs {
		if gr.Spec.Resource.Kind == kind &&
			gr.Spec.Resource.Namespace == namespace &&
			gr.Spec.Resource.Name == name {
			list = append(list, grs[idx])

		}
	}
	return list, err
}

func (s generateRequestNamespaceLister) GetGenerateRequestsForClusterPolicy(policy string) ([]*v1.GenerateRequest, error) {
	var list []*v1.GenerateRequest
	grs, err := s.List(labels.NewSelector())
	if err != nil {
		return nil, err
	}
	for idx, gr := range grs {
		if gr.Spec.Policy == policy {
			list = append(list, grs[idx])
		}
	}
	return list, err
}
