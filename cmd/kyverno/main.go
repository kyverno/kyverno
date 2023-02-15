package main

// We currently accept the risk of exposing pprof and rely on users to protect the endpoint.
import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/cmd/internal"
	"github.com/kyverno/kyverno/pkg/background"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	dynamicclient "github.com/kyverno/kyverno/pkg/clients/dynamic"
	kubeclient "github.com/kyverno/kyverno/pkg/clients/kube"
	kyvernoclient "github.com/kyverno/kyverno/pkg/clients/kyverno"
	metadataclient "github.com/kyverno/kyverno/pkg/clients/metadata"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers/certmanager"
	configcontroller "github.com/kyverno/kyverno/pkg/controllers/config"
	genericwebhookcontroller "github.com/kyverno/kyverno/pkg/controllers/generic/webhook"
	policymetricscontroller "github.com/kyverno/kyverno/pkg/controllers/metrics/policy"
	openapicontroller "github.com/kyverno/kyverno/pkg/controllers/openapi"
	policycachecontroller "github.com/kyverno/kyverno/pkg/controllers/policycache"
	admissionreportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/admission"
	aggregatereportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/aggregate"
	backgroundscancontroller "github.com/kyverno/kyverno/pkg/controllers/report/background"
	resourcereportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/resource"
	webhookcontroller "github.com/kyverno/kyverno/pkg/controllers/webhook"
	"github.com/kyverno/kyverno/pkg/cosign"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context/resolvers"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/registryclient"
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
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	kyamlopenapi "sigs.k8s.io/kustomize/kyaml/openapi"
)

const (
	resyncPeriod                   = 15 * time.Minute
	exceptionWebhookControllerName = "exception-webhook-controller"
)

func setupRegistryClient(ctx context.Context, logger logr.Logger, lister corev1listers.SecretNamespaceLister, imagePullSecrets string, allowInsecureRegistry bool) (registryclient.Client, error) {
	logger = logger.WithName("registry-client")
	logger.Info("setup registry client...", "secrets", imagePullSecrets, "insecure", allowInsecureRegistry)
	registryOptions := []registryclient.Option{
		registryclient.WithTracing(),
	}
	secrets := strings.Split(imagePullSecrets, ",")
	if imagePullSecrets != "" && len(secrets) > 0 {
		registryOptions = append(registryOptions, registryclient.WithKeychainPullSecrets(ctx, lister, secrets...))
	}
	if allowInsecureRegistry {
		registryOptions = append(registryOptions, registryclient.WithAllowInsecureRegistry())
	}
	return registryclient.New(registryOptions...)
}

func setupCosign(logger logr.Logger, imageSignatureRepository string) {
	logger = logger.WithName("cosign")
	logger.Info("setup cosign...", "repository", imageSignatureRepository)
	if imageSignatureRepository != "" {
		cosign.ImageSignatureRepository = imageSignatureRepository
	}
}

func showWarnings(logger logr.Logger) {
	logger = logger.WithName("warnings")
	// log if `forceFailurePolicyIgnore` flag has been set or not
	if toggle.ForceFailurePolicyIgnore.Enabled() {
		logger.Info("'ForceFailurePolicyIgnore' is enabled, all policies with policy failures will be set to Ignore")
	}
}

func sanityChecks(apiserverClient apiserver.Interface) error {
	if !kubeutils.CRDsInstalled(apiserverClient) {
		return fmt.Errorf("CRDs not installed")
	}
	return nil
}

func createNonLeaderControllers(
	genWorkers int,
	kubeInformer kubeinformers.SharedInformerFactory,
	kubeKyvernoInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	kyvernoClient versioned.Interface,
	dynamicClient dclient.Interface,
	rclient registryclient.Client,
	configuration config.Configuration,
	policyCache policycache.Cache,
	eventGenerator event.Interface,
	manager openapi.Manager,
	informerCacheResolvers resolvers.ConfigmapResolver,
) ([]internal.Controller, func() error) {
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
	configurationController := configcontroller.NewController(
		configuration,
		kubeKyvernoInformer.Core().V1().ConfigMaps(),
	)

	return []internal.Controller{
			internal.NewController(policycachecontroller.ControllerName, policyCacheController, policycachecontroller.Workers),
			internal.NewController(openapicontroller.ControllerName, openApiController, openapicontroller.Workers),
			internal.NewController(configcontroller.ControllerName, configurationController, configcontroller.Workers),
		},
		func() error {
			return policyCacheController.WarmUp()
		}
}

