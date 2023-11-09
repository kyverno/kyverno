package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"sync"
	"time"

	"github.com/kyverno/kyverno/api/kyverno"
	policyhandlers "github.com/kyverno/kyverno/cmd/cleanup-controller/handlers/admission/policy"
	resourcehandlers "github.com/kyverno/kyverno/cmd/cleanup-controller/handlers/admission/resource"
	"github.com/kyverno/kyverno/cmd/internal"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers/certmanager"
	"github.com/kyverno/kyverno/pkg/controllers/cleanup"
	genericloggingcontroller "github.com/kyverno/kyverno/pkg/controllers/generic/logging"
	genericwebhookcontroller "github.com/kyverno/kyverno/pkg/controllers/generic/webhook"
	ttlcontroller "github.com/kyverno/kyverno/pkg/controllers/ttl"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/informers"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/tls"
	"github.com/kyverno/kyverno/pkg/webhooks"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
)

const (
	resyncPeriod                = 15 * time.Minute
	webhookWorkers              = 2
	policyWebhookControllerName = "policy-webhook-controller"
	ttlWebhookControllerName    = "ttl-webhook-controller"
)

var (
	caSecretName  string
	tlsSecretName string
)

// TODO:
// - helm review labels / selectors
// - implement probes
// - supports certs in cronjob

type probes struct{}

func (probes) IsReady(context.Context) bool {
	return true
}

func (probes) IsLive(context.Context) bool {
	return true
}

