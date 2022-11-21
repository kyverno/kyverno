package main

// We currently accept the risk of exposing pprof and rely on users to protect the endpoint.
import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/cmd/internal"
	"github.com/kyverno/kyverno/pkg/background"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	kubeclient "github.com/kyverno/kyverno/pkg/clients/kube"
	kyvernoclient "github.com/kyverno/kyverno/pkg/clients/kyverno"
	metadataclient "github.com/kyverno/kyverno/pkg/clients/metadata"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers/certmanager"
	configcontroller "github.com/kyverno/kyverno/pkg/controllers/config"
	policymetricscontroller "github.com/kyverno/kyverno/pkg/controllers/metrics/policy"
	openapicontroller "github.com/kyverno/kyverno/pkg/controllers/openapi"
	policycachecontroller "github.com/kyverno/kyverno/pkg/controllers/policycache"
	admissionreportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/admission"
	aggregatereportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/aggregate"
	backgroundscancontroller "github.com/kyverno/kyverno/pkg/controllers/report/background"
	resourcereportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/resource"
	webhookcontroller "github.com/kyverno/kyverno/pkg/controllers/webhook"
	"github.com/kyverno/kyverno/pkg/cosign"
	event "github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/kyverno/pkg/tls"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/kyverno/pkg/utils"
	runtimeutils "github.com/kyverno/kyverno/pkg/utils/runtime"
	"github.com/kyverno/kyverno/pkg/webhooks"
	webhookspolicy "github.com/kyverno/kyverno/pkg/webhooks/policy"
	webhooksresource "github.com/kyverno/kyverno/pkg/webhooks/resource"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	corev1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
)

const (
	resyncPeriod         = 15 * time.Minute
	metadataResyncPeriod = 15 * time.Minute
)

var (
	// TODO: this has been added to backward support command line arguments
	// will be removed in future and the configuration will be set only via configmaps
	kubeconfig                 string
	serverIP                   string
	metricsPort                string
	webhookTimeout             int
	genWorkers                 int
	maxQueuedEvents            int
	disableMetricsExport       bool
	otel                       string
	otelCollector              string
	transportCreds             string
	autoUpdateWebhooks         bool
	imagePullSecrets           string
	imageSignatureRepository   string
	allowInsecureRegistry      bool
	clientRateLimitQPS         float64
	clientRateLimitBurst       int
	webhookRegistrationTimeout time.Duration
	backgroundScan             bool
	admissionReports           bool
	reportsChunkSize           int
	backgroundScanWorkers      int
	dumpPayload                bool
	leaderElectionRetryPeriod  time.Duration
	// DEPRECATED: remove in 1.9
	splitPolicyReport bool
)