func createReportControllers(
	backgroundScan bool,
	admissionReports bool,
	reportsChunkSize int,
	backgroundScanWorkers int,
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	rclient registryclient.Client,
	metadataFactory metadatainformers.SharedInformerFactory,
	kubeInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	configMapResolver resolvers.ConfigmapResolver,
	backgroundScanInterval time.Duration,
	configuration config.Configuration,
	eventGenerator event.Interface,
	enablePolicyException bool,
	exceptionNamespace string,
) ([]internal.Controller, func(context.Context) error) {
	var ctrls []internal.Controller
	var warmups []func(context.Context) error
	kyvernoV1 := kyvernoInformer.Kyverno().V1()
	kyvernoV2Alpha1 := kyvernoInformer.Kyverno().V2alpha1()
	if backgroundScan || admissionReports {
		resourceReportController := resourcereportcontroller.NewController(
			client,
			kyvernoV1.Policies(),
			kyvernoV1.ClusterPolicies(),
		)
		warmups = append(warmups, func(ctx context.Context) error {
			return resourceReportController.Warmup(ctx)
		})
		ctrls = append(ctrls, internal.NewController(
			resourcereportcontroller.ControllerName,
			resourceReportController,
			resourcereportcontroller.Workers,
		))
		ctrls = append(ctrls, internal.NewController(
			aggregatereportcontroller.ControllerName,
			aggregatereportcontroller.NewController(
				kyvernoClient,
				metadataFactory,
				kyvernoV1.Policies(),
				kyvernoV1.ClusterPolicies(),
				resourceReportController,
				reportsChunkSize,
			),
			aggregatereportcontroller.Workers,
		))
		if admissionReports {
			ctrls = append(ctrls, internal.NewController(
				admissionreportcontroller.ControllerName,
				admissionreportcontroller.NewController(
					kyvernoClient,
					metadataFactory,
					resourceReportController,
				),
				admissionreportcontroller.Workers,
			))
		}
		if backgroundScan {
			var exceptionsLister engine.PolicyExceptionLister
			if enablePolicyException {
				lister := kyvernoV2Alpha1.PolicyExceptions().Lister()
				if exceptionNamespace != "" {
					exceptionsLister = lister.PolicyExceptions(exceptionNamespace)
				} else {
					exceptionsLister = lister
				}
			}
			ctrls = append(ctrls, internal.NewController(
				backgroundscancontroller.ControllerName,
				backgroundscancontroller.NewController(
					client,
					kyvernoClient,
					rclient,
					metadataFactory,
					kyvernoV1.Policies(),
					kyvernoV1.ClusterPolicies(),
					kubeInformer.Core().V1().Namespaces(),
					exceptionsLister,
					resourceReportController,
					configMapResolver,
					backgroundScanInterval,
					configuration,
					eventGenerator,
				),
				backgroundScanWorkers,
			))
		}
	}
	return ctrls, func(ctx context.Context) error {
		for _, warmup := range warmups {
			if err := warmup(ctx); err != nil {
				return err
			}
		}
		return nil
	}
}

