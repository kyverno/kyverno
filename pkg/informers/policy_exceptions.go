package informers

import (
	"errors"
	"time"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2beta1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2beta1"
	"k8s.io/client-go/tools/cache"
)

type policyExceptionInformer struct {
	indexer  cache.Indexer
	informer cache.SharedIndexInformer
	lister   kyvernolister.PolicyExceptionLister
}

func NewPolicyExceptionInformer(
	client versioned.Interface,
	namespace string,
	resyncPeriod time.Duration,
) *policyExceptionInformer {
	indexers := cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
		"name": func(obj interface{}) ([]string, error) {
			polex, ok := obj.(*kyvernov2beta1.PolicyException)
			if !ok {
				return []string{""}, errors.New("object is not a policy exception")
			}
			var keys []string
			for _, x := range polex.Spec.Exceptions {
				keys = append(keys, x.PolicyName)
			}
			return keys, nil
		},
	}
	informer := kyvernoinformer.NewFilteredPolicyExceptionInformer(
		client,
		namespace,
		resyncPeriod,
		indexers,
		nil,
	)
	indexer := informer.GetIndexer()
	lister := kyvernolister.NewPolicyExceptionLister(informer.GetIndexer())
	return &policyExceptionInformer{indexer, informer, lister}
}

func (i *policyExceptionInformer) Informer() cache.SharedIndexInformer {
	return i.informer
}

func (i *policyExceptionInformer) Lister() kyvernolister.PolicyExceptionLister {
	return i.lister
}

func (i *policyExceptionInformer) Find(policyName string) (ret []*kyvernov2beta1.PolicyException, err error) {
	objs, err := i.indexer.ByIndex("name", policyName)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		ret = append(ret, obj.(*kyvernov2beta1.PolicyException))
	}
	return
}