func parseFlags(config internal.Configuration) {
	internal.InitFlags(config)
	flag.BoolVar(&dumpPayload, "dumpPayload", false, "Set this flag to activate/deactivate debug mode.")
	flag.IntVar(&webhookTimeout, "webhookTimeout", webhookcontroller.DefaultWebhookTimeout, "Timeout for webhook configurations.")
	flag.IntVar(&genWorkers, "genWorkers", 10, "Workers for generate controller.")
	flag.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flag.BoolVar(&disableMetricsExport, "disableMetrics", false, "Set this flag to 'true' to disable metrics.")
	flag.StringVar(&otel, "otelConfig", "prometheus", "Set this flag to 'grpc', to enable exporting metrics to an Opentelemetry Collector. The default collector is set to \"prometheus\"")
	flag.StringVar(&otelCollector, "otelCollector", "opentelemetrycollector.kyverno.svc.cluster.local", "Set this flag to the OpenTelemetry Collector Service Address. Kyverno will try to connect to this on the metrics port.")
	flag.StringVar(&transportCreds, "transportCreds", "", "Set this flag to the CA secret containing the certificate which is used by our Opentelemetry Metrics Client. If empty string is set, means an insecure connection will be used")
	flag.StringVar(&metricsPort, "metricsPort", "8000", "Expose prometheus metrics at the given port, default to 8000.")
	flag.StringVar(&imagePullSecrets, "imagePullSecrets", "", "Secret resource names for image registry access credentials.")
	flag.StringVar(&imageSignatureRepository, "imageSignatureRepository", "", "Alternate repository for image signatures. Can be overridden per rule via `verifyImages.Repository`.")
	flag.BoolVar(&allowInsecureRegistry, "allowInsecureRegistry", false, "Whether to allow insecure connections to registries. Don't use this for anything but testing.")
	flag.BoolVar(&autoUpdateWebhooks, "autoUpdateWebhooks", true, "Set this flag to 'false' to disable auto-configuration of the webhook.")
	flag.Float64Var(&clientRateLimitQPS, "clientRateLimitQPS", 20, "Configure the maximum QPS to the Kubernetes API server from Kyverno. Uses the client default if zero.")
	flag.IntVar(&clientRateLimitBurst, "clientRateLimitBurst", 50, "Configure the maximum burst for throttle. Uses the client default if zero.")
	flag.DurationVar(&webhookRegistrationTimeout, "webhookRegistrationTimeout", 120*time.Second, "Timeout for webhook registration, e.g., 30s, 1m, 5m.")
	flag.Func(toggle.ProtectManagedResourcesFlagName, toggle.ProtectManagedResourcesDescription, toggle.ProtectManagedResources.Parse)
	flag.BoolVar(&backgroundScan, "backgroundScan", true, "Enable or disable backgound scan.")
	flag.Func(toggle.ForceFailurePolicyIgnoreFlagName, toggle.ForceFailurePolicyIgnoreDescription, toggle.ForceFailurePolicyIgnore.Parse)
	flag.BoolVar(&admissionReports, "admissionReports", true, "Enable or disable admission reports.")
	flag.IntVar(&reportsChunkSize, "reportsChunkSize", 1000, "Max number of results in generated reports, reports will be split accordingly if there are more results to be stored.")
	flag.IntVar(&backgroundScanWorkers, "backgroundScanWorkers", backgroundscancontroller.Workers, "Configure the number of background scan workers.")
	flag.DurationVar(&leaderElectionRetryPeriod, "leaderElectionRetryPeriod", leaderelection.DefaultRetryPeriod, "Configure leader election retry period.")
	// DEPRECATED: remove in 1.9
	flag.BoolVar(&splitPolicyReport, "splitPolicyReport", false, "This is deprecated, please don't use it, will be removed in v1.9.")
	flag.Parse()
}

func setupMetrics(logger logr.Logger, kubeClient kubernetes.Interface) (*metrics.MetricsConfig, context.CancelFunc, error) {
	logger = logger.WithName("metrics")
	logger.Info("setup metrics...", "otel", otel, "port", metricsPort, "collector", otelCollector, "creds", transportCreds)
	metricsConfigData, err := config.NewMetricsConfigData(kubeClient)
	if err != nil {
		return nil, nil, err
	}
	metricsAddr := ":" + metricsPort
	metricsConfig, metricsServerMux, metricsPusher, err := metrics.InitMetrics(
		disableMetricsExport,
		otel,
		metricsAddr,
		otelCollector,
		metricsConfigData,
		transportCreds,
		kubeClient,
		logging.WithName("metrics"),
	)
	if err != nil {
		return nil, nil, err
	}
	var cancel context.CancelFunc
	if otel == "grpc" {
		cancel = func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			metrics.ShutDownController(ctx, metricsPusher)
		}
	}
	if otel == "prometheus" {
		go func() {
			metricsServer := http.Server{
				Addr:              metricsAddr,
				Handler:           metricsServerMux,
				ErrorLog:          logging.StdLogger(logger, ""),
				ReadHeaderTimeout: 30 * time.Second,
			}
			if err := metricsServer.ListenAndServe(); err != nil {
				logger.Error(err, "failed to enable metrics", "address", metricsAddr)
				os.Exit(1)
			}
		}()
	}
	return metricsConfig, cancel, nil
}

