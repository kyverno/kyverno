package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	log "sigs.k8s.io/controller-runtime/pkg/log"

	backwardcompatibility "github.com/kyverno/kyverno/pkg/backward_compatibility"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
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
	"github.com/kyverno/kyverno/pkg/policystatus"
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
	filterK8sResources string
	kubeconfig         string
	serverIP           string
	excludeGroupRole   string
	excludeUsername    string
	profilePort        string
	metricsPort        string

	webhookTimeout int
	genWorkers     int

	profile              bool
	disableMetricsExport bool

	policyControllerResyncPeriod time.Duration
	setupLog                     = log.Log.WithName("setup")
)

func main() {
	klog.InitFlags(nil)
	log.SetLogger(klogr.New())
	flag.StringVar(&filterK8sResources, "filterK8sResources", "", "Resource in format [kind,namespace,name] where policy is not evaluated by the admission webhook. For example, --filterK8sResources \"[Deployment, kyverno, kyverno],[Events, *, *]\"")
	flag.StringVar(&excludeGroupRole, "excludeGroupRole", "", "")
	flag.StringVar(&excludeUsername, "excludeUsername", "", "")
	flag.IntVar(&webhookTimeout, "webhookTimeout", 3, "Timeout for webhook configurations")
	flag.IntVar(&genWorkers, "genWorkers", 10, "Workers for generate controller")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flag.BoolVar(&profile, "profile", false, "Set this flag to 'true', to enable profiling.")
	flag.StringVar(&profilePort, "profilePort", "6060", "Enable profiling at given port, default to 6060.")
	flag.BoolVar(&disableMetricsExport, "disableMetrics", false, "Set this flag to 'true', to enable exposing the metrics.")
	flag.StringVar(&metricsPort, "metrics-port", "8000", "Expose prometheus metrics at the given port, default to 8000.")
	flag.DurationVar(&policyControllerResyncPeriod, "backgroundScan", time.Hour, "Perform background scan every given interval, e.g., 30s, 15m, 1h.")
	if err := flag.Set("v", "2"); err != nil {
		setupLog.Error(err, "failed to set log level")
		os.Exit(1)
	}

	flag.Parse()

	version.PrintVersionInfo(log.Log)
	cleanUp := make(chan struct{})
	stopCh := signal.SetupSignalHandler()
	clientConfig, err := config.CreateClientConfig(kubeconfig, log.Log)
	if err != nil {
		setupLog.Error(err, "Failed to build kubeconfig")
		os.Exit(1)
	}

	var profilingServerMux *http.ServeMux
	var metricsServerMux *http.ServeMux
	var promConfig *metrics.PromConfig

	if profile {
		profilingServerMux = http.NewServeMux()
		addr := ":" + profilePort
		setupLog.Info("Enable profiling, see details at https://github.com/kyverno/kyverno/wiki/Profiling-Kyverno-on-Kubernetes", "port", profilePort)
		go func() {
			if err := http.ListenAndServe(addr, profilingServerMux); err != nil {
				setupLog.Error(err, "Failed to enable profiling")
				os.Exit(1)
			}
		}()

	}

	if !disableMetricsExport {
		promConfig = metrics.NewPromConfig()
		metricsServerMux = http.NewServeMux()
		metricsServerMux.Handle("/metrics", promhttp.HandlerFor(promConfig.MetricsRegistry, promhttp.HandlerOpts{Timeout: 10 * time.Second}))
		metricsAddr := ":" + metricsPort
		setupLog.Info("Enable exposure of metrics, see details at https://github.com/kyverno/kyverno/wiki/Metrics-Kyverno-on-Kubernetes", "port", metricsPort)
		go func() {
			if err := http.ListenAndServe(metricsAddr, metricsServerMux); err != nil {
				setupLog.Error(err, "Failed to enable exposure of metrics")
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
	kubedynamicInformer := client.NewDynamicSharedInformerFactory(resyncPeriod)

	rCache, err := resourcecache.NewResourceCache(client, kubedynamicInformer, log.Log.WithName("resourcecache"))
	if err != nil {
		setupLog.Error(err, "ConfigMap lookup disabled: failed to create resource cache")
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
		rCache,
		log.Log.WithName("EventGenerator"))

	// Policy Status Handler - deals with all logic related to policy status
	statusSync := policystatus.NewSync(
		pclient,
		pInformer.Kyverno().V1().ClusterPolicies().Lister(),
		pInformer.Kyverno().V1().Policies().Lister())

	// POLICY Report GENERATOR
	reportReqGen := policyreport.NewReportChangeRequestGenerator(pclient,
		client,
		pInformer.Kyverno().V1alpha1().ReportChangeRequests(),
		pInformer.Kyverno().V1alpha1().ClusterReportChangeRequests(),
		pInformer.Kyverno().V1().ClusterPolicies(),
		pInformer.Kyverno().V1().Policies(),
		statusSync.Listener,
		log.Log.WithName("ReportChangeRequestGenerator"),
	)

	prgen, err := policyreport.NewReportGenerator(
		kubeClient,
		pclient,
		client,
		pInformer.Wgpolicyk8s().V1alpha1().ClusterPolicyReports(),
		pInformer.Wgpolicyk8s().V1alpha1().PolicyReports(),
		pInformer.Kyverno().V1alpha1().ReportChangeRequests(),
		pInformer.Kyverno().V1alpha1().ClusterReportChangeRequests(),
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
		rCache,
		serverIP,
		int32(webhookTimeout),
		debug,
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
		kubeInformer.Core().V1().ConfigMaps(),
		filterK8sResources,
		excludeGroupRole,
		excludeUsername,
		prgen.ReconcileCh,
		webhookCfg.UpdateWebhookChan,
		log.Log.WithName("ConfigData"),
	)

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
		rCache,
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
		pInformer.Kyverno().V1().GenerateRequests(),
		eventGenerator,
		kubedynamicInformer,
		statusSync.Listener,
		log.Log.WithName("GenerateController"),
		configData,
		rCache,
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
		statusSync.Listener,
		reportReqGen,
		kubeInformer.Rbac().V1().RoleBindings(),
		kubeInformer.Rbac().V1().ClusterRoleBindings(),
		kubeInformer.Core().V1().Namespaces(),
		log.Log.WithName("ValidateAuditHandler"),
		configData,
		rCache,
		client,
		promConfig,
	)

	certRenewer := ktls.NewCertRenewer(client, clientConfig, ktls.CertRenewalInterval, ktls.CertValidityDuration, serverIP, log.Log.WithName("CertRenewer"))
	certManager, err := webhookconfig.NewCertManager(
		kubeInformer.Core().V1().Secrets(),
		kubeClient,
		certRenewer,
		log.Log.WithName("CertManager"),
		stopCh,
	)

	if err != nil {
		setupLog.Error(err, "failed to initialize CertManager")
		os.Exit(1)
	}

	registerWrapperRetry := common.RetryFunc(time.Second, 30*time.Second, webhookCfg.Register, setupLog)
	registerWebhookConfigurations := func() {
		certManager.InitTLSPemPair()

		// validate the ConfigMap format
		if err := webhookCfg.ValidateWebhookConfigurations(config.KyvernoNamespace, configData.GetInitConfigMapName()); err != nil {
			setupLog.Error(err, "invalid format of the Kyverno init ConfigMap, please correct the format of 'data.webhooks'")
			os.Exit(1)
		}

		go webhookCfg.UpdateWebhookConfigurations(configData)
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
		statusSync.Listener,
		configData,
		reportReqGen,
		grgen,
		auditHandler,
		cleanUp,
		log.Log.WithName("WebhookServer"),
		openAPIController,
		rCache,
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
		setupLog.Error(err, "failed to elector leader")
		os.Exit(1)
	}

	// init events handlers
	// start Kyverno controllers
	go le.Run(ctx)

	go reportReqGen.Run(2, stopCh)
	go configData.Run(stopCh)
	go eventGenerator.Run(3, stopCh)
	go grgen.Run(10, stopCh)
	go statusSync.Run(1, stopCh)
	go pCacheController.Run(1, stopCh)
	go auditHandler.Run(10, stopCh)
	if !debug {
		go webhookMonitor.Run(webhookCfg, certRenewer, eventGenerator, stopCh)
	}

	go backwardcompatibility.AddLabels(pclient, pInformer.Kyverno().V1().GenerateRequests())
	go backwardcompatibility.AddCloneLabel(client, pInformer.Kyverno().V1().ClusterPolicies())

	pInformer.Start(stopCh)
	kubeInformer.Start(stopCh)
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
