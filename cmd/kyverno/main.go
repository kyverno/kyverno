package main

// We currently accept the risk of exposing pprof and rely on users to protect the endpoint.
import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof" // #nosec
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/background"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	kyvernoclient "github.com/kyverno/kyverno/pkg/clients/wrappers"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers/certmanager"
	configcontroller "github.com/kyverno/kyverno/pkg/controllers/config"
	policymetricscontroller "github.com/kyverno/kyverno/pkg/controllers/metrics/policy"
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
	"github.com/kyverno/kyverno/pkg/tracing"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/kyverno/kyverno/pkg/version"
	"github.com/kyverno/kyverno/pkg/webhookconfig"
	"github.com/kyverno/kyverno/pkg/webhooks"
	webhookspolicy "github.com/kyverno/kyverno/pkg/webhooks/policy"
	webhooksresource "github.com/kyverno/kyverno/pkg/webhooks/resource"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	_ "go.uber.org/automaxprocs" // #nosec
	corev1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	metadataclient "k8s.io/client-go/metadata"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/rest"
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
	profilePort                string
	metricsPort                string
	webhookTimeout             int
	genWorkers                 int
	maxQueuedEvents            int
	profile                    bool
	disableMetricsExport       bool
	enableTracing              bool
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
	logFormat                  string
	// DEPRECATED: remove in 1.9
	splitPolicyReport bool
)

func parseFlags() error {
	logging.Init(nil)
	flag.StringVar(&logFormat, "loggingFormat", logging.TextFormat, "This determines the output format of the logger.")
	flag.IntVar(&webhookTimeout, "webhookTimeout", int(webhookconfig.DefaultWebhookTimeout), "Timeout for webhook configurations.")
	flag.IntVar(&genWorkers, "genWorkers", 10, "Workers for generate controller.")
	flag.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flag.BoolVar(&profile, "profile", false, "Set this flag to 'true', to enable profiling.")
	flag.StringVar(&profilePort, "profilePort", "6060", "Enable profiling at given port, defaults to 6060.")
	flag.BoolVar(&disableMetricsExport, "disableMetrics", false, "Set this flag to 'true' to disable metrics.")
	flag.BoolVar(&enableTracing, "enableTracing", false, "Set this flag to 'true', to enable exposing traces.")
	flag.StringVar(&otel, "otelConfig", "prometheus", "Set this flag to 'grpc', to enable exporting metrics to an Opentelemetry Collector. The default collector is set to \"prometheus\"")
	flag.StringVar(&otelCollector, "otelCollector", "opentelemetrycollector.kyverno.svc.cluster.local", "Set this flag to the OpenTelemetry Collector Service Address. Kyverno will try to connect to this on the metrics port.")
	flag.StringVar(&transportCreds, "transportCreds", "", "Set this flag to the CA secret containing the certificate which is used by our Opentelemetry Metrics Client. If empty string is set, means an insecure connection will be used")
	flag.StringVar(&metricsPort, "metricsPort", "8000", "Expose prometheus metrics at the given port, default to 8000.")
	flag.StringVar(&imagePullSecrets, "imagePullSecrets", "", "Secret resource names for image registry access credentials.")
	flag.StringVar(&imageSignatureRepository, "imageSignatureRepository", "", "Alternate repository for image signatures. Can be overridden per rule via `verifyImages.Repository`.")
	flag.BoolVar(&allowInsecureRegistry, "allowInsecureRegistry", false, "Whether to allow insecure connections to registries. Don't use this for anything but testing.")
	flag.BoolVar(&autoUpdateWebhooks, "autoUpdateWebhooks", true, "Set this flag to 'false' to disable auto-configuration of the webhook.")
	flag.Float64Var(&clientRateLimitQPS, "clientRateLimitQPS", 100, "Configure the maximum QPS to the Kubernetes API server from Kyverno. Uses the client default if zero.")
	flag.IntVar(&clientRateLimitBurst, "clientRateLimitBurst", 100, "Configure the maximum burst for throttle. Uses the client default if zero.")
	flag.Func(toggle.AutogenInternalsFlagName, toggle.AutogenInternalsDescription, toggle.AutogenInternals.Parse)
	flag.DurationVar(&webhookRegistrationTimeout, "webhookRegistrationTimeout", 120*time.Second, "Timeout for webhook registration, e.g., 30s, 1m, 5m.")
	flag.Func(toggle.ProtectManagedResourcesFlagName, toggle.ProtectManagedResourcesDescription, toggle.ProtectManagedResources.Parse)
	flag.BoolVar(&backgroundScan, "backgroundScan", true, "Enable or disable backgound scan.")
	flag.BoolVar(&admissionReports, "admissionReports", true, "Enable or disable admission reports.")
	flag.IntVar(&reportsChunkSize, "reportsChunkSize", 1000, "Max number of results in generated reports, reports will be split accordingly if there are more results to be stored.")
	// DEPRECATED: remove in 1.9
	flag.BoolVar(&splitPolicyReport, "splitPolicyReport", false, "This is deprecated, please don't use it, will be removed in v1.9.")
	if err := flag.Set("v", "2"); err != nil {
		return err
	}
	flag.Parse()
	return nil
}