func createrLeaderControllers(
	backgroundScan bool,
	admissionReports bool,
	reportsChunkSize int,
	backgroundScanWorkers int,
	genWorkers int,
	serverIP string,
	webhookTimeout int,
	autoUpdateWebhooks bool,
	kubeInformer kubeinformers.SharedInformerFactory,
	kubeKyvernoInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	metadataInformer metadatainformers.SharedInformerFactory,
	kubeClient kubernetes.Interface,
	kyvernoClient versioned.Interface,
	dynamicClient dclient.Interface,
	rclient registryclient.Client,
	configuration config.Configuration,
	metricsConfig metrics.MetricsConfigManager,
	eventGenerator event.Interface,
	certRenewer tls.CertRenewer,
	runtime runtimeutils.Runtime,
	configMapResolver resolvers.ConfigmapResolver,
	backgroundScanInterval time.Duration,
	enablePolicyException bool,
	exceptionNamespace string,
) ([]internal.Controller, func(context.Context) error, error) {
	policyCtrl, err := policy.NewPolicyController(
		kyvernoClient,
		dynamicClient,
		rclient,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		kyvernoInformer.Kyverno().V1beta1().UpdateRequests(),
		configuration,
		eventGenerator,
		kubeInformer.Core().V1().Namespaces(),
		configMapResolver,
		logging.WithName("PolicyController"),
		time.Hour,
		metricsConfig,
	)
	if err != nil {
		return nil, nil, err
	}
	certManager := certmanager.NewController(
		kubeKyvernoInformer.Core().V1().Secrets(),
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
		kubeKyvernoInformer.Core().V1().Secrets(),
		kubeKyvernoInformer.Core().V1().ConfigMaps(),
		kubeKyvernoInformer.Coordination().V1().Leases(),
		kubeInformer.Rbac().V1().ClusterRoles(),
		serverIP,
		int32(webhookTimeout),
		autoUpdateWebhooks,
		admissionReports,
		runtime,
	)
	exceptionWebhookController := genericwebhookcontroller.NewController(
		exceptionWebhookControllerName,
		kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
		kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations(),
		kubeKyvernoInformer.Core().V1().Secrets(),
		config.ExceptionValidatingWebhookConfigurationName,
		config.ExceptionValidatingWebhookServicePath,
		serverIP,
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
	)
	reportControllers, warmup := createReportControllers(
		backgroundScan,
		admissionReports,
		reportsChunkSize,
		backgroundScanWorkers,
		dynamicClient,
		kyvernoClient,
		rclient,
		metadataInformer,
		kubeInformer,
		kyvernoInformer,
		configMapResolver,
		backgroundScanInterval,
		configuration,
		eventGenerator,
		enablePolicyException,
		exceptionNamespace,
	)
	backgroundController := background.NewController(
		kyvernoClient,
		dynamicClient,
		rclient,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		kyvernoInformer.Kyverno().V1beta1().UpdateRequests(),
		kubeInformer.Core().V1().Namespaces(),
		eventGenerator,
		configuration,
		configMapResolver,
	)

	return append(
			[]internal.Controller{
				internal.NewController("policy-controller", policyCtrl, 2),
				internal.NewController(certmanager.ControllerName, certManager, certmanager.Workers),
				internal.NewController(webhookcontroller.ControllerName, webhookController, webhookcontroller.Workers),
				internal.NewController(exceptionWebhookControllerName, exceptionWebhookController, 1),
				internal.NewController("background-controller", backgroundController, genWorkers),
			},
			reportControllers...,
		),
		warmup,
		nil
}