func setupRegistryClient(logger logr.Logger, kubeClient kubernetes.Interface) error {
	logger = logger.WithName("registry-client")
	logger.Info("setup registry client...", "secrets", imagePullSecrets, "insecure", allowInsecureRegistry)
	var registryOptions []registryclient.Option
	secrets := strings.Split(imagePullSecrets, ",")
	if imagePullSecrets != "" && len(secrets) > 0 {
		registryOptions = append(registryOptions, registryclient.WithKeychainPullSecrets(kubeClient, config.KyvernoNamespace(), "", secrets))
	}
	if allowInsecureRegistry {
		registryOptions = append(registryOptions, registryclient.WithAllowInsecureRegistry())
	}
	client, err := registryclient.InitClient(registryOptions...)
	if err != nil {
		return err
	}
	registryclient.DefaultClient = client
	return nil
}

func setupCosign(logger logr.Logger) {
	logger = logger.WithName("cosign")
	logger.Info("setup cosign...", "repository", imageSignatureRepository)
	if imageSignatureRepository != "" {
		cosign.ImageSignatureRepository = imageSignatureRepository
	}
}

func setupSignals() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func showWarnings(logger logr.Logger) {
	logger = logger.WithName("warnings")
	// DEPRECATED: remove in 1.9
	if splitPolicyReport {
		logger.Info("The splitPolicyReport flag is deprecated and will be removed in v1.9. It has no effect and should be removed.")
	}
	// log if `forceFailurePolicyIgnore` flag has been set or not
	if toggle.ForceFailurePolicyIgnore.Enabled() {
		logger.Info("'ForceFailurePolicyIgnore' is enabled, all policies with policy failures will be set to Ignore")
	}
}

func sanityChecks(dynamicClient dclient.Interface) error {
	if !utils.CRDsInstalled(dynamicClient.Discovery()) {
		return fmt.Errorf("CRDs not installed")
	}
	return nil
}

