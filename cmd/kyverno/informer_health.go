package main

import (
	"context"
	"time"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/informers/health"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

// registerAdmissionInformers must run before any consumers request these types
// from the admission controller's unfiltered, all-namespaces factory. The
// factory then supplies the same monitored instance to every existing consumer.
func registerAdmissionInformers(factory kyvernoinformers.SharedInformerFactory, client kyvernoclient.Interface, tracker *health.Tracker) {
	register := func(obj runtime.Object, lw cache.ListerWatcher) {
		factory.InformerFor(obj, func(_ kyvernoclient.Interface, resync time.Duration) cache.SharedIndexInformer {
			return tracker.NewInformer(lw, obj, resync, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		})
	}
	register(&kyvernov2.GlobalContextEntry{}, admissionListWatch(client, client.KyvernoV2().GlobalContextEntries().List, client.KyvernoV2().GlobalContextEntries().Watch))
}

func admissionListWatch[T runtime.Object](client any, list func(context.Context, metav1.ListOptions) (T, error), watch func(context.Context, metav1.ListOptions) (watch.Interface, error)) cache.ListerWatcher {
	return cache.ToListWatcherWithWatchListSemantics(&cache.ListWatch{
		ListWithContextFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return list(ctx, options)
		},
		WatchFuncWithContext: watch,
	}, client)
}
