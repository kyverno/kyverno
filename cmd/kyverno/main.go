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
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	metadataclient "k8s.io/client-go/metadata"
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

func showStartup(logger logr.Logger) {
	logger = logger.WithName("startup")
	logger.Info("kyverno is staring...")
	version.PrintVersionInfo(logger)
	// DEPRECATED: remove in 1.9
	if splitPolicyReport {
		logger.Info("The splitPolicyReport flag is deprecated and will be removed in v1.9. It has no effect and should be removed.")
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
	// show startup message
	showStartup(logger)
	// os signal handler
	signalCtx, signalCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer signalCancel()

	debug := serverIP != ""

	// clients
	clientConfig, err := config.CreateClientConfig(kubeconfig, clientRateLimitQPS, clientRateLimitBurst)
	if err != nil {
		logger.Error(err, "Failed to build kubeconfig")
		os.Exit(1)
	}

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		logger.Error(err, "Failed to create kubernetes client")
		os.Exit(1)
	}

	// Metrics Configuration
	var metricsConfig *metrics.MetricsConfig
	metricsConfigData, err := config.NewMetricsConfigData(kubeClient)
	if err != nil {
		logger.Error(err, "failed to fetch metrics config")
		os.Exit(1)
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
		logging.WithName("Metrics"),
	)
	if err != nil {
		logger.Error(err, "failed to initialize metrics")
		os.Exit(1)
	}

	kyvernoClient, err := kyvernoclient.NewForConfig(clientConfig, metricsConfig)
	if err != nil {
		logger.Error(err, "Failed to create client")
		os.Exit(1)
	}
	dynamicClient, err := dclient.NewClient(signalCtx, clientConfig, kubeClient, metricsConfig, metadataResyncPeriod)
	if err != nil {
		logger.Error(err, "Failed to create dynamic client")
		os.Exit(1)
	}
	metadataClient, err := metadataclient.NewForConfig(clientConfig)
	if err != nil {
		logger.Error(err, "Failed to create metadata client")
		os.Exit(1)
	}
	// The leader queries/updates the lease object quite frequently. So we use a separate kube-client to eliminate the throttle issue
	kubeClientLeaderElection, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		logger.Error(err, "Failed to create kubernetes leader client")
		os.Exit(1)
	}

	// sanity checks
	if !utils.CRDsInstalled(dynamicClient.Discovery()) {
		logger.Error(fmt.Errorf("CRDs not installed"), "Failed to access Kyverno CRDs")
		os.Exit(1)
	}

	if profile {
		addr := ":" + profilePort
		logger.V(2).Info("Enable profiling, see details at https://github.com/kyverno/kyverno/wiki/Profiling-Kyverno-on-Kubernetes", "port", profilePort)
		go func() {
			if err := http.ListenAndServe(addr, nil); err != nil {
				logger.Error(err, "Failed to enable profiling")
				os.Exit(1)
			}
		}()
	}

	// informer factories
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, resyncPeriod)
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
	metadataInformer := metadatainformers.NewSharedInformerFactory(metadataClient, 15*time.Minute)

	// utils
	kyvernoV1 := kyvernoInformer.Kyverno().V1()
	kyvernoV1beta1 := kyvernoInformer.Kyverno().V1beta1()

	var registryOptions []registryclient.Option

	if otel == "grpc" {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer metrics.ShutDownController(ctx, metricsPusher)
		defer cancel()
	}

	if otel == "prometheus" {
		go func() {
			logger.Info("Enabling Metrics for Kyverno", "address", metricsAddr)
			if err := http.ListenAndServe(metricsAddr, metricsServerMux); err != nil {
				logger.Error(err, "failed to enable metrics", "address", metricsAddr)
			}
		}()
	}

	// load image registry secrets
	secrets := strings.Split(imagePullSecrets, ",")
	if imagePullSecrets != "" && len(secrets) > 0 {
		logger.V(2).Info("initializing registry credentials", "secrets", secrets)
		registryOptions = append(
			registryOptions,
			registryclient.WithKeychainPullSecrets(kubeClient, config.KyvernoNamespace(), "", secrets),
		)
	}

	if allowInsecureRegistry {
		logger.V(2).Info("initializing registry with allowing insecure connections to registries")
		registryOptions = append(
			registryOptions,
			registryclient.WithAllowInsecureRegistry(),
		)
	}

	// initialize default registry client with our settings
	registryclient.DefaultClient, err = registryclient.InitClient(registryOptions...)
	if err != nil {
		logger.Error(err, "failed to initialize registry client")
		os.Exit(1)
	}

	if imageSignatureRepository != "" {
		cosign.ImageSignatureRepository = imageSignatureRepository
	}

	// EVENT GENERATOR
	// - generate event with retry mechanism
	eventGenerator := event.NewEventGenerator(dynamicClient, kyvernoV1.ClusterPolicies(), kyvernoV1.Policies(), maxQueuedEvents, logging.WithName("EventGenerator"))

	webhookCfg := webhookconfig.NewRegister(
		signalCtx,
		clientConfig,
		dynamicClient,
		kubeClient,
		kyvernoClient,
		kubeInformer.Admissionregistration().V1().MutatingWebhookConfigurations(),
		kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations(),
		kubeKyvernoInformer.Apps().V1().Deployments(),
		kyvernoV1.ClusterPolicies(),
		kyvernoV1.Policies(),
		metricsConfig,
		serverIP,
		int32(webhookTimeout),
		debug,
		autoUpdateWebhooks,
		logging.GlobalLogger(),
	)

	webhookMonitor, err := webhookconfig.NewMonitor(kubeClient, logging.GlobalLogger())
	if err != nil {
		logger.Error(err, "failed to initialize webhookMonitor")
		os.Exit(1)
	}

	configuration, err := config.NewConfiguration(kubeClient, webhookCfg.UpdateWebhookChan)
	if err != nil {
		logger.Error(err, "failed to initialize configuration")
		os.Exit(1)
	}
	configurationController := configcontroller.NewController(configuration, kubeKyvernoInformer.Core().V1().ConfigMaps())

	// Tracing Configuration
	if enableTracing {
		logger.V(2).Info("Enabling tracing for Kyverno...")
		tracerProvider, err := tracing.NewTraceConfig(otelCollector, transportCreds, kubeClient, logging.WithName("Tracing"))
		if err != nil {
			logger.Error(err, "Failed to enable tracing for Kyverno")
			os.Exit(1)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer tracing.ShutDownController(ctx, tracerProvider)
		defer cancel()
	}

	// POLICY CONTROLLER
	// - reconciliation policy and policy violation
	// - process policy on existing resources
	// - status aggregator: receives stats when a policy is applied & updates the policy status
	policyCtrl, err := policy.NewPolicyController(
		kyvernoClient,
		dynamicClient,
		kyvernoV1.ClusterPolicies(),
		kyvernoV1.Policies(),
		kyvernoV1beta1.UpdateRequests(),
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

	urgen := webhookgenerate.NewGenerator(kyvernoClient, kyvernoV1beta1.UpdateRequests())

	urc := background.NewController(
		kyvernoClient,
		dynamicClient,
		kyvernoV1.ClusterPolicies(),
		kyvernoV1.Policies(),
		kyvernoV1beta1.UpdateRequests(),
		kubeInformer.Core().V1().Namespaces(),
		kubeKyvernoInformer.Core().V1().Pods(),
		eventGenerator,
		configuration,
	)

	policyCache := policycache.NewCache()
	policyCacheController := policycachecontroller.NewController(policyCache, kyvernoV1.ClusterPolicies(), kyvernoV1.Policies())

	certRenewer, err := tls.NewCertRenewer(
		kubeClient,
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
	certManager := certmanager.NewController(kubeKyvernoInformer.Core().V1().Secrets(), certRenewer)

	webhookController := webhookcontroller.NewController(
		metrics.ObjectClient[*corev1.Secret](
			metrics.NamespacedClientQueryRecorder(metricsConfig, config.KyvernoNamespace(), "Secret", metrics.KubeClient),
			kubeClient.CoreV1().Secrets(config.KyvernoNamespace()),
		),
		metrics.ObjectClient[*admissionregistrationv1.MutatingWebhookConfiguration](
			metrics.ClusteredClientQueryRecorder(metricsConfig, "MutatingWebhookConfiguration", metrics.KubeClient),
			kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations(),
		),
		metrics.ObjectClient[*admissionregistrationv1.ValidatingWebhookConfiguration](
			metrics.ClusteredClientQueryRecorder(metricsConfig, "ValidatingWebhookConfiguration", metrics.KubeClient),
			kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations(),
		),
		kubeKyvernoInformer.Core().V1().Secrets(),
		kubeInformer.Admissionregistration().V1().MutatingWebhookConfigurations(),
		kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations(),
	)

	// the webhook server runs across all instances
	openAPIController := startOpenAPIController(signalCtx, logger, dynamicClient)

	// WEBHOOK
	// - https server to provide endpoints called based on rules defined in Mutating & Validation webhook configuration
	// - reports the results based on the response from the policy engine:
	// -- annotations on resources with update details on mutation JSON patches
	// -- generate policy violation resource
	// -- generate events on policy and resource
	policyHandlers := webhookspolicy.NewHandlers(dynamicClient, openAPIController)
	resourceHandlers := webhooksresource.NewHandlers(
		dynamicClient,
		kyvernoClient,
		configuration,
		metricsConfig,
		policyCache,
		kubeInformer.Core().V1().Namespaces().Lister(),
		kubeInformer.Rbac().V1().RoleBindings().Lister(),
		kubeInformer.Rbac().V1().ClusterRoleBindings().Lister(),
		kyvernoV1beta1.UpdateRequests().Lister().UpdateRequests(config.KyvernoNamespace()),
		urgen,
		eventGenerator,
		openAPIController,
		admissionReports,
	)

	server := webhooks.NewServer(
		policyHandlers,
		resourceHandlers,
		certManager.GetTLSPemPair,
		configuration,
		webhookCfg,
		webhookMonitor,
	)

	// wrap all controllers that need leaderelection
	// start them once by the leader
	registerWrapperRetry := common.RetryFunc(time.Second, webhookRegistrationTimeout, webhookCfg.Register, "failed to register webhook", logger)
	run := func() {
		if err := certRenewer.InitTLSPemPair(); err != nil {
			logger.Error(err, "tls initialization error")
			os.Exit(1)
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

		for _, controller := range reportControllers {
			go controller.run(signalCtx)
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

	le, err := leaderelection.New("kyverno", config.KyvernoNamespace(), kubeClientLeaderElection, config.KyvernoPodName(), run, stop, logging.WithName("kyverno/LeaderElection"))
	if err != nil {
		logger.Error(err, "failed to elect a leader")
		os.Exit(1)
	}

	// cancel leader election context on shutdown signals
	go func() {
		defer signalCancel()
		<-signalCtx.Done()
	}()

	if !startInformersAndWaitForCacheSync(signalCtx, kyvernoInformer, kubeInformer, kubeKyvernoInformer) {
		logger.Error(err, "Failed to wait for cache sync")
		os.Exit(1)
	}

	// warmup policy cache
	if err := policyCacheController.WarmUp(); err != nil {
		logger.Error(err, "Failed to warm up policy cache")
		os.Exit(1)
	}

	// init events handlers
	// start Kyverno controllers
	go policyCacheController.Run(signalCtx, policycachecontroller.Workers)
	go urc.Run(signalCtx, genWorkers)
	go le.Run(signalCtx)
	go configurationController.Run(signalCtx, configcontroller.Workers)
	go eventGenerator.Run(signalCtx, 3)

	if !debug {
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

func startOpenAPIController(ctx context.Context, logger logr.Logger, client dclient.Interface) *openapi.Controller {
	logger = logger.WithName("open-api")
	openAPIController, err := openapi.NewOpenAPIController()
	if err != nil {
		logger.Error(err, "Failed to create openAPIController")
		os.Exit(1)
	}
	// Sync openAPI definitions of resources
	openAPISync := openapi.NewCRDSync(client, openAPIController)
	// start openAPI controller, this is used in admission review
	// thus is required in each instance
	openAPISync.Run(ctx, 1)
	return openAPIController
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
		ctrls = append(ctrls, controller{resourceReportController, resourcereportcontroller.Workers})
		ctrls = append(ctrls, controller{
			aggregatereportcontroller.NewController(
				kyvernoClient,
				metadataFactory,
				resourceReportController,
				reportsChunkSize,
			),
			aggregatereportcontroller.Workers,
		})
		if admissionReports {
			ctrls = append(ctrls, controller{
				admissionreportcontroller.NewController(
					kyvernoClient,
					metadataFactory,
					resourceReportController,
				),
				admissionreportcontroller.Workers,
			})
		}
		if backgroundScan {
			ctrls = append(ctrls, controller{
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
			})
		}
	}
	return ctrls
}
