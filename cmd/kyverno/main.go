package main

// We currently accept the risk of exposing pprof and rely on users to protect the endpoint.
import (
	"context"
	"errors"
	"flag"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/cmd/internal"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers/certmanager"
	genericloggingcontroller "github.com/kyverno/kyverno/pkg/controllers/generic/logging"
	genericwebhookcontroller "github.com/kyverno/kyverno/pkg/controllers/generic/webhook"
	policymetricscontroller "github.com/kyverno/kyverno/pkg/controllers/metrics/policy"
	openapicontroller "github.com/kyverno/kyverno/pkg/controllers/openapi"
	policycachecontroller "github.com/kyverno/kyverno/pkg/controllers/policycache"
	webhookcontroller "github.com/kyverno/kyverno/pkg/controllers/webhook"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/informers"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/tls"
	"github.com/kyverno/kyverno/pkg/toggle"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	runtimeutils "github.com/kyverno/kyverno/pkg/utils/runtime"
	"github.com/kyverno/kyverno/pkg/validation/exception"
	"github.com/kyverno/kyverno/pkg/webhooks"
	webhooksexception "github.com/kyverno/kyverno/pkg/webhooks/exception"
	webhookspolicy "github.com/kyverno/kyverno/pkg/webhooks/policy"
	webhooksresource "github.com/kyverno/kyverno/pkg/webhooks/resource"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apiserver "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kubeinformers "k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	kyamlopenapi "sigs.k8s.io/kustomize/kyaml/openapi"
)

const (
	resyncPeriod                   = 15 * time.Minute
	exceptionWebhookControllerName = "exception-webhook-controller"
)

func showWarnings(ctx context.Context, logger logr.Logger) {
	logger = logger.WithName("warnings")
	// log if `forceFailurePolicyIgnore` flag has been set or not
	if toggle.FromContext(ctx).ForceFailurePolicyIgnore() {
		logger.Info("'ForceFailurePolicyIgnore' is enabled, all policies with policy failures will be set to Ignore")
	}
}

func sanityChecks(apiserverClient apiserver.Interface) error {
	return kubeutils.CRDsInstalled(apiserverClient)
}

func createNonLeaderControllers(
	eng engineapi.Engine,
	kubeInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	kubeClient kubernetes.Interface,
	kyvernoClient versioned.Interface,
	dynamicClient dclient.Interface,
	configuration config.Configuration,
	policyCache policycache.Cache,
	manager openapi.Manager,
) ([]internal.Controller, func(context.Context) error) {
	policyCacheController := policycachecontroller.NewController(
		dynamicClient,
		policyCache,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
	)
	openApiController := openapicontroller.NewController(
		dynamicClient,
		manager,
	)
	return []internal.Controller{
			internal.NewController(policycachecontroller.ControllerName, policyCacheController, policycachecontroller.Workers),
			internal.NewController(openapicontroller.ControllerName, openApiController, openapicontroller.Workers),
		},
		func(ctx context.Context) error {
			if err := policyCacheController.WarmUp(); err != nil {
				return err
			}
			return nil
		}
}

func createrLeaderControllers(
	admissionReports bool,
	serverIP string,
	webhookTimeout int,
	autoUpdateWebhooks bool,
	kubeInformer kubeinformers.SharedInformerFactory,
	kubeKyvernoInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	caInformer corev1informers.SecretInformer,
	tlsInformer corev1informers.SecretInformer,
	kubeClient kubernetes.Interface,
	kyvernoClient versioned.Interface,
	dynamicClient dclient.Interface,
	certRenewer tls.CertRenewer,
	runtime runtimeutils.Runtime,
	servicePort int32,
	configuration config.Configuration,
) ([]internal.Controller, func(context.Context) error, error) {
	certManager := certmanager.NewController(
		caInformer,
		tlsInformer,
		certRenewer,
	)
	webhookController := webhookcontroller.NewController(
		dynamicClient.Discovery(),
		kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations(),
		kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
		kubeClient.CoordinationV1().Leases(config.KyvernoNamespace()),
		kyvernoClient,
		kubeInformer.Admissionregistration().V1().MutatingWebhookConfigurations(),
		kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations(),
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		caInformer,
		kubeKyvernoInformer.Coordination().V1().Leases(),
		kubeInformer.Rbac().V1().ClusterRoles(),
		serverIP,
		int32(webhookTimeout),
		servicePort,
		autoUpdateWebhooks,
		admissionReports,
		runtime,
		configuration,
	)
	exceptionWebhookController := genericwebhookcontroller.NewController(
		exceptionWebhookControllerName,
		kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
		kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations(),
		caInformer,
		config.ExceptionValidatingWebhookConfigurationName,
		config.ExceptionValidatingWebhookServicePath,
		serverIP,
		servicePort,
		nil,
		[]admissionregistrationv1.RuleWithOperations{{
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{"kyverno.io"},
				APIVersions: []string{"v2alpha1"},
				Resources:   []string{"policyexceptions"},
			},
			Operations: []admissionregistrationv1.OperationType{
				admissionregistrationv1.Create,
				admissionregistrationv1.Update,
			},
		}},
		genericwebhookcontroller.Fail,
		genericwebhookcontroller.None,
		configuration,
	)
	return []internal.Controller{
			internal.NewController(certmanager.ControllerName, certManager, certmanager.Workers),
			internal.NewController(webhookcontroller.ControllerName, webhookController, webhookcontroller.Workers),
			internal.NewController(exceptionWebhookControllerName, exceptionWebhookController, 1),
		},
		nil,
		nil
}

