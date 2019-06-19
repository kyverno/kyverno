package sharedinformer

import (
	"github.com/nirmata/kyverno/pkg/client/clientset/versioned/fake"
	informers "github.com/nirmata/kyverno/pkg/client/informers/externalversions"
	"k8s.io/apimachinery/pkg/runtime"
)

func NewFakeSharedInformerFactory(objects ...runtime.Object) (SharedInfomer, error) {
	fakePolicyClient := fake.NewSimpleClientset(objects...)
	policyInformerFactory := informers.NewSharedInformerFactory(fakePolicyClient, 0)
	return &sharedInfomer{
		policyInformerFactory: policyInformerFactory,
	}, nil
}
