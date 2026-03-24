package framework

import (
	"context"

	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
)

// NewGpolListers creates real informer-backed listers from a Kyverno clientset,
// mirroring the production wiring in cmd/kyverno/main.go:
// kyvernoClient → SharedInformerFactory → Listers.
func NewGpolListers(ctx context.Context, kyvernoClient kyvernoclient.Interface) (
	policiesv1beta1listers.GeneratingPolicyLister,
	policiesv1beta1listers.NamespacedGeneratingPolicyLister,
) {
	factory := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, 0)
	gpolLister := factory.Policies().V1beta1().GeneratingPolicies().Lister()
	ngpolLister := factory.Policies().V1beta1().NamespacedGeneratingPolicies().Lister()
	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
	return gpolLister, ngpolLister
}
