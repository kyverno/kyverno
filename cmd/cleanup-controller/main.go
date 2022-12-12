package main

import (
	"context"
	"errors"
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
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tls"
	"github.com/kyverno/kyverno/pkg/webhooks"
	corev1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
)

const (
	resyncPeriod = 15 * time.Minute
)

// TODO:
// - helm review labels / selectors
// - implement probes
// - better certs management
// - supports certs in cronjob

type probes struct{}

func (probes) IsReady() bool {
	return true
}

func (probes) IsLive() bool {
	return true
}

func main() {
	var (
		leaderElectionRetryPeriod time.Duration
		dumpPayload               bool
	)
	flagset := flag.NewFlagSet("cleanup-controller", flag.ExitOnError)
	flagset.BoolVar(&dumpPayload, "dumpPayload", false, "Set this flag to activate/deactivate debug mode.")
	flagset.DurationVar(&leaderElectionRetryPeriod, "leaderElectionRetryPeriod", leaderelection.DefaultRetryPeriod, "Configure leader election retry period.")
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
	leaderElectionClient := internal.CreateKubernetesClient(logger, kubeclient.WithMetrics(metricsConfig, metrics.KubeClient), kubeclient.WithTracing())
	kyvernoClient := internal.CreateKyvernoClient(logger, kyvernoclient.WithMetrics(metricsConfig, metrics.KubeClient), kyvernoclient.WithTracing())
	// setup leader election
	le, err := leaderelection.New(
		logger.WithName("leader-election"),
		"kyverno-cleanup-controller",
		config.KyvernoNamespace(),
		leaderElectionClient,
		config.KyvernoPodName(),
		leaderElectionRetryPeriod,
		func(ctx context.Context) {
			logger := logger.WithName("leader")
			// informer factories
			kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod)
			kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
			// controllers
			controller := internal.NewController(
				cleanup.ControllerName,
				cleanup.NewController(
					kubeClient,
					kyvernoInformer.Kyverno().V2alpha1().ClusterCleanupPolicies(),
					kyvernoInformer.Kyverno().V2alpha1().CleanupPolicies(),
					kubeInformer.Batch().V1().CronJobs(),
					"https://"+config.KyvernoServiceName()+"."+config.KyvernoNamespace()+".svc",
				),
				cleanup.Workers,
			)
			// start informers and wait for cache sync
			if !internal.StartInformersAndWaitForCacheSync(ctx, kyvernoInformer, kubeInformer) {
				logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
				os.Exit(1)
			}
			// start leader controllers
			var wg sync.WaitGroup
			controller.Run(ctx, logger.WithName("cleanup-controller"), &wg)
			// wait all controllers shut down
			wg.Wait()
		},
		nil,
	)
	if err != nil {
		logger.Error(err, "failed to initialize leader election")
		os.Exit(1)
	}
	dynamicClient := internal.CreateDynamicClient(logger, dynamicclient.WithMetrics(metricsConfig, metrics.KyvernoClient), dynamicclient.WithTracing())
	dClient := internal.CreateDClient(logger, ctx, dynamicClient, kubeClient, 15*time.Minute)
	// informer factories
	kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod)
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
	// listers
	secretLister := kubeKyvernoInformer.Core().V1().Secrets().Lister().Secrets(config.KyvernoNamespace())
	cpolLister := kyvernoInformer.Kyverno().V2alpha1().ClusterCleanupPolicies().Lister()
	polLister := kyvernoInformer.Kyverno().V2alpha1().CleanupPolicies().Lister()
	nsLister := kubeInformer.Core().V1().Namespaces().Lister()
	// start informers and wait for cache sync
	if !internal.StartInformersAndWaitForCacheSync(ctx, kubeKyvernoInformer, kubeInformer, kyvernoInformer) {
		os.Exit(1)
	}
	// create handlers
	admissionHandlers := admissionhandlers.New(dClient)
	cleanupHandlers := cleanuphandlers.New(dClient, cpolLister, polLister, nsLister)
	// create server
	server := NewServer(
		func() ([]byte, []byte, error) {
			secret, err := secretLister.Get(tls.GenerateTLSPairSecretName())
			if err != nil {
				return nil, nil, err
			}
			return secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey], nil
		},
		admissionHandlers.Validate,
		cleanupHandlers.Cleanup,
		metricsConfig,
		webhooks.DebugModeOptions{
			DumpPayload: dumpPayload,
		},
		probes{},
	)
	// start server
	server.Run(ctx.Done())
	// wait for termination signal and run leader election loop
	for {
		select {
		case <-ctx.Done():
			return
		default:
			le.Run(ctx)
		}
	}
}
