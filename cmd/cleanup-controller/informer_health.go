package main

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/informers/health"
	"github.com/kyverno/kyverno/pkg/tls"
	runtimeutils "github.com/kyverno/kyverno/pkg/utils/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/clock"
)

func setupRuntimeProbes(
	logger logr.Logger,
	kubeClient kubernetes.Interface,
	kyvernoClient kyvernoclient.Interface,
	kyvernoInformer kyvernoinformers.SharedInformerFactory,
	kyvernoDeployment appsv1informers.DeploymentInformer,
	clk clock.Clock,
	serverIP string,
	renewBefore time.Duration,
	caSecretName string,
	tlsSecretName string,
	keyAlgorithm tls.KeyAlgorithm,
) (tls.CertRenewer, runtimeutils.Runtime) {
	informerHealth := health.NewTracker(logger.WithName("informer-health"), clk)
	registerCleanupInformers(kyvernoInformer, kyvernoClient, informerHealth)
	certRenewer := tls.NewCertRenewer(
		kubeClient.CoreV1().Secrets(config.KyvernoNamespace()),
		tls.CertRenewalInterval,
		tls.CAValidityDuration,
		tls.TLSValidityDuration,
		renewBefore,
		serverIP,
		config.KyvernoServiceName(),
		config.DnsNames(config.KyvernoServiceName(), config.KyvernoNamespace()),
		config.KyvernoNamespace(),
		caSecretName,
		tlsSecretName,
		keyAlgorithm,
	)
	runtime := runtimeutils.NewRuntime(
		logger.WithName("runtime-checks"),
		serverIP,
		kyvernoDeployment,
		certRenewer,
		informerHealth.Ready,
	)
	return certRenewer, runtime
}

// registerCleanupInformers must run before any consumers request these types
// from the cleanup-controller's outer-scope factory so every consumer shares
// the same health-tracked informer instance.
func registerCleanupInformers(factory kyvernoinformers.SharedInformerFactory, client kyvernoclient.Interface, tracker *health.Tracker) {
	register := func(obj runtime.Object, lw cache.ListerWatcher) {
		factory.InformerFor(obj, func(_ kyvernoclient.Interface, resync time.Duration) cache.SharedIndexInformer {
			return tracker.NewInformer(lw, obj, resync, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		})
	}
	register(&kyvernov2beta1.GlobalContextEntry{}, cleanupListWatch(client, client.KyvernoV2beta1().GlobalContextEntries().List, client.KyvernoV2beta1().GlobalContextEntries().Watch))
}

func cleanupListWatch[T runtime.Object](client any, list func(context.Context, metav1.ListOptions) (T, error), watch func(context.Context, metav1.ListOptions) (watch.Interface, error)) cache.ListerWatcher {
	return cache.ToListWatcherWithWatchListSemantics(&cache.ListWatch{
		ListWithContextFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
			return list(ctx, options)
		},
		WatchFuncWithContext: watch,
	}, client)
}
