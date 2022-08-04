package main

// We currently accept the risk of exposing pprof and rely on users to protect the endpoint.
import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof" // #nosec
	"os"
	"strings"
	"time"

	_ "go.uber.org/automaxprocs" // #nosec

	"github.com/kyverno/kyverno/pkg/background"
	generatecleanup "github.com/kyverno/kyverno/pkg/background/generate/cleanup"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers/certmanager"
	configcontroller "github.com/kyverno/kyverno/pkg/controllers/config"
	policycachecontroller "github.com/kyverno/kyverno/pkg/controllers/policycache"
	"github.com/kyverno/kyverno/pkg/cosign"
	"github.com/kyverno/kyverno/pkg/dclient"
	event "github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/kyverno/pkg/signal"
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
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const resyncPeriod = 15 * time.Minute

var (
	// TODO: this has been added to backward support command line arguments
	// will be removed in future and the configuration will be set only via configmaps
	serverIP                     string
	profilePort                  string
	metricsPort                  string
	webhookTimeout               int
	genWorkers                   int
	maxQueuedEvents              int
	profile                      bool
	disableMetricsExport         bool
	enableTracing                bool
	otel                         string
	otelCollector                string
	transportCreds               string
	autoUpdateWebhooks           bool
	policyControllerResyncPeriod time.Duration
	imagePullSecrets             string
	imageSignatureRepository     string
	allowInsecureRegistry        bool
	clientRateLimitQPS           float64
	clientRateLimitBurst         int
	changeRequestLimit           int
	webhookRegistrationTimeout   time.Duration
	setupLog                     = log.Log.WithName("setup")
)