func startProfiling(logger logr.Logger) {
	logger = logger.WithName("profiling")
	logger.Info("start profiling...", "profile", profile, "port", profilePort)
	if profile {
		addr := ":" + profilePort
		logger.Info("Enable profiling, see details at https://github.com/kyverno/kyverno/wiki/Profiling-Kyverno-on-Kubernetes", "port", profilePort)
		go func() {
			if err := http.ListenAndServe(addr, nil); err != nil {
				logger.Error(err, "Failed to enable profiling")
				os.Exit(1)
			}
		}()
	}
}

func createKubeClients(logger logr.Logger) (*rest.Config, *kubernetes.Clientset, metadataclient.Interface, kubernetes.Interface, error) {
	logger = logger.WithName("kube-clients")
	logger.Info("create kube clients...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	clientConfig, err := config.CreateClientConfig(kubeconfig, clientRateLimitQPS, clientRateLimitBurst)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	metadataClient, err := metadataclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	// The leader queries/updates the lease object quite frequently. So we use a separate kube-client to eliminate the throttle issue
	kubeClientLeaderElection, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return clientConfig, kubeClient, metadataClient, kubeClientLeaderElection, nil
}

func createInstrumentedClients(ctx context.Context, logger logr.Logger, clientConfig *rest.Config, kubeClient *kubernetes.Clientset, metricsConfig *metrics.MetricsConfig) (versioned.Interface, dclient.Interface, error) {
	logger = logger.WithName("instrumented-clients")
	logger.Info("create instrumented clients...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	kyvernoClient, err := kyvernoclient.NewForConfig(clientConfig, metricsConfig)
	if err != nil {
		return nil, nil, err
	}
	dynamicClient, err := dclient.NewClient(ctx, clientConfig, kubeClient, metricsConfig, metadataResyncPeriod)
	if err != nil {
		return nil, nil, err
	}
	return kyvernoClient, dynamicClient, nil
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
			if err := http.ListenAndServe(metricsAddr, metricsServerMux); err != nil {
				logger.Error(err, "failed to enable metrics", "address", metricsAddr)
			}
		}()
	}
	return metricsConfig, cancel, nil
}

func setupTracing(logger logr.Logger, kubeClient kubernetes.Interface) (context.CancelFunc, error) {
	logger = logger.WithName("tracing")
	logger.Info("setup tracing...", "enabled", enableTracing, "port", otelCollector, "creds", transportCreds)
	var cancel context.CancelFunc
	if enableTracing {
		tracerProvider, err := tracing.NewTraceConfig(otelCollector, transportCreds, kubeClient, logging.WithName("tracing"))
		if err != nil {
			return nil, err
		}
		cancel = func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			defer tracing.ShutDownController(ctx, tracerProvider)
		}
	}
	return cancel, nil
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
}

func showVersion(logger logr.Logger) {
	logger = logger.WithName("version")
	version.PrintVersionInfo(logger)
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
	manager *openapi.Controller,
) ([]controller, func() error) {
	policyCacheController := policycachecontroller.NewController(
		policyCache,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
	)
	openApiController := openapi.NewCRDSync(
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
			newController("openapi-controller", openApiController, 1),
			newController(configcontroller.ControllerName, configurationController, configcontroller.Workers),
			newController("update-request-controller", updateRequestController, genWorkers),
		},
		func() error {
			return policyCacheController.WarmUp()
		}
}

