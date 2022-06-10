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

	"github.com/kyverno/kyverno/pkg/background"
	generatecleanup "github.com/kyverno/kyverno/pkg/background/generate/cleanup"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/controllers/certmanager"
	configcontroller "github.com/kyverno/kyverno/pkg/controllers/config"
	"github.com/kyverno/kyverno/pkg/cosign"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
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
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/kyverno/kyverno/pkg/version"
	"github.com/kyverno/kyverno/pkg/webhookconfig"
	"github.com/kyverno/kyverno/pkg/webhooks"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const resyncPeriod = 15 * time.Minute

var (
	//TODO: this has been added to backward support command line arguments
	// will be removed in future and the configuration will be set only via configmaps
	serverIP                     string
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
	log.SetLogger(klogr.New().WithCallDepth(1))
	flag.IntVar(&webhookTimeout, "webhookTimeout", int(webhookconfig.DefaultWebhookTimeout), "Timeout for webhook configurations.")
	flag.IntVar(&genWorkers, "genWorkers", 10, "Workers for generate controller")
	flag.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flag.BoolVar(&profile, "profile", false, "Set this flag to 'true', to enable profiling.")
	flag.StringVar(&profilePort, "profilePort", "6060", "Enable profiling at given port, defaults to 6060.")
	flag.BoolVar(&disableMetricsExport, "disableMetrics", false, "Set this flag to 'true', to enable exposing the metrics.")
	flag.StringVar(&metricsPort, "metricsPort", "8000", "Expose prometheus metrics at the given port, default to 8000.")
	flag.DurationVar(&policyControllerResyncPeriod, "backgroundScan", time.Hour, "Perform background scan every given interval, e.g., 30s, 15m, 1h.")
	flag.StringVar(&imagePullSecrets, "imagePullSecrets", "", "Secret resource names for image registry access credentials.")
	flag.StringVar(&imageSignatureRepository, "imageSignatureRepository", "", "Alternate repository for image signatures. Can be overridden per rule via `verifyImages.Repository`.")
	flag.BoolVar(&autoUpdateWebhooks, "autoUpdateWebhooks", true, "Set this flag to 'false' to disable auto-configuration of the webhook.")
	flag.Float64Var(&clientRateLimitQPS, "clientRateLimitQPS", 0, "Configure the maximum QPS to the Kubernetes API server from Kyverno. Uses the client default if zero.")
	flag.IntVar(&clientRateLimitBurst, "clientRateLimitBurst", 0, "Configure the maximum burst for throttle. Uses the client default if zero.")
	flag.Func(toggle.AutogenInternalsFlagName, toggle.AutogenInternalsDescription, toggle.AutogenInternalsFlag)
	flag.DurationVar(&webhookRegistrationTimeout, "webhookRegistrationTimeout", 120*time.Second, "Timeout for webhook registration, e.g., 30s, 1m, 5m.")
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
	dynamicClient, err := dclient.NewClient(clientConfig, 15*time.Minute, stopCh)
	if err != nil {
		setupLog.Error(err, "Failed to create dynamic client")
		os.Exit(1)
	}
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		setupLog.Error(err, "Failed to create kubernetes client")
		os.Exit(1)
	}

	// sanity checks
	if !utils.CRDsInstalled(dynamicClient.Discovery()) {
		setupLog.Error(fmt.Errorf("CRDs not installed"), "Failed to access Kyverno CRDs")
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

	// informer factories
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClient, resyncPeriod)
	kubeKyvernoInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace))
	kyvernoInformer := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, policyControllerResyncPeriod)

	// utils
	kyvernoV1 := kyvernoInformer.Kyverno().V1()
	kyvernoV1alpha2 := kyvernoInformer.Kyverno().V1alpha2()

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

	// EVENT GENERATOR
	// - generate event with retry mechanism
	eventGenerator := event.NewEventGenerator(dynamicClient, kyvernoV1.ClusterPolicies(), kyvernoV1.Policies(), log.Log.WithName("EventGenerator"))

	// POLICY Report GENERATOR
	reportReqGen := policyreport.NewReportChangeRequestGenerator(kyvernoClient,
		dynamicClient,
		kyvernoV1alpha2.ReportChangeRequests(),
		kyvernoV1alpha2.ClusterReportChangeRequests(),
		kyvernoV1.ClusterPolicies(),
		kyvernoV1.Policies(),
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
	configurationController := configcontroller.NewController(kubeKyvernoInformer.Core().V1().ConfigMaps(), configuration)

	metricsConfigData, err := config.NewMetricsConfigData(kubeClient)
	if err != nil {
		setupLog.Error(err, "failed to fetch metrics config")
		os.Exit(1)
	}

	if !disableMetricsExport {
		promConfig, err = metrics.NewPromConfig(metricsConfigData)
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
		kyvernoClient,
		dynamicClient,
		kyvernoV1.ClusterPolicies(),
		kyvernoV1.Policies(),
		kyvernoInformer.Kyverno().V1beta1().UpdateRequests(),
		configuration,
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

	urgen := webhookgenerate.NewGenerator(kyvernoClient, kyvernoInformer.Kyverno().V1beta1().UpdateRequests())

	urc := background.NewController(
		kubeClient,
		kyvernoClient,
		dynamicClient,
		kyvernoV1.ClusterPolicies(),
		kyvernoV1.Policies(),
		kyvernoInformer.Kyverno().V1beta1().UpdateRequests(),
		kubeInformer.Core().V1().Namespaces(),
		kubeInformer.Core().V1().Pods(),
		eventGenerator,
		configuration,
	)

	grcc, err := generatecleanup.NewController(
		kubeClient,
		kyvernoClient,
		dynamicClient,
		kyvernoV1.ClusterPolicies(),
		kyvernoV1.Policies(),
		kyvernoInformer.Kyverno().V1beta1().UpdateRequests(),
		kubeInformer.Core().V1().Namespaces(),
		log.Log.WithName("GenerateCleanUpController"),
	)
	if err != nil {
		setupLog.Error(err, "Failed to create generate cleanup controller")
		os.Exit(1)
	}

	pCacheController := policycache.NewPolicyCacheController(kyvernoV1.ClusterPolicies(), kyvernoV1.Policies())

	auditHandler := webhooks.NewValidateAuditHandler(
		pCacheController.Cache,
		eventGenerator,
		reportReqGen,
		kubeInformer.Rbac().V1().RoleBindings(),
		kubeInformer.Rbac().V1().ClusterRoleBindings(),
		kubeInformer.Core().V1().Namespaces(),
		log.Log.WithName("ValidateAuditHandler"),
		configuration,
		dynamicClient,
		promConfig,
	)

	certRenewer := tls.NewCertRenewer(kubeClient, clientConfig, tls.CertRenewalInterval, tls.CertValidityDuration, serverIP, log.Log.WithName("CertRenewer"))
	certManager, err := certmanager.NewController(kubeKyvernoInformer.Core().V1().Secrets(), kubeClient, certRenewer)
	if err != nil {
		setupLog.Error(err, "failed to initialize CertManager")
		os.Exit(1)
	}

	registerWrapperRetry := common.RetryFunc(time.Second, webhookRegistrationTimeout, webhookCfg.Register, "failed to register webhook", setupLog)
	registerWebhookConfigurations := func() {
		certManager.InitTLSPemPair()
		waitForCacheSync(stopCh, kyvernoInformer, kubeInformer, kubeKyvernoInformer)

		// validate the ConfigMap format
		if err := webhookCfg.ValidateWebhookConfigurations(config.KyvernoNamespace, config.KyvernoConfigMapName); err != nil {
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
	webhookRegisterLeader, err := leaderelection.New("webhook-register", config.KyvernoNamespace, kubeClient, registerWebhookConfigurations, nil, log.Log.WithName("webhookRegister/LeaderElection"))
	if err != nil {
		setupLog.Error(err, "failed to elect a leader")
		os.Exit(1)
	}

	go webhookRegisterLeader.Run(ctx)

	// the webhook server runs across all instances
	openAPIController := startOpenAPIController(dynamicClient, stopCh)

	var tlsPair *tls.PemPair
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
		kyvernoClient,
		dynamicClient,
		tlsPair,
		kyvernoInformer.Kyverno().V1beta1().UpdateRequests(),
		kyvernoV1.ClusterPolicies(),
		kubeInformer.Rbac().V1().RoleBindings(),
		kubeInformer.Rbac().V1().ClusterRoleBindings(),
		kubeInformer.Rbac().V1().Roles(),
		kubeInformer.Rbac().V1().ClusterRoles(),
		kubeInformer.Core().V1().Namespaces(),
		eventGenerator,
		pCacheController.Cache,
		webhookCfg,
		webhookMonitor,
		configuration,
		reportReqGen,
		urgen,
		auditHandler,
		cleanUp,
		log.Log.WithName("WebhookServer"),
		openAPIController,
		&urc,
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

	le, err := leaderelection.New("kyverno", config.KyvernoNamespace, kubeClientLeaderElection, run, stop, log.Log.WithName("kyverno/LeaderElection"))
	if err != nil {
		setupLog.Error(err, "failed to elect a leader")
		os.Exit(1)
	}

	startInformersAndWaitForCacheSync(stopCh, kyvernoInformer, kubeInformer, kubeKyvernoInformer)

	pCacheController.CheckPolicySync(stopCh)

	// init events handlers
	// start Kyverno controllers
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
	server.RunAsync(stopCh)

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
