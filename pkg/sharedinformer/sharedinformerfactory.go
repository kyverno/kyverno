package sharedinformer

import (
	"fmt"

	policyclientset "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	informers "github.com/nirmata/kyverno/pkg/client/informers/externalversions"
	infomertypes "github.com/nirmata/kyverno/pkg/client/informers/externalversions/policy/v1alpha1"
	v1alpha1 "github.com/nirmata/kyverno/pkg/client/listers/policy/v1alpha1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type PolicyInformer interface {
	GetLister() v1alpha1.PolicyLister
	GetInfomer() cache.SharedIndexInformer
}

type SharedInfomer interface {
	PolicyInformer
	Run(stopCh <-chan struct{})
}

type sharedInfomer struct {
	policyInformerFactory informers.SharedInformerFactory
}

//NewSharedInformer returns shared informer
func NewSharedInformerFactory(clientConfig *rest.Config) (SharedInfomer, error) {
	// create policy client
	policyClientset, err := policyclientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("Error creating policyClient: %v\n", err)
	}
	//TODO: replace with NewSharedInformerFactoryWithOptions
	policyInformerFactory := informers.NewSharedInformerFactory(policyClientset, 0)
	return &sharedInfomer{
		policyInformerFactory: policyInformerFactory,
	}, nil
}

func (si *sharedInfomer) Run(stopCh <-chan struct{}) {
	si.policyInformerFactory.Start(stopCh)
}

func (si *sharedInfomer) getInfomer() infomertypes.PolicyInformer {
	return si.policyInformerFactory.Kubepolicy().V1alpha1().Policies()
}
func (si *sharedInfomer) GetInfomer() cache.SharedIndexInformer {
	return si.getInfomer().Informer()
}

func (si *sharedInfomer) GetLister() v1alpha1.PolicyLister {
	return si.getInfomer().Lister()
}
