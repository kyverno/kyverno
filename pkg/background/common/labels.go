package common

import (
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	pkglabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

type Object interface {
	GetName() string
	GetNamespace() string
	GetKind() string
	GetAPIVersion() string
	GetUID() types.UID
}

func GenerateLabelsSet(policyKey string) pkglabels.Set {
	_, policyName, _ := cache.SplitMetaNamespaceKey(policyKey)

	set := pkglabels.Set{
		kyvernov2.URGeneratePolicyLabel: policyName,
	}
	return set
}
