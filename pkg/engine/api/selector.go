package api

import (
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"k8s.io/apimachinery/pkg/labels"
)

// NamespacedResourceSelector is an abstract interface used to list namespaced resources given a label selector
// Any implementation might exist, cache based, file based, client based etc...
type NamespacedResourceSelector[T any] interface {
	// List selects resources based on label selector.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []T, err error)
}

// PolicyExceptionSelector is an abstract interface used to resolve poliicy exceptions
type PolicyExceptionSelector = NamespacedResourceSelector[*kyvernov2beta1.PolicyException]