func main() {
	var (
		// TODO: this has been added to backward support command line arguments
		// will be removed in future and the configuration will be set only via configmaps
		serverIP                     string
		webhookTimeout               int
		maxQueuedEvents              int
		omitEvents                   string
		autoUpdateWebhooks           bool
		webhookRegistrationTimeout   time.Duration
		admissionReports             bool
		dumpPayload                  bool
		servicePort                  int
		backgroundServiceAccountName string
	)
	flagset := flag.NewFlagSet("kyverno", flag.ExitOnError)
	flagset.BoolVar(&dumpPayload, "dumpPayload", false, "Set this flag to activate/deactivate debug mode.")
	flagset.IntVar(&webhookTimeout, "webhookTimeout", webhookcontroller.DefaultWebhookTimeout, "Timeout for webhook configurations.")
	flagset.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	flagset.StringVar(&omitEvents, "omit-events", "", "Set this flag to a comma sperated list of PolicyViolation, PolicyApplied, PolicyError, PolicySkipped to disable events, e.g. --omit-events=PolicyApplied,PolicyViolation")
	flagset.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flagset.BoolVar(&autoUpdateWebhooks, "autoUpdateWebhooks", true, "Set this flag to 'false' to disable auto-configuration of the webhook.")
	flagset.DurationVar(&webhookRegistrationTimeout, "webhookRegistrationTimeout", 120*time.Second, "Timeout for webhook registration, e.g., 30s, 1m, 5m.")
	flagset.Func(toggle.ProtectManagedResourcesFlagName, toggle.ProtectManagedResourcesDescription, toggle.ProtectManagedResources.Parse)
	flagset.Func(toggle.ForceFailurePolicyIgnoreFlagName, toggle.ForceFailurePolicyIgnoreDescription, toggle.ForceFailurePolicyIgnore.Parse)
	flagset.BoolVar(&admissionReports, "admissionReports", true, "Enable or disable admission reports.")
	flagset.IntVar(&servicePort, "servicePort", 443, "Port used by the Kyverno Service resource and for webhook configurations.")
	flagset.StringVar(&backgroundServiceAccountName, "backgroundServiceAccountName", "", "Background service account name.")
	// config
	appConfig := internal.NewConfiguration(
		internal.WithProfiling(),
		internal.WithTracing(),
		internal.WithMetrics(),
		internal.WithKubeconfig(),
		internal.WithPolicyExceptions(),
		internal.WithConfigMapCaching(),
		internal.WithDeferredLoading(),
		internal.WithCosign(),
		internal.WithRegistryClient(),
		internal.WithImageVerifyCache(),
		internal.WithLeaderElection(),
		internal.WithKyvernoClient(),
		internal.WithDynamicClient(),
		internal.WithKyvernoDynamicClient(),
		internal.WithApiServerClient(),
		internal.WithFlagSets(flagset),
	)
	// parse flags
	internal.ParseFlags(appConfig)
	// setup
	signalCtx, setup, sdown := internal.Setup(appConfig, "kyverno-admission-controller", false)
	defer sdown()
	caSecret := informers.NewSecretInformer(setup.KubeClient, config.KyvernoNamespace(), config.GenerateRootCASecretName(), resyncPeriod)
	tlsSecret := informers.NewSecretInformer(setup.KubeClient, config.KyvernoNamespace(), config.GenerateTLSPairSecretName(), resyncPeriod)
	if !informers.StartInformersAndWaitForCacheSync(signalCtx, setup.Logger, caSecret, tlsSecret) {
		setup.Logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
		os.Exit(1)
	}
	// show version
	showWarnings(signalCtx, setup.Logger)
	// THIS IS AN UGLY FIX
	// ELSE KYAML IS NOT THREAD SAFE
	kyamlopenapi.Schema()
	// check we can run
	if err := sanityChecks(setup.ApiServerClient); err != nil {
		setup.Logger.Error(err, "sanity checks failed")
		os.Exit(1)
	}
	// informer factories
	kubeInformer := kubeinformers.NewSharedInformerFactory(setup.KubeClient, resyncPeriod)
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(setup.KubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, resyncPeriod)
	openApiManager, err := openapi.NewManager(setup.Logger.WithName("openapi"))
	if err != nil {
		setup.Logger.Error(err, "Failed to create openapi manager")
		os.Exit(1)
	}
	var wg sync.WaitGroup
	certRenewer := tls.NewCertRenewer(
		setup.KubeClient.CoreV1().Secrets(config.KyvernoNamespace()),
		tls.CertRenewalInterval,
		tls.CAValidityDuration,
		tls.TLSValidityDuration,
		serverIP,
		config.KyvernoServiceName(),
		config.DnsNames(),
		config.KyvernoNamespace(),
		config.GenerateRootCASecretName(),
		config.GenerateTLSPairSecretName(),
	)
	policyCache := policycache.NewCache()
	omitEventsValues := strings.Split(omitEvents, ",")
	if omitEvents == "" {
		omitEventsValues = []string{}
	}
	eventGenerator := event.NewEventGenerator(
		setup.KyvernoDynamicClient,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		maxQueuedEvents,
		omitEventsValues,
		logging.WithName("EventGenerator"),
	)
	// this controller only subscribe to events, nothing is returned...
	policymetricscontroller.NewController(
		setup.MetricsManager,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		&wg,
	)
	// log policy changes
	genericloggingcontroller.NewController(
		setup.Logger.WithName("policy"),
		"Policy",
		kyvernoInformer.Kyverno().V1().Policies(),
		genericloggingcontroller.CheckGeneration,
	)
	genericloggingcontroller.NewController(
		setup.Logger.WithName("cluster-policy"),
		"ClusterPolicy",
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		genericloggingcontroller.CheckGeneration,
	)
	runtime := runtimeutils.NewRuntime(
		setup.Logger.WithName("runtime-checks"),
		serverIP,
		kubeKyvernoInformer.Apps().V1().Deployments(),
		certRenewer,
	)
	// engine
	engine := internal.NewEngine(
		signalCtx,
		setup.Logger,
		setup.Configuration,
		setup.MetricsConfiguration,
		setup.Jp,
		setup.KyvernoDynamicClient,
		setup.RegistryClient,
		setup.ImageVerifyCacheClient,
		setup.KubeClient,
		setup.KyvernoClient,
		setup.RegistrySecretLister,
	)
	// create non leader controllers
	nonLeaderControllers, nonLeaderBootstrap := createNonLeaderControllers(
		engine,
		kubeInformer,
		kyvernoInformer,
		setup.KubeClient,
		setup.KyvernoClient,
		setup.KyvernoDynamicClient,
		setup.Configuration,
		policyCache,
		openApiManager,
	)
	// start informers and wait for cache sync
	if !internal.StartInformersAndWaitForCacheSync(signalCtx, setup.Logger, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
		setup.Logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
		os.Exit(1)
	}
	// bootstrap non leader controllers
	if nonLeaderBootstrap != nil {
		if err := nonLeaderBootstrap(signalCtx); err != nil {
			setup.Logger.Error(err, "failed to bootstrap non leader controllers")
			os.Exit(1)
		}
	}
	// start event generator
	go eventGenerator.Run(signalCtx, 3, &wg)
	// setup leader election
	le, err := leaderelection.New(
		setup.Logger.WithName("leader-election"),
		"kyverno",
		config.KyvernoNamespace(),
		setup.LeaderElectionClient,
		config.KyvernoPodName(),
		internal.LeaderElectionRetryPeriod(),
		func(ctx context.Context) {
			logger := setup.Logger.WithName("leader")
			// create leader factories
			kubeInformer := kubeinformers.NewSharedInformerFactory(setup.KubeClient, resyncPeriod)
			kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(setup.KyvernoClient, resyncPeriod)
			// create leader controllers
			leaderControllers, warmup, err := createrLeaderControllers(
				admissionReports,
				serverIP,
				webhookTimeout,
				autoUpdateWebhooks,
				kubeInformer,
				kubeKyvernoInformer,
				kyvernoInformer,
				caSecret,
				tlsSecret,
				setup.KubeClient,
				setup.KyvernoClient,
				setup.KyvernoDynamicClient,
				certRenewer,
				runtime,
				int32(servicePort),
				setup.Configuration,
			)
			if err != nil {
				logger.Error(err, "failed to create leader controllers")
				os.Exit(1)
			}
			// start informers and wait for cache sync
			if !internal.StartInformersAndWaitForCacheSync(signalCtx, logger, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
				logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
				os.Exit(1)
			}
			if warmup != nil {
				if err := warmup(ctx); err != nil {
					logger.Error(err, "failed to run warmup")
					os.Exit(1)
				}
			}
			// start leader controllers
			var wg sync.WaitGroup
			for _, controller := range leaderControllers {
				controller.Run(signalCtx, logger.WithName("controllers"), &wg)
			}
			// wait all controllers shut down
			wg.Wait()
		},
		nil,
	)
	if err != nil {
		setup.Logger.Error(err, "failed to initialize leader election")
		os.Exit(1)
	}
	// start non leader controllers
	for _, controller := range nonLeaderControllers {
		controller.Run(signalCtx, setup.Logger.WithName("controllers"), &wg)
	}
	// start leader election
	go func() {
		select {
		case <-signalCtx.Done():
			return
		default:
			le.Run(signalCtx)
		}
	}()
	// create webhooks server
	urgen := webhookgenerate.NewGenerator(
		setup.KyvernoClient,
		kyvernoInformer.Kyverno().V1beta1().UpdateRequests(),
	)
	policyHandlers := webhookspolicy.NewHandlers(
		setup.KyvernoDynamicClient,
		openApiManager,
		backgroundServiceAccountName,
	)
	resourceHandlers := webhooksresource.NewHandlers(
		engine,
		setup.KyvernoDynamicClient,
		setup.KyvernoClient,
		setup.Configuration,
		setup.MetricsManager,
		policyCache,
		kubeInformer.Core().V1().Namespaces().Lister(),
		kyvernoInformer.Kyverno().V1beta1().UpdateRequests().Lister().UpdateRequests(config.KyvernoNamespace()),
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		urgen,
		eventGenerator,
		openApiManager,
		admissionReports,
		backgroundServiceAccountName,
		setup.Jp,
	)
	exceptionHandlers := webhooksexception.NewHandlers(exception.ValidationOptions{
		Enabled:   internal.PolicyExceptionEnabled(),
		Namespace: internal.ExceptionNamespace(),
	})
	server := webhooks.NewServer(
		signalCtx,
		policyHandlers,
		resourceHandlers,
		exceptionHandlers,
		setup.Configuration,
		setup.MetricsManager,
		webhooks.DebugModeOptions{
			DumpPayload: dumpPayload,
		},
		func() ([]byte, []byte, error) {
			secret, err := tlsSecret.Lister().Secrets(config.KyvernoNamespace()).Get(config.GenerateTLSPairSecretName())
			if err != nil {
				return nil, nil, err
			}
			return secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey], nil
		},
		setup.KubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations(),
		setup.KubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
		setup.KubeClient.CoordinationV1().Leases(config.KyvernoNamespace()),
		runtime,
		kubeInformer.Rbac().V1().RoleBindings().Lister(),
		kubeInformer.Rbac().V1().ClusterRoleBindings().Lister(),
		setup.KyvernoDynamicClient.Discovery(),
	)
	// start informers and wait for cache sync
	// we need to call start again because we potentially registered new informers
	if !internal.StartInformersAndWaitForCacheSync(signalCtx, setup.Logger, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
		setup.Logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
		os.Exit(1)
	}
	// start webhooks server
	server.Run(signalCtx.Done())
	wg.Wait()
	// say goodbye...
	setup.Logger.V(2).Info("Kyverno shutdown successful")
}
