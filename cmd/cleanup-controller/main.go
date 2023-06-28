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
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers/certmanager"
	"github.com/kyverno/kyverno/pkg/controllers/cleanup"
	genericloggingcontroller "github.com/kyverno/kyverno/pkg/controllers/generic/logging"
	genericwebhookcontroller "github.com/kyverno/kyverno/pkg/controllers/generic/webhook"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/tls"
	"github.com/kyverno/kyverno/pkg/webhooks"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
)

const (
	resyncPeriod          = 15 * time.Minute
	webhookWorkers        = 2
	webhookControllerName = "webhook-controller"
)

// TODO:
// - helm review labels / selectors
// - implement probes
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
		dumpPayload bool
		serverIP    string
		servicePort int
	)
	flagset := flag.NewFlagSet("cleanup-controller", flag.ExitOnError)
	flagset.BoolVar(&dumpPayload, "dumpPayload", false, "Set this flag to activate/deactivate debug mode.")
	flagset.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flagset.IntVar(&servicePort, "servicePort", 443, "Port used by the Kyverno Service resource and for webhook configurations.")
	// config
	appConfig := internal.NewConfiguration(
		internal.WithProfiling(),
		internal.WithMetrics(),
		internal.WithTracing(),
		internal.WithKubeconfig(),
		internal.WithLeaderElection(),
		internal.WithKyvernoClient(),
		internal.WithKyvernoDynamicClient(),
		internal.WithConfigMapCaching(),
		internal.WithDeferredLoading(),
		internal.WithFlagSets(flagset),
	)
	// parse flags
	internal.ParseFlags(appConfig)
	// setup
	ctx, setup, sdown := internal.Setup(appConfig, "kyverno-cleanup-controller", false)
	defer sdown()
	// setup leader election
	le, err := leaderelection.New(
		setup.Logger.WithName("leader-election"),
		"kyverno-cleanup-controller",
		config.KyvernoNamespace(),
		setup.LeaderElectionClient,
		config.KyvernoPodName(),
		internal.LeaderElectionRetryPeriod(),
		func(ctx context.Context) {
			logger := setup.Logger.WithName("leader")
			// informer factories
			kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(setup.KubeClient, resyncPeriod)
			kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, resyncPeriod)
			kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(setup.KubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
			// listers
			secretLister := kubeKyvernoInformer.Core().V1().Secrets().Lister().Secrets(config.KyvernoNamespace())
			// controllers
			renewer := tls.NewCertRenewer(
				setup.KubeClient.CoreV1().Secrets(config.KyvernoNamespace()),
				secretLister,
				tls.CertRenewalInterval,
				tls.CAValidityDuration,
				tls.TLSValidityDuration,
				serverIP,
			)
			certController := internal.NewController(
				certmanager.ControllerName,
				certmanager.NewController(
					kubeKyvernoInformer.Core().V1().Secrets(),
					renewer,
				),
				certmanager.Workers,
			)
			webhookController := internal.NewController(
				webhookControllerName,
				genericwebhookcontroller.NewController(
					webhookControllerName,
					setup.KubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
					kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations(),
					kubeKyvernoInformer.Core().V1().Secrets(),
					config.CleanupValidatingWebhookConfigurationName,
					config.CleanupValidatingWebhookServicePath,
					serverIP,
					int32(servicePort),
					[]admissionregistrationv1.RuleWithOperations{{
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{"kyverno.io"},
							APIVersions: []string{"v2alpha1"},
							Resources: []string{
								"cleanuppolicies/*",
								"clustercleanuppolicies/*",
							},
						},
						Operations: []admissionregistrationv1.OperationType{
							admissionregistrationv1.Create,
							admissionregistrationv1.Update,
						},
					}},
					genericwebhookcontroller.Fail,
					genericwebhookcontroller.None,
					setup.Configuration,
				),
				webhookWorkers,
			)
			cleanupController := internal.NewController(
				cleanup.ControllerName,
				cleanup.NewController(
					setup.KubeClient,
					kyvernoInformer.Kyverno().V2alpha1().ClusterCleanupPolicies(),
					kyvernoInformer.Kyverno().V2alpha1().CleanupPolicies(),
					kubeInformer.Batch().V1().CronJobs(),
					"https://"+config.KyvernoServiceName()+"."+config.KyvernoNamespace()+".svc",
				),
				cleanup.Workers,
			)
			// start informers and wait for cache sync
			if !internal.StartInformersAndWaitForCacheSync(ctx, logger, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
				logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
				os.Exit(1)
			}
			// start leader controllers
			var wg sync.WaitGroup
			certController.Run(ctx, logger, &wg)
			webhookController.Run(ctx, logger, &wg)
			cleanupController.Run(ctx, logger, &wg)
			// wait all controllers shut down
			wg.Wait()
		},
		nil,
	)
	if err != nil {
		setup.Logger.Error(err, "failed to initialize leader election")
		os.Exit(1)
	}
	// informer factories
	kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(setup.KubeClient, resyncPeriod)
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(setup.KubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, resyncPeriod)
	// listers
	secretLister := kubeKyvernoInformer.Core().V1().Secrets().Lister().Secrets(config.KyvernoNamespace())
	cpolLister := kyvernoInformer.Kyverno().V2alpha1().ClusterCleanupPolicies().Lister()
	polLister := kyvernoInformer.Kyverno().V2alpha1().CleanupPolicies().Lister()
	nsLister := kubeInformer.Core().V1().Namespaces().Lister()
	// log policy changes
	genericloggingcontroller.NewController(
		setup.Logger.WithName("cleanup-policy"),
		"CleanupPolicy",
		kyvernoInformer.Kyverno().V2alpha1().CleanupPolicies(),
		genericloggingcontroller.CheckGeneration,
	)
	genericloggingcontroller.NewController(
		setup.Logger.WithName("cluster-cleanup-policy"),
		"ClusterCleanupPolicy",
		kyvernoInformer.Kyverno().V2alpha1().ClusterCleanupPolicies(),
		genericloggingcontroller.CheckGeneration,
	)
	// start informers and wait for cache sync
	if !internal.StartInformersAndWaitForCacheSync(ctx, setup.Logger, kubeKyvernoInformer, kubeInformer, kyvernoInformer) {
		os.Exit(1)
	}
	// create handlers
	admissionHandlers := admissionhandlers.New(setup.KyvernoDynamicClient)
	cleanupHandlers := cleanuphandlers.New(setup.Logger.WithName("cleanup-handler"), setup.KyvernoDynamicClient, cpolLister, polLister, nsLister, setup.Jp)
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
		setup.MetricsManager,
		webhooks.DebugModeOptions{
			DumpPayload: dumpPayload,
		},
		probes{},
		setup.Configuration,
	)
	// start server
	server.Run(ctx.Done())
	// start leader election
	le.Run(ctx)
}
