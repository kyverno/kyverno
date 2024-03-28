package informers

import (
	"fmt"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"k8s.io/client-go/tools/cache"
)

const (
	PolicyExceptionIndexName = "PolicyRulePair"
)

type policyExceptionInformer struct {
	informer cache.SharedIndexInformer
	indexer  cache.Indexer
}

func NewPolicyExceptionInformer(
	factory kyvernoinformer.SharedInformerFactory,
) (*policyExceptionInformer, error) {
	indexers := cache.Indexers{
		PolicyExceptionIndexName: policyRulePairIndexer,
	}

	informer := factory.Kyverno().V2beta1().PolicyExceptions().Informer()
	err := informer.AddIndexers(indexers)
	if err != nil {
		return nil, err
	}

	return &policyExceptionInformer{
		informer: informer,
		indexer:  informer.GetIndexer(),
	}, nil
}

func (i *policyExceptionInformer) Informer() cache.SharedIndexInformer {
	return i.informer
}

func (i *policyExceptionInformer) Indexer() cache.Indexer {
	return i.indexer
}

func (inf *policyExceptionInformer) GetPolicyExceptionsByPolicyRulePair(policyName, ruleName string) ([]*kyvernov2beta1.PolicyException, error) {
	indexKey := fmt.Sprintf("%s/%s", policyName, ruleName)
	objs, err := inf.indexer.ByIndex(PolicyExceptionIndexName, indexKey)
	if err != nil {
		return nil, err
	}

	wildcardIndexKey := fmt.Sprintf("%s/*", policyName)
	wildcardObjs, err := inf.indexer.ByIndex(PolicyExceptionIndexName, wildcardIndexKey)
	if err != nil {
		return nil, err
	}

	polexes := make([]*kyvernov2beta1.PolicyException, 0, len(objs)+len(wildcardObjs))
	if err := assertPolicyExceptionsTypes(objs, &polexes); err != nil {
		return nil, err
	}
	if err := assertPolicyExceptionsTypes(wildcardObjs, &polexes); err != nil {
		return nil, err
	}

	return polexes, nil
}

func policyRulePairIndexer(obj interface{}) ([]string, error) {
	polex, ok := obj.(*kyvernov2beta1.PolicyException)
	if !ok {
		return nil, fmt.Errorf("expected PolicyException, got %T", obj)
	}

	indexKeys := make([]string, 0, 1)
	for _, exception := range polex.Spec.Exceptions {
		for _, ruleName := range exception.RuleNames {
			indexKey := fmt.Sprintf("%s/%s", exception.PolicyName, ruleName)
			indexKeys = append(indexKeys, indexKey)
		}
	}

	return indexKeys, nil
}

func assertPolicyExceptionsTypes(objs []interface{}, polexes *[]*kyvernov2beta1.PolicyException) error {
	for _, obj := range objs {
		polex, ok := obj.(*kyvernov2beta1.PolicyException)
		if !ok {
			return fmt.Errorf("expected PolicyException, got %T", obj)
		}
		*polexes = append(*polexes, polex)
	}

	return nil
}
