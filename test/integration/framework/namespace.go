package framework

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// NewNamespaceResolver returns the namespace resolver the admission and background controllers
// use (celengine.NewNamespaceResolver, wired as in cmd/kyverno/main.go): a namespace lister with a
// live GET fallback for namespaces the informer has not seen yet. Engines use it to match a
// namespaceSelector against the namespace's real labels and to populate namespaceObject, for
// policies and PolicyExceptions alike. The informer runs until ctx is cancelled. It does not wait
// for the informer to sync: until it has, lookups fall back to the live GET, as in production.
func NewNamespaceResolver(ctx context.Context, kubeClient kubernetes.Interface) celengine.NamespaceResolver {
	factory, lister := namespaceInformer(kubeClient)
	factory.Start(ctx.Done())
	return celengine.NewNamespaceResolver(logr.Discard(), lister, kubeClient)
}

// namespaceResolverForManager is NewNamespaceResolver for the engine constructors that receive a
// manager: the namespace informer starts and stops with the manager. Until it has synced, lookups
// fall back to a live GET, the same as in production.
func namespaceResolverForManager(mgr ctrl.Manager) (celengine.NamespaceResolver, error) {
	kubeClient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create kube client for the namespace resolver: %w", err)
	}
	factory, lister := namespaceInformer(kubeClient)
	if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		factory.Start(ctx.Done())
		<-ctx.Done()
		factory.Shutdown()
		return nil
	})); err != nil {
		return nil, fmt.Errorf("failed to register the namespace informer: %w", err)
	}
	return celengine.NewNamespaceResolver(logr.Discard(), lister, kubeClient), nil
}

func namespaceInformer(kubeClient kubernetes.Interface) (informers.SharedInformerFactory, corev1listers.NamespaceLister) {
	factory := informers.NewSharedInformerFactory(kubeClient, 0)
	// Taking the lister registers the informer, so it must happen before Start.
	return factory, factory.Core().V1().Namespaces().Lister()
}