func main() {
	// parse flags
	if err := parseFlags(); err != nil {
		fmt.Println("failed to parse flags", err)
		os.Exit(1)
	}
	// setup logger
	if err := logging.Setup(logFormat); err != nil {
		fmt.Println("failed to setup logger", err)
		os.Exit(1)
	}
	logger := logging.WithName("setup")
	// show version
	showWarnings(logger)
	// show version
	showVersion(logger)
	// start profiling
	startProfiling(logger)
	// create client config and kube clients
	clientConfig, kubeClient, metadataClient, kubeClientLeaderElection, err := createKubeClients(logger)
	if err != nil {
		logger.Error(err, "failed to create kubernetes clients")
		os.Exit(1)
	}
	// setup metrics
	metricsConfig, metricsShutdown, err := setupMetrics(logger, kubeClient)
	if err != nil {
		logger.Error(err, "failed to setup metrics")
		os.Exit(1)
	}
	if metricsShutdown != nil {
		defer metricsShutdown()
	}
	// setup tracing
	if tracingShutdown, err := setupTracing(logger, kubeClient); err != nil {
		logger.Error(err, "failed to setup tracing")
		os.Exit(1)
	} else if tracingShutdown != nil {
		defer tracingShutdown()
	}
	// setup registry client
	if err := setupRegistryClient(logger, kubeClient); err != nil {
		logger.Error(err, "failed to setup registry client")
		os.Exit(1)
	}
	// setup cosign
	setupCosign(logger)
	// setup signals
	signalCtx, signalCancel := setupSignals()
	defer signalCancel()
	// create instrumented clients
	kyvernoClient, dynamicClient, err := createInstrumentedClients(signalCtx, logger, clientConfig, kubeClient, metricsConfig)
	if err != nil {
		logger.Error(err, "failed to create instrument clients")
		os.Exit(1)
	}
	// check we can run
	if err := sanityChecks(dynamicClient); err != nil {
		logger.Error(err, "sanity checks failed")
		os.Exit(1)
	}
	// informer factories
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, resyncPeriod)
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
	metadataInformer := metadatainformers.NewSharedInformerFactory(metadataClient, 15*time.Minute)

	webhookCfg := webhookconfig.NewRegister(
		signalCtx,
		clientConfig,
		dynamicClient,
		kubeClient,
		kyvernoClient,
		kubeInformer.Admissionregistration().V1().MutatingWebhookConfigurations(),
		kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations(),
		kubeKyvernoInformer.Apps().V1().Deployments(),
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		metricsConfig,
		serverIP,
		int32(webhookTimeout),
		autoUpdateWebhooks,
		logging.GlobalLogger(),
	)
	configuration, err := config.NewConfiguration(
		kubeClient,
		webhookCfg.UpdateWebhookChan,
	)
	if err != nil {
		logger.Error(err, "failed to initialize configuration")
		os.Exit(1)
	}
	openApiManager, err := openapi.NewOpenAPIController()
	if err != nil {
		logger.Error(err, "Failed to create openapi manager")
		os.Exit(1)
	}
	policyCache := policycache.NewCache()
	eventGenerator := event.NewEventGenerator(
		dynamicClient,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		maxQueuedEvents,
		logging.WithName("EventGenerator"),
	)

	webhookMonitor, err := webhookconfig.NewMonitor(kubeClient, logging.GlobalLogger())
	if err != nil {
		logger.Error(err, "failed to initialize webhookMonitor")
		os.Exit(1)
	}

	// POLICY CONTROLLER
	// - reconciliation policy and policy violation
	// - process policy on existing resources
	// - status aggregator: receives stats when a policy is applied & updates the policy status
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
		logger.Error(err, "Failed to create policy controller")
		os.Exit(1)
	}

	urgen := webhookgenerate.NewGenerator(kyvernoClient, kyvernoInformer.Kyverno().V1beta1().UpdateRequests())

	certRenewer, err := tls.NewCertRenewer(
		metrics.ObjectClient[*corev1.Secret](
			metrics.NamespacedClientQueryRecorder(metricsConfig, config.KyvernoNamespace(), "Secret", metrics.KubeClient),
			kubeClient.CoreV1().Secrets(config.KyvernoNamespace()),
		),
		clientConfig,
		tls.CertRenewalInterval,
		tls.CAValidityDuration,
		tls.TLSValidityDuration,
		serverIP,
		logging.WithName("CertRenewer"),
	)
	if err != nil {
		logger.Error(err, "failed to initialize CertRenewer")
		os.Exit(1)
	}
	policyCache := policycache.NewCache()
	eventGenerator := event.NewEventGenerator(
		dynamicClient,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
		maxQueuedEvents,
		logging.WithName("EventGenerator"),
	)
	// This controller only subscribe to events, nothing is returned...
	policymetricscontroller.NewController(
		metricsConfig,
		kyvernoInformer.Kyverno().V1().ClusterPolicies(),
		kyvernoInformer.Kyverno().V1().Policies(),
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
	if !startInformersAndWaitForCacheSync(signalCtx, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
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
	// start non leader controllers
	for _, controller := range nonLeaderControllers {
		go controller.run(signalCtx, logger.WithName("controllers"))
	}
	// setup leader election
	le, err := leaderelection.New(
		logger.WithName("leader-election"),
		"kyverno",
		config.KyvernoNamespace(),
		kubeClientLeaderElection,
		config.KyvernoPodName(),
		func(ctx context.Context) {
			logger := logger.WithName("leader")
			// when losing the lead we just terminate the pod
			defer signalCancel()
			// init tls secret
			if err := certRenewer.InitTLSPemPair(); err != nil {
				logger.Error(err, "tls initialization error")
				os.Exit(1)
			}
			// validate config
			if err := webhookCfg.ValidateWebhookConfigurations(config.KyvernoNamespace(), config.KyvernoConfigMapName()); err != nil {
				logger.Error(err, "invalid format of the Kyverno init ConfigMap, please correct the format of 'data.webhooks'")
				os.Exit(1)
			}
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
			)
			if err != nil {
				logger.Error(err, "failed to create leader controllers")
				os.Exit(1)
			}
			// start informers and wait for cache sync
			if !startInformersAndWaitForCacheSync(signalCtx, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
				logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
				os.Exit(1)
			}
			startInformers(signalCtx, metadataInformer)
			if !checkCacheSync(metadataInformer.WaitForCacheSync(signalCtx.Done())) {
				// TODO: shall we just exit ?
				logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
			}
			// bootstrap
			if autoUpdateWebhooks {
				go webhookCfg.UpdateWebhookConfigurations(configuration)
			}
			registerWrapperRetry := common.RetryFunc(time.Second, webhookRegistrationTimeout, webhookCfg.Register, "failed to register webhook", logger)
			if err := registerWrapperRetry(); err != nil {
				logger.Error(err, "timeout registering admission control webhooks")
				os.Exit(1)
			}
			webhookCfg.UpdateWebhookChan <- true
			// start leader controllers
			for _, controller := range leaderControllers {
				go controller.run(signalCtx, logger.WithName("controllers"))
			}
			// wait until we loose the lead (or signal context is canceled)
			<-ctx.Done()
		},
		nil,
	)
	if err != nil {
		logger.Error(err, "failed to initialize leader election")
		os.Exit(1)
	}
	// start leader election
	go le.Run(signalCtx)
	// create monitor
	webhookMonitor, err := webhookconfig.NewMonitor(kubeClient, logging.GlobalLogger())
	if err != nil {
		logger.Error(err, "failed to initialize webhookMonitor")
		os.Exit(1)
	}
	// start monitor (only when running in cluster)
	if serverIP == "" {
		go webhookMonitor.Run(signalCtx, webhookCfg, certRenewer, eventGenerator)
	}
	// create webhooks server
	urgen := webhookgenerate.NewGenerator(
		kyvernoClient,
		kyvernoInformer.Kyverno().V1beta1().UpdateRequests(),
	)
	policyHandlers := webhookspolicy.NewHandlers(
		dynamicClient,
		openApiManager,
	)

	// WEBHOOK
	// - https server to provide endpoints called based on rules defined in Mutating & Validation webhook configuration
	// - reports the results based on the response from the policy engine:
	// -- annotations on resources with update details on mutation JSON patches
	// -- generate policy violation resource
	// -- generate events on policy and resource
	policyHandlers := webhookspolicy.NewHandlers(dynamicClient, openApiManager)
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
		func() ([]byte, []byte, error) {
			secret, err := secretLister.Secrets(config.KyvernoNamespace()).Get(tls.GenerateTLSPairSecretName())
			if err != nil {
				return nil, nil, err
			}
			return secret.Data[corev1.TLSCertKey], secret.Data[corev1.TLSPrivateKeyKey], nil
		},
		configuration,
		webhookCfg,
		webhookMonitor,
	)

	// wrap all controllers that need leaderelection
	// start them once by the leader
	registerWrapperRetry := common.RetryFunc(time.Second, webhookRegistrationTimeout, webhookCfg.Register, "failed to register webhook", logger)
	run := func(context.Context) {
		logger := logger.WithName("leader")
		if err := certRenewer.InitTLSPemPair(); err != nil {
			logger.Error(err, "tls initialization error")
			os.Exit(1)
		}
		// wait for cache to be synced before use it
		if !waitForInformersCacheSync(signalCtx,
			kubeInformer.Admissionregistration().V1().MutatingWebhookConfigurations().Informer(),
			kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations().Informer(),
		) {
			// TODO: shall we just exit ?
			logger.Info("failed to wait for cache sync")
		}

		// validate the ConfigMap format
		if err := webhookCfg.ValidateWebhookConfigurations(config.KyvernoNamespace(), config.KyvernoConfigMapName()); err != nil {
			logger.Error(err, "invalid format of the Kyverno init ConfigMap, please correct the format of 'data.webhooks'")
			os.Exit(1)
		}
		if autoUpdateWebhooks {
			go webhookCfg.UpdateWebhookConfigurations(configuration)
		}
		if registrationErr := registerWrapperRetry(); registrationErr != nil {
			logger.Error(err, "Timeout registering admission control webhooks")
			os.Exit(1)
		}
		webhookCfg.UpdateWebhookChan <- true
		go certManager.Run(signalCtx, certmanager.Workers)
		go policyCtrl.Run(signalCtx, 2)
		go webhookController.Run(signalCtx, webhookcontroller.Workers)

		reportControllers := setupReportControllers(
			backgroundScan,
			admissionReports,
			dynamicClient,
			kyvernoClient,
			metadataInformer,
			kubeInformer,
			kyvernoInformer,
		)
		startInformers(signalCtx, metadataInformer)
		if !checkCacheSync(metadataInformer.WaitForCacheSync(signalCtx.Done())) {
			// TODO: shall we just exit ?
			logger.Info("failed to wait for cache sync")
		}

		for i := range reportControllers {
			go reportControllers[i].run(signalCtx, logger.WithName("controllers"))
		}
	}

	// cleanup Kyverno managed resources followed by webhook shutdown
	// No need to exit here, as server.Stop(ctx) closes the cleanUp
	// chan, thus the main process exits.
	stop := func() {
		c, cancel := context.WithCancel(context.Background())
		defer cancel()
		server.Stop(c)
	}

	le, err := leaderelection.New(
		logger.WithName("leader-election"),
		"kyverno",
		config.KyvernoNamespace(),
		kubeClientLeaderElection,
		config.KyvernoPodName(),
		run,
		stop,
	)
	if err != nil {
		logger.Error(err, "failed to elect a leader")
		os.Exit(1)
	}

	// cancel leader election context on shutdown signals
	go func() {
		defer signalCancel()
		<-signalCtx.Done()
	}()
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
	if !startInformersAndWaitForCacheSync(signalCtx, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
		logger.Error(err, "failed to wait for cache sync")
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
	// start leader election
	go le.Run(signalCtx)
	// start non leader controllers
	for _, controller := range nonLeaderControllers {
		go controller.run(signalCtx, logger.WithName("controllers"))
	}
	// start monitor (only when running in cluster)
	if serverIP == "" {
		go webhookMonitor.Run(signalCtx, webhookCfg, certRenewer, eventGenerator)
	}

	// verifies if the admission control is enabled and active
	server.Run(signalCtx.Done())

	<-signalCtx.Done()

	// resource cleanup
	// remove webhook configurations
	<-server.Cleanup()
	logger.V(2).Info("Kyverno shutdown successful")
}

func setupReportControllers(
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
				backgroundscancontroller.Workers,
			))
		}
	}
	return ctrls
}