func main() {
	var (
		// TODO: this has been added to backward support command line arguments
		// will be removed in future and the configuration will be set only via configmaps
		serverIP                   string
		webhookTimeout             int
		genWorkers                 int
		maxQueuedEvents            int
		autoUpdateWebhooks         bool
		imagePullSecrets           string
		imageSignatureRepository   string
		allowInsecureRegistry      bool
		webhookRegistrationTimeout time.Duration
		backgroundScan             bool
		admissionReports           bool
		reportsChunkSize           int
		backgroundScanWorkers      int
		dumpPayload                bool
		leaderElectionRetryPeriod  time.Duration
		backgroundScanInterval     time.Duration
		enablePolicyException      bool
		exceptionNamespace         string
	)
	flagset := flag.NewFlagSet("kyverno", flag.ExitOnError)
	flagset.BoolVar(&dumpPayload, "dumpPayload", false, "Set this flag to activate/deactivate debug mode.")
	flagset.IntVar(&webhookTimeout, "webhookTimeout", webhookcontroller.DefaultWebhookTimeout, "Timeout for webhook configurations.")
	flagset.IntVar(&genWorkers, "genWorkers", 10, "Workers for generate controller.")
	flagset.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	flagset.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flagset.StringVar(&imagePullSecrets, "imagePullSecrets", "", "Secret resource names for image registry access credentials.")
	flagset.StringVar(&imageSignatureRepository, "imageSignatureRepository", "", "Alternate repository for image signatures. Can be overridden per rule via `verifyImages.Repository`.")
	flagset.BoolVar(&allowInsecureRegistry, "allowInsecureRegistry", false, "Whether to allow insecure connections to registries. Don't use this for anything but testing.")
	flagset.BoolVar(&autoUpdateWebhooks, "autoUpdateWebhooks", true, "Set this flag to 'false' to disable auto-configuration of the webhook.")
	flagset.DurationVar(&webhookRegistrationTimeout, "webhookRegistrationTimeout", 120*time.Second, "Timeout for webhook registration, e.g., 30s, 1m, 5m.")
	flagset.Func(toggle.ProtectManagedResourcesFlagName, toggle.ProtectManagedResourcesDescription, toggle.ProtectManagedResources.Parse)
	flagset.BoolVar(&backgroundScan, "backgroundScan", true, "Enable or disable backgound scan.")
	flagset.Func(toggle.ForceFailurePolicyIgnoreFlagName, toggle.ForceFailurePolicyIgnoreDescription, toggle.ForceFailurePolicyIgnore.Parse)
	flagset.BoolVar(&admissionReports, "admissionReports", true, "Enable or disable admission reports.")
	flagset.IntVar(&reportsChunkSize, "reportsChunkSize", 1000, "Max number of results in generated reports, reports will be split accordingly if there are more results to be stored.")
	flagset.IntVar(&backgroundScanWorkers, "backgroundScanWorkers", backgroundscancontroller.Workers, "Configure the number of background scan workers.")
	flagset.DurationVar(&leaderElectionRetryPeriod, "leaderElectionRetryPeriod", leaderelection.DefaultRetryPeriod, "Configure leader election retry period.")
	flagset.DurationVar(&backgroundScanInterval, "backgroundScanInterval", time.Hour, "Configure background scan interval.")
	flagset.StringVar(&exceptionNamespace, "exceptionNamespace", "", "Configure the namespace to accept PolicyExceptions.")
	flagset.BoolVar(&enablePolicyException, "enablePolicyException", false, "Enable PolicyException feature.")
	// config
	appConfig := internal.NewConfiguration(
		internal.WithProfiling(),
		internal.WithTracing(),
		internal.WithMetrics(),
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
	signalCtx, logger, metricsConfig, sdown := internal.Setup("kyverno")
	defer sdown()
	// show version
	showWarnings(logger)
	// create instrumented clients
	kubeClient := internal.CreateKubernetesClient(logger, kubeclient.WithMetrics(metricsConfig, metrics.KubeClient), kubeclient.WithTracing())
	leaderElectionClient := internal.CreateKubernetesClient(logger, kubeclient.WithMetrics(metricsConfig, metrics.KubeClient), kubeclient.WithTracing())
	kyvernoClient := internal.CreateKyvernoClient(logger, kyvernoclient.WithMetrics(metricsConfig, metrics.KyvernoClient), kyvernoclient.WithTracing())
	metadataClient := internal.CreateMetadataClient(logger, metadataclient.WithMetrics(metricsConfig, metrics.KyvernoClient), metadataclient.WithTracing())
	dynamicClient := internal.CreateDynamicClient(logger, dynamicclient.WithMetrics(metricsConfig, metrics.KyvernoClient), dynamicclient.WithTracing())
	dClient, err := dclient.NewClient(signalCtx, dynamicClient, kubeClient, 15*time.Minute)
	if err != nil {
		logger.Error(err, "failed to create dynamic client")
		os.Exit(1)
	}
	apiserverClient, err := apiserver.NewForConfig(internal.CreateClientConfig(logger))
	if err != nil {
		logger.Error(err, "failed to create apiserver client")
		os.Exit(1)
	}
	// THIS IS AN UGLY FIX
	// ELSE KYAML IS NOT THREAD SAFE
	kyamlopenapi.Schema()
	// check we can run
	if err := sanityChecks(apiserverClient); err != nil {
		logger.Error(err, "sanity checks failed")
		os.Exit(1)
	}
	// informer factories
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, resyncPeriod)
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
	cacheInformer, err := resolvers.GetCacheInformerFactory(kubeClient, resyncPeriod)
	if err != nil {
		logger.Error(err, "failed to create cache informer factory")
		os.Exit(1)
	}
	secretLister := kubeKyvernoInformer.Core().V1().Secrets().Lister().Secrets(config.KyvernoNamespace())
	// setup registry client
	rclient, err := setupRegistryClient(signalCtx, logger, secretLister, imagePullSecrets, allowInsecureRegistry)
	if err != nil {
		logger.Error(err, "failed to setup registry client")
		os.Exit(1)
	}
	// setup cosign
	setupCosign(logger, imageSignatureRepository)
	informerBasedResolver, err := resolvers.NewInformerBasedResolver(cacheInformer.Core().V1().ConfigMaps().Lister())
	if err != nil {
		logger.Error(err, "failed to create informer based resolver")
		os.Exit(1)
	}
	clientBasedResolver, err := resolvers.NewClientBasedResolver(kubeClient)
	if err != nil {
		logger.Error(err, "failed to create client based resolver")
		os.Exit(1)
	}
	configMapResolver, err := resolvers.NewResolverChain(informerBasedResolver, clientBasedResolver)
	if err != nil {
		logger.Error(err, "failed to create config map resolver")
		os.Exit(1)
	}
	configuration, err := config.NewConfiguration(kubeClient)
	if err != nil {
		logger.Error(err, "failed to initialize configuration")
		os.Exit(1)
	}
	openApiManager, err := openapi.NewManager()
	if err != nil {
		logger.Error(err, "Failed to create openapi manager")
		os.Exit(1)
	}
	certRenewer := tls.NewCertRenewer(
		kubeClient.CoreV1().Secrets(config.KyvernoNamespace()),
		secretLister,
		tls.CertRenewalInterval,
		tls.CAValidityDuration,
		tls.TLSValidityDuration,
		serverIP,
	)
	policyCache := policycache.NewCache()
	eventGenerator := event.NewEventGenerator(
		dClient,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		maxQueuedEvents,
		logging.WithName("EventGenerator"),
	)
	// this controller only subscribe to events, nothing is returned...
	policymetricscontroller.NewController(
		metricsConfig,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
	)
	runtime := runtimeutils.NewRuntime(
		logger.WithName("runtime-checks"),
		serverIP,
		kubeKyvernoInformer.Apps().V1().Deployments(),
		certRenewer,
	)
	// create non leader controllers
	nonLeaderControllers, nonLeaderBootstrap := createNonLeaderControllers(
		genWorkers,
		kubeInformer,
		kubeKyvernoInformer,
		kyvernoInformer,
		kyvernoClient,
		dClient,
		rclient,
		configuration,
		policyCache,
		eventGenerator,
		openApiManager,
		configMapResolver,
	)
	// start informers and wait for cache sync
	if !internal.StartInformersAndWaitForCacheSync(signalCtx, logger, kyvernoInformer, kubeInformer, kubeKyvernoInformer, cacheInformer) {
		logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
		os.Exit(1)
	}
	// bootstrap non leader controllers
	if nonLeaderBootstrap != nil {
		if err := nonLeaderBootstrap(); err != nil {
			logger.Error(err, "failed to bootstrap non leader controllers")
			os.Exit(1)
		}
	}
	// start event generator
	go eventGenerator.Run(signalCtx, 3)
	// setup leader election
	le, err := leaderelection.New(
		logger.WithName("leader-election"),
		"kyverno",
		config.KyvernoNamespace(),
		leaderElectionClient,
		config.KyvernoPodName(),
		leaderElectionRetryPeriod,
		func(ctx context.Context) {
			logger := logger.WithName("leader")
			// validate config
			// if err := webhookCfg.ValidateWebhookConfigurations(config.KyvernoNamespace(), config.KyvernoConfigMapName()); err != nil {
			// 	logger.Error(err, "invalid format of the Kyverno init ConfigMap, please correct the format of 'data.webhooks'")
			// 	os.Exit(1)
			// }
			// create leader factories
			kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, resyncPeriod)
			kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
			kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
			metadataInformer := metadatainformers.NewSharedInformerFactory(metadataClient, 15*time.Minute)
			// create leader controllers
			leaderControllers, warmup, err := createrLeaderControllers(
				backgroundScan,
				admissionReports,
				reportsChunkSize,
				backgroundScanWorkers,
				genWorkers,
				serverIP,
				webhookTimeout,
				autoUpdateWebhooks,
				kubeInformer,
				kubeKyvernoInformer,
				kyvernoInformer,
				metadataInformer,
				kubeClient,
				kyvernoClient,
				dClient,
				rclient,
				configuration,
				metricsConfig,
				eventGenerator,
				certRenewer,
				runtime,
				configMapResolver,
				backgroundScanInterval,
				enablePolicyException,
				exceptionNamespace,
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
			internal.StartInformers(signalCtx, metadataInformer)
			if !internal.CheckCacheSync(logger, metadataInformer.WaitForCacheSync(signalCtx.Done())) {
				// TODO: shall we just exit ?
				logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
			}
			if err := warmup(ctx); err != nil {
				logger.Error(err, "failed to run warmup")
				os.Exit(1)
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
		logger.Error(err, "failed to initialize leader election")
		os.Exit(1)
	}
	// start non leader controllers
	var wg sync.WaitGroup
	for _, controller := range nonLeaderControllers {
		controller.Run(signalCtx, logger.WithName("controllers"), &wg)
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
		kyvernoClient,
		kyvernoInformer.Kyverno().V1beta1().UpdateRequests(),
	)
	policyHandlers := webhookspolicy.NewHandlers(
		dClient,
		openApiManager,
	)
	var exceptionsLister engine.PolicyExceptionLister
	if enablePolicyException {
		lister := kyvernoInformer.Kyverno().V2alpha1().PolicyExceptions().Lister()
		if exceptionNamespace != "" {
			exceptionsLister = lister.PolicyExceptions(exceptionNamespace)
		} else {
			exceptionsLister = lister
		}
	}
	resourceHandlers := webhooksresource.NewHandlers(
		dClient,
		kyvernoClient,
		rclient,
		configuration,
		metricsConfig,
		policyCache,
		configMapResolver,
		kubeInformer.Core().V1().Namespaces().Lister(),
		kubeInformer.Rbac().V1().RoleBindings().Lister(),
		kubeInformer.Rbac().V1().ClusterRoleBindings().Lister(),
		kyvernoInformer.Kyverno().V1beta1().UpdateRequests().Lister().UpdateRequests(config.KyvernoNamespace()),
		exceptionsLister,
		urgen,
		eventGenerator,
		openApiManager,
		admissionReports,
	)
	exceptionHandlers := webhooksexception.NewHandlers(exception.ValidationOptions{
		Enabled:   enablePolicyException,
		Namespace: exceptionNamespace,
	})
	server := webhooks.NewServer(
		policyHandlers,
		resourceHandlers,
		exceptionHandlers,
		configuration,
		metricsConfig,
		webhooks.DebugModeOptions{
			DumpPayload: dumpPayload,
		},
		func() ([]byte, []byte, error) {
			secret, err := secretLister.Get(tls.GenerateTLSPairSecretName())
			if err != nil {
				return nil, nil, err
			}
			return secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey], nil
		},
		kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations(),
		kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
		kubeClient.CoordinationV1().Leases(config.KyvernoNamespace()),
		runtime,
		kubeInformer.Rbac().V1().RoleBindings().Lister(),
		kubeInformer.Rbac().V1().ClusterRoleBindings().Lister(),
	)
	// start informers and wait for cache sync
	// we need to call start again because we potentially registered new informers
	if !internal.StartInformersAndWaitForCacheSync(signalCtx, logger, kyvernoInformer, kubeInformer, kubeKyvernoInformer, cacheInformer) {
		logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
		os.Exit(1)
	}
	// start webhooks server
	server.Run(signalCtx.Done())
	wg.Wait()
	// say goodbye...
	logger.V(2).Info("Kyverno shutdown successful")
}
