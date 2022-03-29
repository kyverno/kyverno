package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"

	// We currently accept the risk of exposing pprof and rely on users to protect the endpoint.
	_ "net/http/pprof" // #nosec
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	log "sigs.k8s.io/controller-runtime/pkg/log"

	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/common"
	backwardcompatibility "github.com/kyverno/kyverno/pkg/compatibility"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/cosign"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	event "github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/generate"
	generatecleanup "github.com/kyverno/kyverno/pkg/generate/cleanup"
	"github.com/kyverno/kyverno/pkg/leaderelection"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	"github.com/kyverno/kyverno/pkg/signal"
	ktls "github.com/kyverno/kyverno/pkg/tls"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/kyverno/kyverno/pkg/version"
	"github.com/kyverno/kyverno/pkg/webhookconfig"
	"github.com/kyverno/kyverno/pkg/webhooks"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/generate"
)

const resyncPeriod = 15 * time.Minute

var (
	//TODO: this has been added to backward support command line arguments
	// will be removed in future and the configuration will be set only via configmaps
	filterK8sResources           string
	kubeconfig                   string
	serverIP                     string
	excludeGroupRole             string
	excludeUsername              string
	profilePort                  string
	metricsPort                  string
	webhookTimeout               int
	genWorkers                   int
	profile                      bool
	disableMetricsExport         bool
	autoUpdateWebhooks           bool
	policyControllerResyncPeriod time.Duration
	imagePullSecrets             string
	imageSignatureRepository     string
	clientRateLimitQPS           float64
	clientRateLimitBurst         int
	webhookRegistrationTimeout   time.Duration
	setupLog                     = log.Log.WithName("setup")
)

