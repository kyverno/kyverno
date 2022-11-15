package main

// We currently accept the risk of exposing pprof and rely on users to protect the endpoint.
import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof" // #nosec
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	kyvernoclient "github.com/kyverno/kyverno/pkg/clients/wrappers"
	"github.com/kyverno/kyverno/pkg/config"
	admissionreportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/admission"
	aggregatereportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/aggregate"
	backgroundscancontroller "github.com/kyverno/kyverno/pkg/controllers/report/background"
	resourcereportcontroller "github.com/kyverno/kyverno/pkg/controllers/report/resource"
	"github.com/kyverno/kyverno/pkg/cosign"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/kyverno/pkg/tracing"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/kyverno/kyverno/pkg/version"
	"go.uber.org/automaxprocs/maxprocs" // #nosec
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
	backgroundScanWorkers      int
	logFormat                  string
	dumpPayload                bool
	leaderElectionRetryPeriod  time.Duration
	// DEPRECATED: remove in 1.9
	splitPolicyReport bool
)

func parseFlags() error {
	logging.Init(nil)
	flag.StringVar(&logFormat, "loggingFormat", logging.TextFormat, "This determines the output format of the logger.")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
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
	flag.Float64Var(&clientRateLimitQPS, "clientRateLimitQPS", 20, "Configure the maximum QPS to the Kubernetes API server from Kyverno. Uses the client default if zero.")
	flag.IntVar(&clientRateLimitBurst, "clientRateLimitBurst", 50, "Configure the maximum burst for throttle. Uses the client default if zero.")
	flag.BoolVar(&backgroundScan, "backgroundScan", true, "Enable or disable backgound scan.")
	flag.BoolVar(&admissionReports, "admissionReports", true, "Enable or disable admission reports.")
	flag.IntVar(&reportsChunkSize, "reportsChunkSize", 1000, "Max number of results in generated reports, reports will be split accordingly if there are more results to be stored.")
	flag.IntVar(&backgroundScanWorkers, "backgroundScanWorkers", backgroundscancontroller.Workers, "Configure the number of background scan workers.")
	if err := flag.Set("v", "2"); err != nil {
		return err
	}
	flag.Parse()
	return nil
}

func setupMaxProcs(logger logr.Logger) (func(), error) {
	logger = logger.WithName("maxprocs")
	if undo, err := maxprocs.Set(maxprocs.Logger(func(format string, args ...interface{}) {
		logger.Info(fmt.Sprintf(format, args...))
	})); err != nil {
		return nil, err
	} else {
		return undo, nil
	}
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

func createKubeClients(logger logr.Logger) (*rest.Config, *kubernetes.Clientset, metadataclient.Interface, error) {
	logger = logger.WithName("kube-clients")
	logger.Info("create kube clients...", "kubeconfig", kubeconfig, "qps", clientRateLimitQPS, "burst", clientRateLimitBurst)
	clientConfig, err := config.CreateClientConfig(kubeconfig, clientRateLimitQPS, clientRateLimitBurst)
	if err != nil {
		return nil, nil, nil, err
	}
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, nil, err
	}
	metadataClient, err := metadataclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, nil, nil, err
	}
	return clientConfig, kubeClient, metadataClient, nil
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

