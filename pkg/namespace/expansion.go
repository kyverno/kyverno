package namespace

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	v1CoreLister "k8s.io/client-go/listers/core/v1"
)

//NamespaceListerExpansion ...
type NamespaceListerExpansion interface {
	v1CoreLister.NamespaceLister
	// List lists all Namespaces in the indexer.
	ListResources(selector labels.Selector) (ret []*v1.Namespace, err error)
	// GetsResource and injects gvk
	GetResource(name string) (*v1.Namespace, error)
}

//NamespaceLister ...
type NamespaceLister struct {
	v1CoreLister.NamespaceLister
}

//NewNamespaceLister returns a new NamespaceLister
func NewNamespaceLister(nsLister v1CoreLister.NamespaceLister) NamespaceListerExpansion {
	nsl := NamespaceLister{
		nsLister,
	}
	return &nsl
}

//ListResources is a wrapper to List and adds the resource kind information
// as the lister is specific to a gvk we can harcode the values here
func (nsl *NamespaceLister) ListResources(selector labels.Selector) (ret []*v1.Namespace, err error) {
	namespaces, err := nsl.List(selector)
	for index := range namespaces {
		namespaces[index].SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("Namespace"))
	}
	return namespaces, err
}

//GetResource is a wrapper to get the resource and inject the GVK
func (nsl *NamespaceLister) GetResource(name string) (*v1.Namespace, error) {
	namespace, err := nsl.Get(name)
	namespace.SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("Namespace"))
	return namespace, err
}