func createNonLeaderControllers(
	kubeInformer kubeinformers.SharedInformerFactory,
	kubeKyvernoInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	kubeClient kubernetes.Interface,
	kyvernoClient versioned.Interface,
	dynamicClient dclient.Interface,
	configuration config.Configuration,
	policyCache policycache.Cache,
	eventGenerator event.Interface,
	manager openapi.Manager,
) ([]controller, func() error) {
	policyCacheController := policycachecontroller.NewController(
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
	updateRequestController := background.NewController(
		kyvernoClient,
		dynamicClient,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		kyvernoInformer.Kyverno().V1beta1().UpdateRequests(),
		kubeInformer.Core().V1().Namespaces(),
		kubeKyvernoInformer.Core().V1().Pods(),
		eventGenerator,
		configuration,
	)
	return []controller{
			newController(policycachecontroller.ControllerName, policyCacheController, policycachecontroller.Workers),
			newController(openapicontroller.ControllerName, openApiController, openapicontroller.Workers),
			newController(configcontroller.ControllerName, configurationController, configcontroller.Workers),
			newController("update-request-controller", updateRequestController, genWorkers),
		},
		func() error {
			return policyCacheController.WarmUp()
		}
}

func createReportControllers(
	backgroundScan bool,
	admissionReports bool,
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	metadataFactory metadatainformers.SharedInformerFactory,
	kubeInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
) []controller {
	var ctrls []controller
	kyvernoV1 := kyvernoInformer.Kyverno().V1()
	if backgroundScan || admissionReports {
		resourceReportController := resourcereportcontroller.NewController(
			client,
			kyvernoV1.Policies(),
			kyvernoV1.ClusterPolicies(),
		)
		ctrls = append(ctrls, newController(
			resourcereportcontroller.ControllerName,
			resourceReportController,
			resourcereportcontroller.Workers,
		))
		ctrls = append(ctrls, newController(
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
			ctrls = append(ctrls, newController(
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
			ctrls = append(ctrls, newController(
				backgroundscancontroller.ControllerName,
				backgroundscancontroller.NewController(
					client,
					kyvernoClient,
					metadataFactory,
					kyvernoV1.Policies(),
					kyvernoV1.ClusterPolicies(),
					kubeInformer.Core().V1().Namespaces(),
					resourceReportController,
				),
				backgroundScanWorkers,
			))
		}
	}
	return ctrls
}

func createrLeaderControllers(
	kubeInformer kubeinformers.SharedInformerFactory,
	kubeKyvernoInformer kubeinformers.SharedInformerFactory,
	kyvernoInformer kyvernoinformer.SharedInformerFactory,
	metadataInformer metadatainformers.SharedInformerFactory,
	kubeClient kubernetes.Interface,
	kyvernoClient versioned.Interface,
	dynamicClient dclient.Interface,
	configuration config.Configuration,
	metricsConfig *metrics.MetricsConfig,
	eventGenerator event.Interface,
	certRenewer tls.CertRenewer,
	runtime runtimeutils.Runtime,
) ([]controller, error) {
	policyCtrl, err := policy.NewPolicyController(
		kyvernoClient,
		dynamicClient,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		kyvernoInformer.Kyverno().V1beta1().UpdateRequests(),
		configuration,
		eventGenerator,
		kubeInformer.Core().V1().Namespaces(),
		logging.WithName("PolicyController"),
		time.Hour,
		metricsConfig,
	)
	if err != nil {
		return nil, err
	}
	certManager := certmanager.NewController(
		kubeKyvernoInformer.Core().V1().Secrets(),
		certRenewer,
	)
	webhookController := webhookcontroller.NewController(
		dynamicClient.Discovery(),
		kubeClient.CoreV1().Secrets(config.KyvernoNamespace()),
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
		serverIP,
		int32(webhookTimeout),
		autoUpdateWebhooks,
		admissionReports,
		runtime,
	)
	return append(
			[]controller{
				newController("policy-controller", policyCtrl, 2),
				newController(certmanager.ControllerName, certManager, certmanager.Workers),
				newController(webhookcontroller.ControllerName, webhookController, webhookcontroller.Workers),
			},
			createReportControllers(
				backgroundScan,
				admissionReports,
				dynamicClient,
				kyvernoClient,
				metadataInformer,
				kubeInformer,
				kyvernoInformer,
			)...,
		),
		nil
}

func main() {
	// config
	appConfig := internal.NewConfiguration(internal.WithProfiling(), internal.WithTracing())
	// parse flags
	parseFlags(appConfig)
	// setup logger
	logger := internal.SetupLogger()
	// setup maxprocs
	undo := internal.SetupMaxProcs(logger)
	defer undo()
	// show version
	showWarnings(logger)
	// show version
	internal.ShowVersion(logger)
	// start profiling
	internal.SetupProfiling(logger)
	// create raw client
	rawClient := internal.CreateKubernetesClient(logger)
	// setup metrics
	metricsConfig, metricsShutdown, err := setupMetrics(logger, rawClient)
	if err != nil {
		logger.Error(err, "failed to setup metrics")
		os.Exit(1)
	}
	if metricsShutdown != nil {
		defer metricsShutdown()
	}
	// setup signals
	signalCtx, signalCancel := setupSignals()
	defer signalCancel()
	// create instrumented clients
	kubeClient := internal.CreateKubernetesClient(logger, kubeclient.WithMetrics(metricsConfig, metrics.KubeClient), kubeclient.WithTracing())
	leaderElectionClient := internal.CreateKubernetesClient(logger, kubeclient.WithMetrics(metricsConfig, metrics.KubeClient), kubeclient.WithTracing())
	kyvernoClient := internal.CreateKyvernoClient(logger, kyvernoclient.WithMetrics(metricsConfig, metrics.KyvernoClient), kyvernoclient.WithTracing())
	metadataClient := internal.CreateMetadataClient(logger, metadataclient.WithMetrics(metricsConfig, metrics.KyvernoClient), metadataclient.WithTracing())
	dynamicClient, err := dclient.NewClient(signalCtx, internal.CreateClientConfig(logger), kubeClient, metricsConfig, 15*time.Minute)
	if err != nil {
		logger.Error(err, "failed to create dynamic client")
		os.Exit(1)
	}
	// setup tracing
	tracingShutdown := internal.SetupTracing(logger, "kyverno", kubeClient)
	defer tracingShutdown()
	// setup registry client
	if err := setupRegistryClient(logger, kubeClient); err != nil {
		logger.Error(err, "failed to setup registry client")
		os.Exit(1)
	}
	// setup cosign
	setupCosign(logger)
	// check we can run
	if err := sanityChecks(dynamicClient); err != nil {
		logger.Error(err, "sanity checks failed")
		os.Exit(1)
	}
	// informer factories
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, resyncPeriod)
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
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
		tls.CertRenewalInterval,
		tls.CAValidityDuration,
		tls.TLSValidityDuration,
		serverIP,
	)
	policyCache := policycache.NewCache()
	eventGenerator := event.NewEventGenerator(
		dynamicClient,
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
		kubeInformer,
		kubeKyvernoInformer,
		kyvernoInformer,
		kubeClient,
		kyvernoClient,
		dynamicClient,
		configuration,
		policyCache,
		eventGenerator,
		openApiManager,
	)
	// start informers and wait for cache sync
	if !internal.StartInformersAndWaitForCacheSync(signalCtx, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
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
			leaderControllers, err := createrLeaderControllers(
				kubeInformer,
				kubeKyvernoInformer,
				kyvernoInformer,
				metadataInformer,
				kubeClient,
				kyvernoClient,
				dynamicClient,
				configuration,
				metricsConfig,
				eventGenerator,
				certRenewer,
				runtime,
			)
			if err != nil {
				logger.Error(err, "failed to create leader controllers")
				os.Exit(1)
			}
			// start informers and wait for cache sync
			if !internal.StartInformersAndWaitForCacheSync(signalCtx, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
				logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
				os.Exit(1)
			}
			internal.StartInformers(signalCtx, metadataInformer)
			if !internal.CheckCacheSync(metadataInformer.WaitForCacheSync(signalCtx.Done())) {
				// TODO: shall we just exit ?
				logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
			}
			// start leader controllers
			var wg sync.WaitGroup
			for _, controller := range leaderControllers {
				controller.run(signalCtx, logger.WithName("controllers"), &wg)
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
		controller.run(signalCtx, logger.WithName("controllers"), &wg)
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
		dynamicClient,
		openApiManager,
	)
	resourceHandlers := webhooksresource.NewHandlers(
		dynamicClient,
		kyvernoClient,
		configuration,
		metricsConfig,
		policyCache,
		kubeInformer.Core().V1().Namespaces().Lister(),
		kubeInformer.Rbac().V1().RoleBindings().Lister(),
		kubeInformer.Rbac().V1().ClusterRoleBindings().Lister(),
		kyvernoInformer.Kyverno().V1beta1().UpdateRequests().Lister().UpdateRequests(config.KyvernoNamespace()),
		urgen,
		eventGenerator,
		openApiManager,
		admissionReports,
	)
	secretLister := kubeKyvernoInformer.Core().V1().Secrets().Lister()
	server := webhooks.NewServer(
		policyHandlers,
		resourceHandlers,
		configuration,
		metricsConfig,
		webhooks.DebugModeOptions{
			DumpPayload: dumpPayload,
		},
		func() ([]byte, []byte, error) {
			secret, err := secretLister.Secrets(config.KyvernoNamespace()).Get(tls.GenerateTLSPairSecretName())
			if err != nil {
				return nil, nil, err
			}
			return secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey], nil
		},
		kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations(),
		kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
		kubeClient.CoordinationV1().Leases(config.KyvernoNamespace()),
		runtime,
	)
	// start informers and wait for cache sync
	// we need to call start again because we potentially registered new informers
	if !internal.StartInformersAndWaitForCacheSync(signalCtx, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
		logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
		os.Exit(1)
	}
	// start webhooks server
	server.Run(signalCtx.Done())
	// wait for termination signal
	<-signalCtx.Done()
	wg.Wait()
	// wait for server cleanup
	<-server.Cleanup()
	// say goodbye...
	logger.V(2).Info("Kyverno shutdown successful")
}