func main() {
	// parse flags
	if err := parseFlags(); err != nil {
		fmt.Println("failed to parse flags", err)
		os.Exit(1)
	}
	// setup logger
	logLevel, err := strconv.Atoi(flag.Lookup("v").Value.String())
	if err != nil {
		fmt.Println("failed to setup logger", err)
		os.Exit(1)
	}
	if err := logging.Setup(logFormat, logLevel); err != nil {
		fmt.Println("failed to setup logger", err)
		os.Exit(1)
	}
	logger := logging.WithName("setup")
	// setup maxprocs
	if undo, err := setupMaxProcs(logger); err != nil {
		logger.Error(err, "failed to configure maxprocs")
		os.Exit(1)
	} else {
		defer undo()
	}
	// show version
	showVersion(logger)
	// start profiling
	startProfiling(logger)
	// create client config and kube clients
	clientConfig, kubeClient, _, err := createKubeClients(logger)
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
	_, dynamicClient, err := createInstrumentedClients(signalCtx, logger, clientConfig, kubeClient, metricsConfig)
	if err != nil {
		logger.Error(err, "failed to create instrument clients")
		os.Exit(1)
	}
	// check we can run
	if err := sanityChecks(dynamicClient); err != nil {
		logger.Error(err, "sanity checks failed")
		os.Exit(1)
	}
	// // informer factories
	// kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, resyncPeriod)
	// kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	// kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
	// // setup leader election
	// le, err := leaderelection.New(
	// 	logger.WithName("leader-election"),
	// 	"kyverno",
	// 	config.KyvernoNamespace(),
	// 	kubeClientLeaderElection,
	// 	config.KyvernoPodName(),
	// 	leaderElectionRetryPeriod,
	// 	func(ctx context.Context) {
	// 		logger := logger.WithName("leader")
	// 		// validate config
	// 		// if err := webhookCfg.ValidateWebhookConfigurations(config.KyvernoNamespace(), config.KyvernoConfigMapName()); err != nil {
	// 		// 	logger.Error(err, "invalid format of the Kyverno init ConfigMap, please correct the format of 'data.webhooks'")
	// 		// 	os.Exit(1)
	// 		// }
	// 		// create leader factories
	// 		kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, resyncPeriod)
	// 		kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	// 		kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
	// 		metadataInformer := metadatainformers.NewSharedInformerFactory(metadataClient, 15*time.Minute)
	// 		// create leader controllers
	// 		leaderControllers, err := createrLeaderControllers(
	// 			kubeInformer,
	// 			kubeKyvernoInformer,
	// 			kyvernoInformer,
	// 			metadataInformer,
	// 			kubeClient,
	// 			kyvernoClient,
	// 			dynamicClient,
	// 			configuration,
	// 			metricsConfig,
	// 			eventGenerator,
	// 			certRenewer,
	// 			runtime,
	// 		)
	// 		if err != nil {
	// 			logger.Error(err, "failed to create leader controllers")
	// 			os.Exit(1)
	// 		}
	// 		// start informers and wait for cache sync
	// 		if !startInformersAndWaitForCacheSync(signalCtx, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
	// 			logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
	// 			os.Exit(1)
	// 		}
	// 		startInformers(signalCtx, metadataInformer)
	// 		if !checkCacheSync(metadataInformer.WaitForCacheSync(signalCtx.Done())) {
	// 			// TODO: shall we just exit ?
	// 			logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
	// 		}
	// 		// start leader controllers
	// 		var wg sync.WaitGroup
	// 		for _, controller := range leaderControllers {
	// 			controller.run(signalCtx, logger.WithName("controllers"), &wg)
	// 		}
	// 		// wait all controllers shut down
	// 		wg.Wait()
	// 	},
	// 	nil,
	// )
	// if err != nil {
	// 	logger.Error(err, "failed to initialize leader election")
	// 	os.Exit(1)
	// }
	// // start non leader controllers
	// var wg sync.WaitGroup
	// for _, controller := range nonLeaderControllers {
	// 	controller.run(signalCtx, logger.WithName("controllers"), &wg)
	// }
	// // start leader election
	// go func() {
	// 	select {
	// 	case <-signalCtx.Done():
	// 		return
	// 	default:
	// 		le.Run(signalCtx)
	// 	}
	// }()
	// // create webhooks server
	// urgen := webhookgenerate.NewGenerator(
	// 	kyvernoClient,
	// 	kyvernoInformer.Kyverno().V1beta1().UpdateRequests(),
	// )
	// policyHandlers := webhookspolicy.NewHandlers(
	// 	dynamicClient,
	// 	openApiManager,
	// )
	// resourceHandlers := webhooksresource.NewHandlers(
	// 	dynamicClient,
	// 	kyvernoClient,
	// 	configuration,
	// 	metricsConfig,
	// 	policyCache,
	// 	kubeInformer.Core().V1().Namespaces().Lister(),
	// 	kubeInformer.Rbac().V1().RoleBindings().Lister(),
	// 	kubeInformer.Rbac().V1().ClusterRoleBindings().Lister(),
	// 	kyvernoInformer.Kyverno().V1beta1().UpdateRequests().Lister().UpdateRequests(config.KyvernoNamespace()),
	// 	urgen,
	// 	eventGenerator,
	// 	openApiManager,
	// 	admissionReports,
	// )
	// // start informers and wait for cache sync
	// // we need to call start again because we potentially registered new informers
	// if !startInformersAndWaitForCacheSync(signalCtx, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
	// 	logger.Error(errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
	// 	os.Exit(1)
	// }
	// wait for termination signal
	<-signalCtx.Done()
	// wg.Wait()
	// wait for server cleanup
	// say goodbye...
	logger.V(2).Info("Kyverno shutdown successful")
}