func main() {
	// clear flags initialized in static dependencies
	if flag.CommandLine.Lookup("log_dir") != nil {
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	}

	klog.InitFlags(nil)
	log.SetLogger(klogr.New())
	flag.IntVar(&webhookTimeout, "webhookTimeout", int(webhookconfig.DefaultWebhookTimeout), "Timeout for webhook configurations.")
	flag.IntVar(&genWorkers, "genWorkers", 10, "Workers for generate controller.")
	flag.IntVar(&maxQueuedEvents, "maxQueuedEvents", 1000, "Maximum events to be queued.")
	flag.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flag.BoolVar(&profile, "profile", false, "Set this flag to 'true', to enable profiling.")
	flag.StringVar(&profilePort, "profilePort", "6060", "Enable profiling at given port, defaults to 6060.")
	flag.BoolVar(&disableMetricsExport, "disableMetrics", false, "Set this flag to 'true' to disable metrics.")
	flag.BoolVar(&enableTracing, "enableTracing", false, "Set this flag to 'true', to enable exposing traces.")
	flag.StringVar(&otel, "otelConfig", "prometheus", "Set this flag to 'grpc', to enable exporting metrics to an Opentelemetry Collector. The default collector is set to \"prometheus\"")
	flag.StringVar(&otelCollector, "otelCollector", "opentelemetrycollector.kyverno.svc.cluster.local", "Set this flag to the OpenTelemetry Collector Service Address. Kyverno will try to connect to this on the metrics port.")
	flag.StringVar(&transportCreds, "transportCreds", "", "Set this flag to the CA secret containing the certificate which is used by our Opentelemetry Metrics Client. If empty string is set, means an insecure connection will be used")
	flag.StringVar(&metricsPort, "metricsPort", "8000", "Expose prometheus metrics at the given port, default to 8000.")
	flag.DurationVar(&policyControllerResyncPeriod, "backgroundScan", time.Hour, "Perform background scan every given interval, e.g., 30s, 15m, 1h.")
	flag.StringVar(&imagePullSecrets, "imagePullSecrets", "", "Secret resource names for image registry access credentials.")
	flag.StringVar(&imageSignatureRepository, "imageSignatureRepository", "", "Alternate repository for image signatures. Can be overridden per rule via `verifyImages.Repository`.")
	flag.BoolVar(&allowInsecureRegistry, "allowInsecureRegistry", false, "Whether to allow insecure connections to registries. Don't use this for anything but testing.")
	flag.BoolVar(&autoUpdateWebhooks, "autoUpdateWebhooks", true, "Set this flag to 'false' to disable auto-configuration of the webhook.")
	flag.Float64Var(&clientRateLimitQPS, "clientRateLimitQPS", 0, "Configure the maximum QPS to the Kubernetes API server from Kyverno. Uses the client default if zero.")
	flag.IntVar(&clientRateLimitBurst, "clientRateLimitBurst", 0, "Configure the maximum burst for throttle. Uses the client default if zero.")
	flag.Func(toggle.AutogenInternalsFlagName, toggle.AutogenInternalsDescription, toggle.AutogenInternalsFlag)
	flag.DurationVar(&webhookRegistrationTimeout, "webhookRegistrationTimeout", 120*time.Second, "Timeout for webhook registration, e.g., 30s, 1m, 5m.")
	flag.IntVar(&changeRequestLimit, "maxReportChangeRequests", 1000, "Maximum pending report change requests per namespace or for the cluster-wide policy report.")
	flag.Func(toggle.SplitPolicyReportFlagName, "Set the flag to 'true', to enable the split-up PolicyReports per policy.", toggle.SplitPolicyReportFlag)
	if err := flag.Set("v", "2"); err != nil {
		setupLog.Error(err, "failed to set log level")
		os.Exit(1)
	}
	flag.Parse()

	version.PrintVersionInfo(log.Log)

	cleanUp := make(chan struct{})
	stopCh := signal.SetupSignalHandler()
	debug := serverIP != ""

	// clients
	clientConfig, err := rest.InClusterConfig()
	if err != nil {
		setupLog.Error(err, "Failed to create clientConfig")
		os.Exit(1)
	}
	if err := config.ConfigureClientConfig(clientConfig, clientRateLimitQPS, clientRateLimitBurst); err != nil {
		setupLog.Error(err, "Failed to create clientConfig")
		os.Exit(1)
	}
	kyvernoClient, err := kyvernoclient.NewForConfig(clientConfig)
	if err != nil {
		setupLog.Error(err, "Failed to create client")
		os.Exit(1)
	}
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		setupLog.Error(err, "Failed to create kubernetes client")
		os.Exit(1)
	}
	dynamicClient, err := dclient.NewClient(clientConfig, kubeClient, 15*time.Minute, stopCh)
	if err != nil {
		setupLog.Error(err, "Failed to create dynamic client")
		os.Exit(1)
	}

	// sanity checks
	if !utils.CRDsInstalled(dynamicClient.Discovery()) {
		setupLog.Error(fmt.Errorf("CRDs not installed"), "Failed to access Kyverno CRDs")
		os.Exit(1)
	}

	var metricsConfig *metrics.MetricsConfig

	if profile {
		addr := ":" + profilePort
		setupLog.Info("Enable profiling, see details at https://github.com/kyverno/kyverno/wiki/Profiling-Kyverno-on-Kubernetes", "port", profilePort)
		go func() {
			if err := http.ListenAndServe(addr, nil); err != nil {
				setupLog.Error(err, "Failed to enable profiling")
				os.Exit(1)
			}
		}()
	}

	// informer factories
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, resyncPeriod)
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, policyControllerResyncPeriod)

	// utils
	kyvernoV1 := kyvernoInformer.Kyverno().V1()
	kyvernoV1beta1 := kyvernoInformer.Kyverno().V1beta1()
	kyvernoV1alpha2 := kyvernoInformer.Kyverno().V1alpha2()

	var registryOptions []registryclient.Option

	// load image registry secrets
	secrets := strings.Split(imagePullSecrets, ",")
	if imagePullSecrets != "" && len(secrets) > 0 {
		setupLog.Info("initializing registry credentials", "secrets", secrets)
		registryOptions = append(
			registryOptions,
			registryclient.WithKeychainPullSecrets(kubeClient, config.KyvernoNamespace(), "", secrets),
		)
	}

	if allowInsecureRegistry {
		setupLog.Info("initializing registry with allowing insecure connections to registries")
		registryOptions = append(
			registryOptions,
			registryclient.WithAllowInsecureRegistry(),
		)
	}

	// initialize default registry client with our settings
	registryclient.DefaultClient, err = registryclient.InitClient(registryOptions...)
	if err != nil {
		setupLog.Error(err, "failed to initialize registry client")
		os.Exit(1)
	}

	if imageSignatureRepository != "" {
		cosign.ImageSignatureRepository = imageSignatureRepository
	}

	// EVENT GENERATOR
	// - generate event with retry mechanism
	eventGenerator := event.NewEventGenerator(dynamicClient, kyvernoV1.ClusterPolicies(), kyvernoV1.Policies(), maxQueuedEvents, log.Log.WithName("EventGenerator"))

	// POLICY Report GENERATOR
	reportReqGen := policyreport.NewReportChangeRequestGenerator(kyvernoClient,
		dynamicClient,
		kyvernoV1alpha2.ReportChangeRequests(),
		kyvernoV1alpha2.ClusterReportChangeRequests(),
		kyvernoV1.ClusterPolicies(),
		kyvernoV1.Policies(),
		changeRequestLimit,
		log.Log.WithName("ReportChangeRequestGenerator"),
	)

	prgen, err := policyreport.NewReportGenerator(
		kyvernoClient,
		dynamicClient,
		kyvernoInformer.Wgpolicyk8s().V1alpha2().ClusterPolicyReports(),
		kyvernoInformer.Wgpolicyk8s().V1alpha2().PolicyReports(),
		kyvernoV1alpha2.ReportChangeRequests(),
		kyvernoV1alpha2.ClusterReportChangeRequests(),
		kubeInformer.Core().V1().Namespaces(),
		reportReqGen.CleanupChangeRequest,
		log.Log.WithName("PolicyReportGenerator"),
	)
	if err != nil {
		setupLog.Error(err, "Failed to create policy report controller")
		os.Exit(1)
	}

	webhookCfg := webhookconfig.NewRegister(
		clientConfig,
		dynamicClient,
		kubeClient,
		kyvernoClient,
		kubeInformer.Admissionregistration().V1().MutatingWebhookConfigurations(),
		kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations(),
		kubeKyvernoInformer.Apps().V1().Deployments(),
		kyvernoV1.ClusterPolicies(),
		kyvernoV1.Policies(),
		serverIP,
		int32(webhookTimeout),
		debug,
		autoUpdateWebhooks,
		stopCh,
		log.Log,
	)

	webhookMonitor, err := webhookconfig.NewMonitor(kubeClient, log.Log)
	if err != nil {
		setupLog.Error(err, "failed to initialize webhookMonitor")
		os.Exit(1)
	}

	configuration, err := config.NewConfiguration(kubeClient, prgen.ReconcileCh, webhookCfg.UpdateWebhookChan)
	if err != nil {
		setupLog.Error(err, "failed to initialize configuration")
		os.Exit(1)
	}
	configurationController := configcontroller.NewController(configuration, kubeKyvernoInformer.Core().V1().ConfigMaps())

	metricsConfigData, err := config.NewMetricsConfigData(kubeClient)
	if err != nil {
		setupLog.Error(err, "failed to fetch metrics config")
		os.Exit(1)
	}

	// Metrics Configuration
	metricsAddr := ":" + metricsPort
	metricsConfig, metricsServerMux, metricsPusher, err := metrics.InitMetrics(
		disableMetricsExport,
		otel,
		metricsAddr,
		otelCollector,
		metricsConfigData,
		transportCreds,
		kubeClient,
		log.Log.WithName("Metrics"),
	)
	if err != nil {
		setupLog.Error(err, "failed to initialize metrics")
		os.Exit(1)
	}

	if otel == "grpc" {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer metrics.ShutDownController(ctx, metricsPusher)
		defer cancel()
	}

	if otel == "prometheus" {
		go func() {
			setupLog.Info("Enabling Metrics for Kyverno", "address", metricsAddr)
			if err := http.ListenAndServe(metricsAddr, metricsServerMux); err != nil {
				setupLog.Error(err, "failed to enable metrics", "address", metricsAddr)
			}

		}()
	}

	// Tracing Configuration
	if enableTracing {
		setupLog.Info("Enabling tracing for Kyverno...")
		tracerProvider, err := tracing.NewTraceConfig(otelCollector, transportCreds, kubeClient, log.Log.WithName("Tracing"))
		if err != nil {
			setupLog.Error(err, "Failed to enable tracing for Kyverno")
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
		kubeClient,
		kyvernoClient,
		dynamicClient,
		kyvernoV1.ClusterPolicies(),
		kyvernoV1.Policies(),
		kyvernoV1beta1.UpdateRequests(),
		configuration,
		eventGenerator,
		reportReqGen,
		prgen,
		kubeInformer.Core().V1().Namespaces(),
		log.Log.WithName("PolicyController"),
		policyControllerResyncPeriod,
		metricsConfig,
	)
	if err != nil {
		setupLog.Error(err, "Failed to create policy controller")
		os.Exit(1)
	}

	urgen := webhookgenerate.NewGenerator(kyvernoClient, kyvernoV1beta1.UpdateRequests())

	urc := background.NewController(
		kubeClient,
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

	grcc := generatecleanup.NewController(
		kubeClient,
		kyvernoClient,
		dynamicClient,
		kyvernoV1.ClusterPolicies(),
		kyvernoV1.Policies(),
		kyvernoV1beta1.UpdateRequests(),
		kubeInformer.Core().V1().Namespaces(),
	)

	policyCache := policycache.NewCache()
	policyCacheController := policycachecontroller.NewController(policyCache, kyvernoV1.ClusterPolicies(), kyvernoV1.Policies())

	auditHandler := webhooksresource.NewValidateAuditHandler(
		policyCache,
		eventGenerator,
		reportReqGen,
		kubeInformer.Rbac().V1().RoleBindings(),
		kubeInformer.Rbac().V1().ClusterRoleBindings(),
		kubeInformer.Core().V1().Namespaces(),
		log.Log.WithName("ValidateAuditHandler"),
		configuration,
		dynamicClient,
		metricsConfig,
	)

	certRenewer, err := tls.NewCertRenewer(
		kubeClient,
		clientConfig,
		tls.CertRenewalInterval,
		tls.CAValidityDuration,
		tls.TLSValidityDuration,
		serverIP,
		log.Log.WithName("CertRenewer"),
	)
	if err != nil {
		setupLog.Error(err, "failed to initialize CertRenewer")
		os.Exit(1)
	}
	certManager, err := certmanager.NewController(kubeKyvernoInformer.Core().V1().Secrets(), certRenewer, webhookCfg.UpdateWebhooksCaBundle)
	if err != nil {
		setupLog.Error(err, "failed to initialize CertManager")
		os.Exit(1)
	}

	registerWrapperRetry := common.RetryFunc(time.Second, webhookRegistrationTimeout, webhookCfg.Register, "failed to register webhook", setupLog)
	registerWebhookConfigurations := func() {
		if err := certRenewer.InitTLSPemPair(); err != nil {
			setupLog.Error(err, "tls initialization error")
			os.Exit(1)
		}
		// wait for cache to be synced before use it
		cache.WaitForCacheSync(stopCh,
			kubeInformer.Admissionregistration().V1().MutatingWebhookConfigurations().Informer().HasSynced,
			kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations().Informer().HasSynced,
		)

		// validate the ConfigMap format
		if err := webhookCfg.ValidateWebhookConfigurations(config.KyvernoNamespace(), config.KyvernoConfigMapName()); err != nil {
			setupLog.Error(err, "invalid format of the Kyverno init ConfigMap, please correct the format of 'data.webhooks'")
			os.Exit(1)
		}
		if autoUpdateWebhooks {
			go webhookCfg.UpdateWebhookConfigurations(configuration)
		}
		if registrationErr := registerWrapperRetry(); registrationErr != nil {
			setupLog.Error(err, "Timeout registering admission control webhooks")
			os.Exit(1)
		}
		webhookCfg.UpdateWebhookChan <- true
	}

	// leader election context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// cancel leader election context on shutdown signals
	go func() {
		<-stopCh
		cancel()
	}()

	// webhookconfigurations are registered by the leader only
	webhookRegisterLeader, err := leaderelection.New("webhook-register", config.KyvernoNamespace(), kubeClient, registerWebhookConfigurations, nil, log.Log.WithName("webhookRegister/LeaderElection"))
	if err != nil {
		setupLog.Error(err, "failed to elect a leader")
		os.Exit(1)
	}

	go webhookRegisterLeader.Run(ctx)

	// the webhook server runs across all instances
	openAPIController := startOpenAPIController(dynamicClient, stopCh)

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
		reportReqGen,
		urgen,
		eventGenerator,
		auditHandler,
		openAPIController,
	)

	server := webhooks.NewServer(
		policyHandlers,
		resourceHandlers,
		certManager.GetTLSPemPair,
		configuration,
		webhookCfg,
		webhookMonitor,
		cleanUp,
	)

	// wrap all controllers that need leaderelection
	// start them once by the leader
	run := func() {
		go certManager.Run(stopCh)
		go policyCtrl.Run(2, prgen.ReconcileCh, reportReqGen.CleanupChangeRequest, stopCh)
		go prgen.Run(1, stopCh)
		go grcc.Run(1, stopCh)
	}

	kubeClientLeaderElection, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		setupLog.Error(err, "Failed to create kubernetes client")
		os.Exit(1)
	}

	// cleanup Kyverno managed resources followed by webhook shutdown
	// No need to exit here, as server.Stop(ctx) closes the cleanUp
	// chan, thus the main process exits.
	stop := func() {
		c, cancel := context.WithCancel(context.Background())
		defer cancel()
		server.Stop(c)
	}

	le, err := leaderelection.New("kyverno", config.KyvernoNamespace(), kubeClientLeaderElection, run, stop, log.Log.WithName("kyverno/LeaderElection"))
	if err != nil {
		setupLog.Error(err, "failed to elect a leader")
		os.Exit(1)
	}

	startInformersAndWaitForCacheSync(stopCh, kyvernoInformer, kubeInformer, kubeKyvernoInformer)

	// warmup policy cache
	if err := policyCacheController.WarmUp(); err != nil {
		setupLog.Error(err, "Failed to warm up policy cache")
		os.Exit(1)
	}

	// init events handlers
	// start Kyverno controllers
	go policyCacheController.Run(stopCh)
	go urc.Run(genWorkers, stopCh)
	go le.Run(ctx)
	go reportReqGen.Run(2, stopCh)
	go configurationController.Run(stopCh)
	go eventGenerator.Run(3, stopCh)
	go auditHandler.Run(10, stopCh)
	if !debug {
		go webhookMonitor.Run(webhookCfg, certRenewer, eventGenerator, stopCh)
	}

	// verifies if the admission control is enabled and active
	server.Run(stopCh)

	<-stopCh

	// resource cleanup
	// remove webhook configurations
	<-cleanUp
	setupLog.Info("Kyverno shutdown successful")
}

func startOpenAPIController(client dclient.Interface, stopCh <-chan struct{}) *openapi.Controller {
	openAPIController, err := openapi.NewOpenAPIController()
	if err != nil {
		setupLog.Error(err, "Failed to create openAPIController")
		os.Exit(1)
	}
	// Sync openAPI definitions of resources
	openAPISync := openapi.NewCRDSync(client, openAPIController)
	// start openAPI controller, this is used in admission review
	// thus is required in each instance
	openAPISync.Run(1, stopCh)
	return openAPIController
}
