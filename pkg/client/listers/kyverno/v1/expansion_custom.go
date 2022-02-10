package v1

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

type GenerateRequestNamespaceListerExpansion interface {
	GetGenerateRequestsForClusterPolicy(policy string) ([]*kyvernov1.GenerateRequest, error)
	GetGenerateRequestsForResource(kind, namespace, name string) ([]*kyvernov1.GenerateRequest, error)
}
