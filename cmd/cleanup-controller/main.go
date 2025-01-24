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
	globalcontextcontroller "github.com/kyverno/kyverno/pkg/controllers/globalcontext"
	ttlcontroller "github.com/kyverno/kyverno/pkg/controllers/ttl"
	webhookcontroller "github.com/kyverno/kyverno/pkg/controllers/webhook"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/kyverno/pkg/informers"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/tls"
	"github.com/kyverno/kyverno/pkg/toggle"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	runtimeutils "github.com/kyverno/kyverno/pkg/utils/runtime"
	"github.com/kyverno/kyverno/pkg/webhooks"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apiserver "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
)

const (
	webhookWorkers                       = 2
	policyWebhookControllerName          = "policy-webhook-controller"
	ttlWebhookControllerName             = "ttl-webhook-controller"
	policyWebhookControllerFinalizerName = "kyverno.io/policywebhooks"
	ttlWebhookControllerFinalizerName    = "kyverno.io/ttlwebhooks"
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

func sanityChecks(apiserverClient apiserver.Interface) error {
	return kubeutils.CRDsInstalled(apiserverClient, "cleanuppolicies.kyverno.io", "clustercleanuppolicies.kyverno.io")
}

func main() {
	var (
		dumpPayload              bool
		serverIP                 string
		servicePort              int
		webhookServerPort        int
		maxQueuedEvents          int
		interval                 time.Duration
		renewBefore              time.Duration
		maxAPICallResponseLength int64
		autoDeleteWebhooks       bool
	)
	flagset := flag.NewFlagSet("cleanup-controller", flag.ExitOnError)
	flagset.BoolVar(&dumpPayload, "dumpPayload", false, "Set this flag to activate/deactivate debug mode.")
	flagset.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flagset.IntVar(&servicePort, "servicePort", 443, "Port used by the Kyverno Service resource and for webhook configurations.")
	flagset.IntVar(&webhookServerPort, "webhookServerPort", 9443, "Port used by the webhook server.")
	flagset.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	flagset.DurationVar(&interval, "ttlReconciliationInterval", time.Minute, "Set this flag to set the interval after which the resource controller reconciliation should occur")
	flagset.Func(toggle.ProtectManagedResourcesFlagName, toggle.ProtectManagedResourcesDescription, toggle.ProtectManagedResources.Parse)
	flagset.StringVar(&caSecretName, "caSecretName", "", "Name of the secret containing CA.")
	flagset.StringVar(&tlsSecretName, "tlsSecretName", "", "Name of the secret containing TLS pair.")
	flagset.DurationVar(&renewBefore, "renewBefore", 15*24*time.Hour, "The certificate renewal time before expiration")
	flagset.Int64Var(&maxAPICallResponseLength, "maxAPICallResponseLength", 2*1000*1000, "Maximum allowed response size from API Calls. A value of 0 bypasses checks (not recommended).")
	flagset.BoolVar(&autoDeleteWebhooks, "autoDeleteWebhooks", false, "Set this flag to 'true' to enable autodeletion of webhook configurations using finalizers (requires extra permissions).")
	// config
	appConfig := internal.NewConfiguration(
		internal.WithProfiling(),
		internal.WithMetrics(),
		internal.WithTracing(),
		internal.WithKubeconfig(),
		internal.WithLeaderElection(),
		internal.WithKyvernoClient(),
		internal.WithKyvernoDynamicClient(),
		internal.WithEventsClient(),
		internal.WithConfigMapCaching(),
		internal.WithDeferredLoading(),
		internal.WithMetadataClient(),
		internal.WithApiServerClient(),
		internal.WithFlagSets(flagset),
	)
	// parse flags
	internal.ParseFlags(appConfig)
	var wg sync.WaitGroup
	func() {
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
		if err := sanityChecks(setup.ApiServerClient); err != nil {
			setup.Logger.Error(err, "sanity checks failed")
			os.Exit(1)
		}
		// certificates informers
		caSecret := informers.NewSecretInformer(setup.KubeClient, config.KyvernoNamespace(), caSecretName, setup.ResyncPeriod)
		tlsSecret := informers.NewSecretInformer(setup.KubeClient, config.KyvernoNamespace(), tlsSecretName, setup.ResyncPeriod)
		kyvernoDeployment := informers.NewDeploymentInformer(setup.KubeClient, config.KyvernoNamespace(), config.KyvernoDeploymentName(), setup.ResyncPeriod)
		if !informers.StartInformersAndWaitForCacheSync(ctx, setup.Logger, caSecret, tlsSecret, kyvernoDeployment) {
			setup.Logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
			os.Exit(1)
		}
		checker := checker.NewSelfChecker(setup.KubeClient.AuthorizationV1().SelfSubjectAccessReviews())
		// informer factories
		kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(setup.KubeClient, setup.ResyncPeriod)
		kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, setup.ResyncPeriod)
		// listers
		nsLister := kubeInformer.Core().V1().Namespaces().Lister()
		// log policy changes
		genericloggingcontroller.NewController(
			setup.Logger.WithName("cleanup-policy"),
			"CleanupPolicy",
			kyvernoInformer.Kyverno().V2().CleanupPolicies(),
			genericloggingcontroller.CheckGeneration,
		)
		genericloggingcontroller.NewController(
			setup.Logger.WithName("cluster-cleanup-policy"),
			"ClusterCleanupPolicy",
			kyvernoInformer.Kyverno().V2().ClusterCleanupPolicies(),
			genericloggingcontroller.CheckGeneration,
		)
		eventGenerator := event.NewEventGenerator(
			setup.EventsClient,
			logging.WithName("EventGenerator"),
			maxQueuedEvents,
		)
		eventController := internal.NewController(
			event.ControllerName,
			eventGenerator,
			event.Workers,
		)
		gcstore := store.New()
		gceController := internal.NewController(
			globalcontextcontroller.ControllerName,
			globalcontextcontroller.NewController(
				kyvernoInformer.Kyverno().V2alpha1().GlobalContextEntries(),
				setup.KyvernoDynamicClient,
				setup.KyvernoClient,
				gcstore,
				eventGenerator,
				maxAPICallResponseLength,
				false,
			),
			globalcontextcontroller.Workers,
		)
		// start informers and wait for cache sync
		if !internal.StartInformersAndWaitForCacheSync(ctx, setup.Logger, kubeInformer, kyvernoInformer) {
			os.Exit(1)
		}
		runtime := runtimeutils.NewRuntime(
			setup.Logger.WithName("runtime-checks"),
			serverIP,
			kyvernoDeployment,
			nil,
		)
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
				kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(setup.KubeClient, setup.ResyncPeriod)
				kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, setup.ResyncPeriod)

				cmResolver := internal.NewConfigMapResolver(ctx, setup.Logger, setup.KubeClient, setup.ResyncPeriod)

				// controllers
				renewer := tls.NewCertRenewer(
					setup.KubeClient.CoreV1().Secrets(config.KyvernoNamespace()),
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
						kyvernoDeployment,
						config.CleanupValidatingWebhookConfigurationName,
						config.CleanupValidatingWebhookServicePath,
						serverIP,
						int32(servicePort),       //nolint:gosec
						int32(webhookServerPort), //nolint:gosec
						nil,
						[]admissionregistrationv1.RuleWithOperations{
							{
								Rule: admissionregistrationv1.Rule{
									APIGroups:   []string{"kyverno.io"},
									APIVersions: []string{"v2beta1"},
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
						runtime,
						autoDeleteWebhooks,
						webhookcontroller.WebhookCleanupSetup(setup.KubeClient, policyWebhookControllerFinalizerName),
						webhookcontroller.WebhookCleanupHandler(setup.KubeClient, policyWebhookControllerFinalizerName),
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
						kyvernoDeployment,
						config.TtlValidatingWebhookConfigurationName,
						config.TtlValidatingWebhookServicePath,
						serverIP,
						int32(servicePort),       //nolint:gosec
						int32(webhookServerPort), //nolint:gosec
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
						runtime,
						autoDeleteWebhooks,
						webhookcontroller.WebhookCleanupSetup(setup.KubeClient, ttlWebhookControllerFinalizerName),
						webhookcontroller.WebhookCleanupHandler(setup.KubeClient, ttlWebhookControllerFinalizerName),
					),
					webhookWorkers,
				)
				cleanupController := internal.NewController(
					cleanup.ControllerName,
					cleanup.NewController(
						setup.KyvernoDynamicClient,
						setup.KyvernoClient,
						kyvernoInformer.Kyverno().V2().ClusterCleanupPolicies(),
						kyvernoInformer.Kyverno().V2().CleanupPolicies(),
						nsLister,
						setup.Configuration,
						cmResolver,
						setup.Jp,
						eventGenerator,
						gcstore,
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
						setup.ResyncPeriod,
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
		server.Run()
		defer server.Stop()
		// start non leader controllers
		eventController.Run(ctx, setup.Logger, &wg)
		gceController.Run(ctx, setup.Logger, &wg)
		// start leader election
		le.Run(ctx)
	}()
	// wait for everything to shut down and exit
	wg.Wait()
}