func main() {
	klog.InitFlags(nil)
	log.SetLogger(klogr.New())
	flag.StringVar(&filterK8sResources, "filterK8sResources", "", "Resource in format [kind,namespace,name] where policy is not evaluated by the admission webhook. For example, --filterK8sResources \"[Deployment, kyverno, kyverno],[Events, *, *]\"")
	flag.StringVar(&excludeGroupRole, "excludeGroupRole", "", "")
	flag.StringVar(&excludeUsername, "excludeUsername", "", "")
	flag.IntVar(&webhookTimeout, "webhooktimeout", int(webhookconfig.DefaultWebhookTimeout), "Timeout for webhook configurations. Deprecated and will be removed in 1.6.0.")
	flag.IntVar(&webhookTimeout, "webhookTimeout", int(webhookconfig.DefaultWebhookTimeout), "Timeout for webhook configurations.")
	// deprecated
	flag.IntVar(&genWorkers, "gen-workers", 10, "Workers for generate controller. Deprecated and will be removed in 1.6.0. ")
	flag.IntVar(&genWorkers, "genWorkers", 10, "Workers for generate controller")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flag.BoolVar(&profile, "profile", false, "Set this flag to 'true', to enable profiling.")
	// deprecated
	flag.StringVar(&profilePort, "profile-port", "6060", "Enable profiling at given port, defaults to 6060. Deprecated and will be removed in 1.6.0. ")
	flag.StringVar(&profilePort, "profilePort", "6060", "Enable profiling at given port, defaults to 6060.")
	// deprecated
	flag.BoolVar(&disableMetricsExport, "disable-metrics", false, "Set this flag to 'true', to enable exposing the metrics. Deprecated and will be removed in 1.6.0. ")
	flag.BoolVar(&disableMetricsExport, "disableMetrics", false, "Set this flag to 'true', to enable exposing the metrics.")
	// deprecated
	flag.StringVar(&metricsPort, "metrics-port", "8000", "Expose prometheus metrics at the given port, default to 8000. Deprecated and will be removed in 1.6.0. ")
	flag.StringVar(&metricsPort, "metricsPort", "8000", "Expose prometheus metrics at the given port, default to 8000.")
	// deprecated
	flag.DurationVar(&policyControllerResyncPeriod, "background-scan", time.Hour, "Perform background scan every given interval, e.g., 30s, 15m, 1h. Deprecated and will be removed in 1.6.0. ")
	flag.DurationVar(&policyControllerResyncPeriod, "backgroundScan", time.Hour, "Perform background scan every given interval, e.g., 30s, 15m, 1h.")
	flag.StringVar(&imagePullSecrets, "imagePullSecrets", "", "Secret resource names for image registry access credentials.")
	flag.StringVar(&imageSignatureRepository, "imageSignatureRepository", "", "Alternate repository for image signatures. Can be overridden per rule via `verifyImages.Repository`.")
	flag.BoolVar(&autoUpdateWebhooks, "autoUpdateWebhooks", true, "Set this flag to 'false' to disable auto-configuration of the webhook.")
	flag.Float64Var(&clientRateLimitQPS, "clientRateLimitQPS", 0, "Configure the maximum QPS to the master from Kyverno. Uses the client default if zero.")
	flag.IntVar(&clientRateLimitBurst, "clientRateLimitBurst", 0, "Configure the maximum burst for throttle. Uses the client default if zero.")

	flag.DurationVar(&webhookRegistrationTimeout, "webhookRegistrationTimeout", 120*time.Second, "Timeout for webhook registration, e.g., 30s, 1m, 5m.")
	if err := flag.Set("v", "2"); err != nil {
		setupLog.Error(err, "failed to set log level")
		os.Exit(1)
	}

	flag.Parse()

	version.PrintVersionInfo(log.Log)
	cleanUp := make(chan struct{})
	stopCh := signal.SetupSignalHandler()
	clientConfig, err := config.CreateClientConfig(kubeconfig, clientRateLimitQPS, clientRateLimitBurst, log.Log)
	if err != nil {
		setupLog.Error(err, "Failed to build kubeconfig")
		os.Exit(1)
	}

	var metricsServerMux *http.ServeMux
	var promConfig *metrics.PromConfig

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

	// KYVERNO CRD CLIENT
	// access CRD resources
	//		- ClusterPolicy, Policy
	//		- ClusterPolicyReport, PolicyReport
	//		- GenerateRequest
	pclient, err := kyvernoclient.NewForConfig(clientConfig)
	if err != nil {
		setupLog.Error(err, "Failed to create client")
		os.Exit(1)
	}

	// DYNAMIC CLIENT
	// - client for all registered resources
	client, err := dclient.NewClient(clientConfig, 15*time.Minute, stopCh, log.Log)
	if err != nil {
		setupLog.Error(err, "Failed to create client")
		os.Exit(1)
	}

	// CRD CHECK
	// - verify if Kyverno CRDs are available
	if !utils.CRDsInstalled(client.DiscoveryClient) {
		setupLog.Error(fmt.Errorf("CRDs not installed"), "Failed to access Kyverno CRDs")
		os.Exit(1)
	}

	kubeClient, err := utils.NewKubeClient(clientConfig)
	if err != nil {
		setupLog.Error(err, "Failed to create kubernetes client")
		os.Exit(1)
	}

	kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod)
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace))
	kubedynamicInformer := client.NewDynamicSharedInformerFactory(resyncPeriod)

	rCache, err := resourcecache.NewResourceCache(client, kubedynamicInformer, log.Log.WithName("resourcecache"))
	if err != nil {
		setupLog.Error(err, "ConfigMap lookup disabled: failed to create resource cache")
		os.Exit(1)
	}

	// load image registry secrets
	secrets := strings.Split(imagePullSecrets, ",")
	if imagePullSecrets != "" && len(secrets) > 0 {
		setupLog.Info("initializing registry credentials", "secrets", secrets)
		if err := registryclient.Initialize(kubeClient, config.KyvernoNamespace, "", secrets); err != nil {
			setupLog.Error(err, "failed to initialize image pull secrets")
			os.Exit(1)
		}
	}

	if imageSignatureRepository != "" {
		cosign.ImageSignatureRepository = imageSignatureRepository
	}

	// KYVERNO CRD INFORMER
	// watches CRD resources:
	//		- ClusterPolicy, Policy
	//		- ClusterPolicyReport, PolicyReport
	//		- GenerateRequest
	//		- ClusterReportChangeRequest, ReportChangeRequest
	pInformer := kyvernoinformer.NewSharedInformerFactoryWithOptions(pclient, policyControllerResyncPeriod)

	// EVENT GENERATOR
	// - generate event with retry mechanism
	eventGenerator := event.NewEventGenerator(
		client,
		pInformer.Kyverno().V1().ClusterPolicies(),
		pInformer.Kyverno().V1().Policies(),
		log.Log.WithName("EventGenerator"))

	// POLICY Report GENERATOR
	reportReqGen := policyreport.NewReportChangeRequestGenerator(pclient,
		client,
		pInformer.Kyverno().V1alpha2().ReportChangeRequests(),
		pInformer.Kyverno().V1alpha2().ClusterReportChangeRequests(),
		pInformer.Kyverno().V1().ClusterPolicies(),
		pInformer.Kyverno().V1().Policies(),
		log.Log.WithName("ReportChangeRequestGenerator"),
	)

	prgen, err := policyreport.NewReportGenerator(
		pclient,
		client,
		pInformer.Wgpolicyk8s().V1alpha2().ClusterPolicyReports(),
		pInformer.Wgpolicyk8s().V1alpha2().PolicyReports(),
		pInformer.Kyverno().V1alpha2().ReportChangeRequests(),
		pInformer.Kyverno().V1alpha2().ClusterReportChangeRequests(),
		kubeInformer.Core().V1().Namespaces(),
		log.Log.WithName("PolicyReportGenerator"),
	)

	if err != nil {
		setupLog.Error(err, "Failed to create policy report controller")
		os.Exit(1)
	}

	debug := serverIP != ""
	webhookCfg := webhookconfig.NewRegister(
		clientConfig,
		client,
		pclient,
		kubeInformer.Admissionregistration().V1().MutatingWebhookConfigurations(),
		kubeInformer.Admissionregistration().V1().ValidatingWebhookConfigurations(),
		rCache,
		kubeKyvernoInformer.Apps().V1().Deployments(),
		kubeInformer.Core().V1().Namespaces(),
		pInformer.Kyverno().V1().ClusterPolicies(),
		pInformer.Kyverno().V1().Policies(),
		serverIP,
		int32(webhookTimeout),
		debug,
		autoUpdateWebhooks,
		stopCh,
		log.Log)

	webhookMonitor, err := webhookconfig.NewMonitor(kubeClient, log.Log.WithName("WebhookMonitor"))
	if err != nil {
		setupLog.Error(err, "failed to initialize webhookMonitor")
		os.Exit(1)
	}

	// Configuration Data
	// dynamically load the configuration from configMap
	// - resource filters
	// if the configMap is update, the configuration will be updated :D
	configData := config.NewConfigData(
		kubeClient,
		kubeKyvernoInformer.Core().V1().ConfigMaps(),
		filterK8sResources,
		excludeGroupRole,
		excludeUsername,
		prgen.ReconcileCh,
		webhookCfg.UpdateWebhookChan,
		log.Log.WithName("ConfigData"),
	)

	metricsConfigData, err := config.NewMetricsConfigData(
		kubeClient,
		log.Log.WithName("MetricsConfigData"),
	)
	if err != nil {
		setupLog.Error(err, "failed to fetch metrics config")
		os.Exit(1)
	}

	if !disableMetricsExport {
		promConfig, err = metrics.NewPromConfig(metricsConfigData, log.Log.WithName("MetricsConfig"))
		if err != nil {
			setupLog.Error(err, "failed to setup Prometheus metric configuration")
			os.Exit(1)
		}
		metricsServerMux = http.NewServeMux()
		metricsServerMux.Handle("/metrics", promhttp.HandlerFor(promConfig.MetricsRegistry, promhttp.HandlerOpts{Timeout: 10 * time.Second}))
		metricsAddr := ":" + metricsPort
		go func() {
			setupLog.Info("enabling metrics service", "address", metricsAddr)
			if err := http.ListenAndServe(metricsAddr, metricsServerMux); err != nil {
				setupLog.Error(err, "failed to enable metrics service", "address", metricsAddr)
				os.Exit(1)
			}
		}()
	}

	// POLICY CONTROLLER
	// - reconciliation policy and policy violation
	// - process policy on existing resources
	// - status aggregator: receives stats when a policy is applied & updates the policy status
	policyCtrl, err := policy.NewPolicyController(
		kubeClient,
		pclient,
		client,
		pInformer.Kyverno().V1().ClusterPolicies(),
		pInformer.Kyverno().V1().Policies(),
		pInformer.Kyverno().V1().GenerateRequests(),
		configData,
		eventGenerator,
		reportReqGen,
		prgen,
		kubeInformer.Core().V1().Namespaces(),
		log.Log.WithName("PolicyController"),
		policyControllerResyncPeriod,
		promConfig,
	)

	if err != nil {
		setupLog.Error(err, "Failed to create policy controller")
		os.Exit(1)
	}

	// GENERATE REQUEST GENERATOR
	grgen := webhookgenerate.NewGenerator(pclient, pInformer.Kyverno().V1().GenerateRequests(), stopCh, log.Log.WithName("GenerateRequestGenerator"))

	// GENERATE CONTROLLER
	// - applies generate rules on resources based on generate requests created by webhook
	grc, err := generate.NewController(
		kubeClient,
		pclient,
		client,
		pInformer.Kyverno().V1().ClusterPolicies(),
		pInformer.Kyverno().V1().Policies(),
		pInformer.Kyverno().V1().GenerateRequests(),
		eventGenerator,
		kubedynamicInformer,
		log.Log.WithName("GenerateController"),
		configData,
	)
	if err != nil {
		setupLog.Error(err, "Failed to create generate controller")
		os.Exit(1)
	}

	// GENERATE REQUEST CLEANUP
	// -- cleans up the generate requests that have not been processed(i.e. state = [Pending, Failed]) for more than defined timeout
	grcc, err := generatecleanup.NewController(
		kubeClient,
		pclient,
		client,
		pInformer.Kyverno().V1().ClusterPolicies(),
		pInformer.Kyverno().V1().Policies(),
		pInformer.Kyverno().V1().GenerateRequests(),
		kubedynamicInformer,
		log.Log.WithName("GenerateCleanUpController"),
	)
	if err != nil {
		setupLog.Error(err, "Failed to create generate cleanup controller")
		os.Exit(1)
	}

	pCacheController := policycache.NewPolicyCacheController(
		pInformer.Kyverno().V1().ClusterPolicies(),
		pInformer.Kyverno().V1().Policies(),
		log.Log.WithName("PolicyCacheController"),
	)

	auditHandler := webhooks.NewValidateAuditHandler(
		pCacheController.Cache,
		eventGenerator,
		reportReqGen,
		kubeInformer.Rbac().V1().RoleBindings(),
		kubeInformer.Rbac().V1().ClusterRoleBindings(),
		kubeInformer.Core().V1().Namespaces(),
		log.Log.WithName("ValidateAuditHandler"),
		configData,
		client,
		promConfig,
	)

	certRenewer := ktls.NewCertRenewer(client, clientConfig, ktls.CertRenewalInterval, ktls.CertValidityDuration, serverIP, log.Log.WithName("CertRenewer"))
	certManager, err := webhookconfig.NewCertManager(
		kubeKyvernoInformer.Core().V1().Secrets(),
		kubeClient,
		certRenewer,
		log.Log.WithName("CertManager"),
		stopCh,
	)

	if err != nil {
		setupLog.Error(err, "failed to initialize CertManager")
		os.Exit(1)
	}

	registerWrapperRetry := common.RetryFunc(time.Second, webhookRegistrationTimeout, webhookCfg.Register, setupLog)
	registerWebhookConfigurations := func() {
		certManager.InitTLSPemPair()
		webhookCfg.Start()

		// validate the ConfigMap format
		if err := webhookCfg.ValidateWebhookConfigurations(config.KyvernoNamespace, configData.GetInitConfigMapName()); err != nil {
			setupLog.Error(err, "invalid format of the Kyverno init ConfigMap, please correct the format of 'data.webhooks'")
			os.Exit(1)
		}

		if autoUpdateWebhooks {
			go webhookCfg.UpdateWebhookConfigurations(configData)
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
	webhookRegisterLeader, err := leaderelection.New("webhook-register", config.KyvernoNamespace, kubeClient, registerWebhookConfigurations, nil, log.Log.WithName("webhookRegister/LeaderElection"))
	if err != nil {
		setupLog.Error(err, "failed to elector leader")
		os.Exit(1)
	}

	go webhookRegisterLeader.Run(ctx)

	// the webhook server runs across all instances
	openAPIController := startOpenAPIController(client, stopCh)

	var tlsPair *ktls.PemPair
	tlsPair, err = certManager.GetTLSPemPair()
	if err != nil {
		setupLog.Error(err, "Failed to get TLS key/certificate pair")
		os.Exit(1)
	}

	// WEBHOOK
	// - https server to provide endpoints called based on rules defined in Mutating & Validation webhook configuration
	// - reports the results based on the response from the policy engine:
	// -- annotations on resources with update details on mutation JSON patches
	// -- generate policy violation resource
	// -- generate events on policy and resource
	server, err := webhooks.NewWebhookServer(
		pclient,
		client,
		tlsPair,
		pInformer.Kyverno().V1().GenerateRequests(),
		pInformer.Kyverno().V1().ClusterPolicies(),
		kubeInformer.Rbac().V1().RoleBindings(),
		kubeInformer.Rbac().V1().ClusterRoleBindings(),
		kubeInformer.Rbac().V1().Roles(),
		kubeInformer.Rbac().V1().ClusterRoles(),
		kubeInformer.Core().V1().Namespaces(),
		eventGenerator,
		pCacheController.Cache,
		webhookCfg,
		webhookMonitor,
		configData,
		reportReqGen,
		grgen,
		auditHandler,
		cleanUp,
		log.Log.WithName("WebhookServer"),
		openAPIController,
		grc,
		promConfig,
	)

	if err != nil {
		setupLog.Error(err, "Failed to create webhook server")
		os.Exit(1)
	}

	// wrap all controllers that need leaderelection
	// start them once by the leader
	run := func() {
		go certManager.Run(stopCh)
		go policyCtrl.Run(2, prgen.ReconcileCh, stopCh)
		go prgen.Run(1, stopCh)
		go grc.Run(genWorkers, stopCh)
		go grcc.Run(1, stopCh)
	}

	kubeClientLeaderElection, err := utils.NewKubeClient(clientConfig)
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

	le, err := leaderelection.New("kyverno", config.KyvernoNamespace, kubeClientLeaderElection, run, stop, log.Log.WithName("kyverno/LeaderElection"))
	if err != nil {
		setupLog.Error(err, "failed to elect a leader")
		os.Exit(1)
	}

	// init events handlers
	// start Kyverno controllers
	go le.Run(ctx)

	go reportReqGen.Run(2, stopCh)
	go configData.Run(stopCh)
	go eventGenerator.Run(3, stopCh)
	go grgen.Run(10, stopCh)
	go pCacheController.Run(1, stopCh)
	go auditHandler.Run(10, stopCh)
	if !debug {
		go webhookMonitor.Run(webhookCfg, certRenewer, eventGenerator, stopCh)
	}

	go backwardcompatibility.AddLabels(pclient, pInformer.Kyverno().V1().GenerateRequests())
	go backwardcompatibility.AddCloneLabel(client, pInformer.Kyverno().V1().ClusterPolicies())

	pInformer.Start(stopCh)
	kubeInformer.Start(stopCh)
	kubeKyvernoInformer.Start(stopCh)
	kubedynamicInformer.Start(stopCh)

	// verifies if the admission control is enabled and active
	server.RunAsync(stopCh)

	<-stopCh

	// resource cleanup
	// remove webhook configurations
	<-cleanUp
	setupLog.Info("Kyverno shutdown successful")
}

func startOpenAPIController(client *dclient.Client, stopCh <-chan struct{}) *openapi.Controller {
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