func main() {
	var (
		dumpPayload     bool
		serverIP        string
		servicePort     int
		maxQueuedEvents int
		interval        time.Duration
	)
	flagset := flag.NewFlagSet("cleanup-controller", flag.ExitOnError)
	flagset.BoolVar(&dumpPayload, "dumpPayload", false, "Set this flag to activate/deactivate debug mode.")
	flagset.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flagset.IntVar(&servicePort, "servicePort", 443, "Port used by the Kyverno Service resource and for webhook configurations.")
	flagset.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	flagset.DurationVar(&interval, "ttlReconciliationInterval", time.Minute, "Set this flag to set the interval after which the resource controller reconciliation should occur")
	flagset.StringVar(&caSecretName, "caSecretName", "", "Name of the secret containing CA.")
	flagset.StringVar(&tlsSecretName, "tlsSecretName", "", "Name of the secret containing TLS pair.")
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
		internal.WithMetadataClient(),
		internal.WithFlagSets(flagset),
	)
	// parse flags
	internal.ParseFlags(appConfig)
	// setup
	ctx, setup, sdown := internal.Setup(appConfig, "kyverno-cleanup-controller", false)
	defer sdown()
	if caSecretName == "" {
		setup.Logger.Error(errors.New("exiting... caSecretName is a required flag"), "exiting... caSecretName is a required flag")
		os.Exit(1)
	}
	if tlsSecretName == "" {
		setup.Logger.Error(errors.New("exiting... tlsSecretName is a required flag"), "exiting... tlsSecretName is a required flag")
		os.Exit(1)
	}
	// certificates informers
	caSecret := informers.NewSecretInformer(setup.KubeClient, config.KyvernoNamespace(), caSecretName, resyncPeriod)
	tlsSecret := informers.NewSecretInformer(setup.KubeClient, config.KyvernoNamespace(), tlsSecretName, resyncPeriod)
	if !informers.StartInformersAndWaitForCacheSync(ctx, setup.Logger, caSecret, tlsSecret) {
		setup.Logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
		os.Exit(1)
	}
	checker := checker.NewSelfChecker(setup.KubeClient.AuthorizationV1().SelfSubjectAccessReviews())
	// informer factories
	kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(setup.KubeClient, resyncPeriod)
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, resyncPeriod)
	// listers
	nsLister := kubeInformer.Core().V1().Namespaces().Lister()
	// log policy changes
	genericloggingcontroller.NewController(
		setup.Logger.WithName("cleanup-policy"),
		"CleanupPolicy",
		kyvernoInformer.Kyverno().V2beta1().CleanupPolicies(),
		genericloggingcontroller.CheckGeneration,
	)
	genericloggingcontroller.NewController(
		setup.Logger.WithName("cluster-cleanup-policy"),
		"ClusterCleanupPolicy",
		kyvernoInformer.Kyverno().V2beta1().ClusterCleanupPolicies(),
		genericloggingcontroller.CheckGeneration,
	)
	eventGenerator := event.NewEventCleanupGenerator(
		setup.KyvernoDynamicClient,
		kyvernoInformer.Kyverno().V2beta1().ClusterCleanupPolicies(),
		kyvernoInformer.Kyverno().V2beta1().CleanupPolicies(),
		maxQueuedEvents,
		logging.WithName("EventGenerator"),
	)
	// start informers and wait for cache sync
	if !internal.StartInformersAndWaitForCacheSync(ctx, setup.Logger, kubeInformer, kyvernoInformer) {
		os.Exit(1)
	}
	// start event generator
	var wg sync.WaitGroup
	go eventGenerator.Run(ctx, 3, &wg)
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

			cmResolver := internal.NewConfigMapResolver(ctx, setup.Logger, setup.KubeClient, resyncPeriod)

			// controllers
			renewer := tls.NewCertRenewer(
				setup.KubeClient.CoreV1().Secrets(config.KyvernoNamespace()),
				tls.CertRenewalInterval,
				tls.CAValidityDuration,
				tls.TLSValidityDuration,
				serverIP,
				config.KyvernoServiceName(),
				config.DnsNames(config.KyvernoServiceName(), config.KyvernoNamespace()),
				config.KyvernoNamespace(),
				caSecretName,
				tlsSecretName,
			)
			certController := internal.NewController(
				certmanager.ControllerName,
				certmanager.NewController(
					caSecret,
					tlsSecret,
					renewer,
					caSecretName,
					tlsSecretName,
					config.KyvernoNamespace(),
				),
				certmanager.Workers,
			)
			policyValidatingWebhookController := internal.NewController(
				policyWebhookControllerName,
				genericwebhookcontroller.NewController(
					policyWebhookControllerName,
					setup.KubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
					kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations(),
					caSecret,
					config.CleanupValidatingWebhookConfigurationName,
					config.CleanupValidatingWebhookServicePath,
					serverIP,
					int32(servicePort),
					nil,
					[]admissionregistrationv1.RuleWithOperations{
						{
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
						},
					},
					genericwebhookcontroller.Fail,
					genericwebhookcontroller.None,
					setup.Configuration,
					caSecretName,
				),
				webhookWorkers,
			)
			ttlWebhookController := internal.NewController(
				ttlWebhookControllerName,
				genericwebhookcontroller.NewController(
					ttlWebhookControllerName,
					setup.KubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
					kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations(),
					caSecret,
					config.TtlValidatingWebhookConfigurationName,
					config.TtlValidatingWebhookServicePath,
					serverIP,
					int32(servicePort),
					&metav1.LabelSelector{
						MatchExpressions: []metav1.LabelSelectorRequirement{
							{
								Key:      kyverno.LabelCleanupTtl,
								Operator: metav1.LabelSelectorOpExists,
							},
						},
					},
					[]admissionregistrationv1.RuleWithOperations{
						{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"*"},
								APIVersions: []string{"*"},
								Resources:   []string{"*"},
							},
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Create,
								admissionregistrationv1.Update,
							},
						},
					},
					genericwebhookcontroller.Ignore,
					genericwebhookcontroller.None,
					setup.Configuration,
					caSecretName,
				),
				webhookWorkers,
			)
			cleanupController := internal.NewController(
				cleanup.ControllerName,
				cleanup.NewController(
					setup.KyvernoDynamicClient,
					setup.KyvernoClient,
					kyvernoInformer.Kyverno().V2beta1().ClusterCleanupPolicies(),
					kyvernoInformer.Kyverno().V2beta1().CleanupPolicies(),
					nsLister,
					setup.Configuration,
					cmResolver,
					setup.Jp,
					eventGenerator,
				),
				cleanup.Workers,
			)
			ttlManagerController := internal.NewController(
				ttlcontroller.ControllerName,
				ttlcontroller.NewManager(
					setup.MetadataClient,
					setup.KubeClient.Discovery(),
					checker,
					interval,
				),
				ttlcontroller.Workers,
			)
			// start informers and wait for cache sync
			if !internal.StartInformersAndWaitForCacheSync(ctx, logger, kyvernoInformer, kubeInformer) {
				logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
				os.Exit(1)
			}
			// start leader controllers
			var wg sync.WaitGroup
			certController.Run(ctx, logger, &wg)
			policyValidatingWebhookController.Run(ctx, logger, &wg)
			ttlWebhookController.Run(ctx, logger, &wg)
			cleanupController.Run(ctx, logger, &wg)
			ttlManagerController.Run(ctx, logger, &wg)
			wg.Wait()
		},
		nil,
	)
	if err != nil {
		setup.Logger.Error(err, "failed to initialize leader election")
		os.Exit(1)
	}
	// create handlers
	policyHandlers := policyhandlers.New(setup.KyvernoDynamicClient)
	resourceHandlers := resourcehandlers.New(checker)
	// create server
	server := NewServer(
		func() ([]byte, []byte, error) {
			secret, err := tlsSecret.Lister().Secrets(config.KyvernoNamespace()).Get(tlsSecretName)
			if err != nil {
				return nil, nil, err
			}
			return secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey], nil
		},
		policyHandlers.Validate,
		resourceHandlers.Validate,
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
