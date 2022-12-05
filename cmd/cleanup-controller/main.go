package main

import (
	"os"
	"sync"
	"time"

	"github.com/kyverno/kyverno/cmd/internal"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	dynamicclient "github.com/kyverno/kyverno/pkg/clients/dynamic"
	kubeclient "github.com/kyverno/kyverno/pkg/clients/kube"
	kyvernoclient "github.com/kyverno/kyverno/pkg/clients/kyverno"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers/cleanup"
	"github.com/kyverno/kyverno/pkg/metrics"
	corev1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
)

const (
	resyncPeriod = 15 * time.Minute
)

func main() {
	// config
	appConfig := internal.NewConfiguration(
		internal.WithProfiling(),
		internal.WithMetrics(),
		internal.WithTracing(),
		internal.WithKubeconfig(),
	)
	// parse flags
	internal.ParseFlags(appConfig)
	// setup logger
	// show version
	// start profiling
	// setup signals
	// setup maxprocs
	// setup metrics
	ctx, logger, metricsConfig, sdown := internal.Setup()
	defer sdown()
	// create instrumented clients
	kubeClient := internal.CreateKubernetesClient(logger, kubeclient.WithMetrics(metricsConfig, metrics.KubeClient), kubeclient.WithTracing())
	dynamicClient := internal.CreateDynamicClient(logger, dynamicclient.WithMetrics(metricsConfig, metrics.KyvernoClient), dynamicclient.WithTracing())
	kyvernoClient := internal.CreateKyvernoClient(logger, kyvernoclient.WithMetrics(metricsConfig, metrics.KubeClient), kyvernoclient.WithTracing())
	dClient := internal.CreateDClient(logger, ctx, dynamicClient, kubeClient, 15*time.Minute)
	// informer factories
	kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod)
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
	// controllers
	controller := internal.NewController(
		cleanup.ControllerName,
		cleanup.NewController(
			kubeClient,
			kyvernoInformer.Kyverno().V1alpha1().ClusterCleanupPolicies(),
			kyvernoInformer.Kyverno().V1alpha1().CleanupPolicies(),
			kubeInformer.Batch().V1().CronJobs(),
		),
		cleanup.Workers,
	)
	secretLister := kubeKyvernoInformer.Core().V1().Secrets().Lister()
	// start informers and wait for cache sync
	if !internal.StartInformersAndWaitForCacheSync(ctx, kubeKyvernoInformer, kubeInformer, kyvernoInformer) {
		os.Exit(1)
	}
	var wg sync.WaitGroup
	controller.Run(ctx, logger.WithName("cleanup-controller"), &wg)
	server := NewServer(
		NewHandlers(dClient),
		func() ([]byte, []byte, error) {
			secret, err := secretLister.Secrets(config.KyvernoNamespace()).Get("cleanup-controller-tls")
			if err != nil {
				return nil, nil, err
			}
			return secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey], nil
		},
	)
	// start webhooks server
	server.Run(ctx.Done())
	// wait for termination signal
	wg.Wait()
}
