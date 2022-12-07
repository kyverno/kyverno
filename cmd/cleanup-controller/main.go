package main

import (
	"flag"
	"os"
	"sync"
	"time"

	admissionhandlers "github.com/kyverno/kyverno/cmd/cleanup-controller/handlers/admission"
	cleanuphandlers "github.com/kyverno/kyverno/cmd/cleanup-controller/handlers/cleanup"
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
	var cleanupService string
	flagset := flag.NewFlagSet("cleanup-controller", flag.ExitOnError)
	flagset.StringVar(&cleanupService, "cleanupService", "https://cleanup-controller.kyverno.svc", "The url to join the cleanup service.")
	// config
	appConfig := internal.NewConfiguration(
		internal.WithProfiling(),
		internal.WithMetrics(),
		internal.WithTracing(),
		internal.WithKubeconfig(),
		internal.WithFlagSets(flagset),
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
			cleanupService,
		),
		cleanup.Workers,
	)
	secretLister := kubeKyvernoInformer.Core().V1().Secrets().Lister()
	cpolLister := kyvernoInformer.Kyverno().V1alpha1().ClusterCleanupPolicies().Lister()
	polLister := kyvernoInformer.Kyverno().V1alpha1().CleanupPolicies().Lister()
	nsLister := kubeInformer.Core().V1().Namespaces().Lister()
	// start informers and wait for cache sync
	if !internal.StartInformersAndWaitForCacheSync(ctx, kubeKyvernoInformer, kubeInformer, kyvernoInformer) {
		os.Exit(1)
	}
	var wg sync.WaitGroup
	controller.Run(ctx, logger.WithName("cleanup-controller"), &wg)
	// create handlers
	admissionHandlers := admissionhandlers.New(dClient)
	cleanupHandlers := cleanuphandlers.New(dClient, cpolLister, polLister, nsLister)
	// create server
	server := NewServer(
		func() ([]byte, []byte, error) {
			secret, err := secretLister.Secrets(config.KyvernoNamespace()).Get("cleanup-controller-tls")
			if err != nil {
				return nil, nil, err
			}
			return secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey], nil
		},
		admissionHandlers.Validate,
		cleanupHandlers.Cleanup,
	)
	// start server
	server.Run(ctx.Done())
	// wait for termination signal
	wg.Wait()
}
