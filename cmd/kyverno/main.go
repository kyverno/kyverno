package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	backwardcompatibility "github.com/kyverno/kyverno/pkg/backward_compatibility"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	event "github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/generate"
	generatecleanup "github.com/kyverno/kyverno/pkg/generate/cleanup"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/policystatus"
	"github.com/kyverno/kyverno/pkg/resourcecache"
	"github.com/kyverno/kyverno/pkg/signal"
	"github.com/kyverno/kyverno/pkg/utils"
	"github.com/kyverno/kyverno/pkg/version"
	"github.com/kyverno/kyverno/pkg/webhookconfig"
	"github.com/kyverno/kyverno/pkg/webhooks"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/generate"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

const resyncPeriod = 15 * time.Minute

var (
	//TODO: this has been added to backward support command line arguments
	// will be removed in future and the configuration will be set only via configmaps
	filterK8sResources             string
	kubeconfig                     string
	serverIP                       string
	runValidationInMutatingWebhook string
	excludeGroupRole               string
	excludeUsername                string
	profilePort                    string

	webhookTimeout int

	profile      bool
	policyReport bool
	setupLog     = log.Log.WithName("setup")
)

func main() {
	klog.InitFlags(nil)
	log.SetLogger(klogr.New())
	flag.StringVar(&filterK8sResources, "filterK8sResources", "", "k8 resource in format [kind,namespace,name] where policy is not evaluated by the admission webhook. example --filterKind \"[Deployment, kyverno, kyverno]\" --filterKind \"[Deployment, kyverno, kyverno],[Events, *, *]\"")
	flag.StringVar(&excludeGroupRole, "excludeGroupRole", "", "")
	flag.StringVar(&excludeUsername, "excludeUsername", "", "")
	flag.IntVar(&webhookTimeout, "webhooktimeout", 3, "timeout for webhook configurations")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flag.StringVar(&runValidationInMutatingWebhook, "runValidationInMutatingWebhook", "", "Validation will also be done using the mutation webhook, set to 'true' to enable. Older kubernetes versions do not work properly when a validation webhook is registered.")
	flag.BoolVar(&profile, "profile", false, "Set this flag to 'true', to enable profiling.")
	flag.StringVar(&profilePort, "profile-port", "6060", "Enable profiling at given port, default to 6060.")
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
	kubedynamicInformer := client.NewDynamicSharedInformerFactory(resyncPeriod)

	rCache, err := resourcecache.NewResourceCache(client, kubedynamicInformer, log.Log.WithName("resourcecache"))
	if err != nil {
		setupLog.Error(err, "ConfigMap lookup disabled: failed to create resource cache")
	}

	webhookCfg := webhookconfig.NewRegister(
		clientConfig,
		client,
		serverIP,
		int32(webhookTimeout),
		log.Log)

	// Resource Mutating Webhook Watcher
	webhookMonitor := webhookconfig.NewMonitor(log.Log.WithName("WebhookMonitor"))

	// KYVERNO CRD INFORMER
	// watches CRD resources:
	//		- ClusterPolicy, Policy
	//		- ClusterPolicyReport, PolicyReport
	//		- GenerateRequest
	//		- ClusterReportChangeRequest, ReportChangeRequest
	pInformer := kyvernoinformer.NewSharedInformerFactoryWithOptions(pclient, resyncPeriod)

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
		log.Log.WithName("ConfigData"),
	)

	// EVENT GENERATOR
	// - generate event with retry mechanism
	eventGenerator := event.NewEventGenerator(
		client,
		pInformer.Kyverno().V1().ClusterPolicies(),
		log.Log.WithName("EventGenerator"))

	// Policy Status Handler - deals with all logic related to policy status
	statusSync := policystatus.NewSync(
		pclient,
		pInformer.Kyverno().V1().ClusterPolicies().Lister(),
		pInformer.Kyverno().V1().Policies().Lister())

	// POLICY Report GENERATOR
	// -- generate policy report
	var reportReqGen *policyreport.Generator
	var prgen *policyreport.ReportGenerator
	reportReqGen = policyreport.NewReportChangeRequestGenerator(pclient,
		client,
		pInformer.Kyverno().V1alpha1().ReportChangeRequests(),
		pInformer.Kyverno().V1alpha1().ClusterReportChangeRequests(),
		pInformer.Kyverno().V1().ClusterPolicies(),
		pInformer.Kyverno().V1().Policies(),
		statusSync.Listener,
		log.Log.WithName("ReportChangeRequestGenerator"),
	)

	prgen = policyreport.NewReportGenerator(client,
		pInformer.Wgpolicyk8s().V1alpha1().ClusterPolicyReports(),
		pInformer.Wgpolicyk8s().V1alpha1().PolicyReports(),
		pInformer.Kyverno().V1alpha1().ReportChangeRequests(),
		pInformer.Kyverno().V1alpha1().ClusterReportChangeRequests(),
		kubeInformer.Core().V1().Namespaces(),
		log.Log.WithName("PolicyReportGenerator"),
	)

	// POLICY CONTROLLER
	// - reconciliation policy and policy violation
	// - process policy on existing resources
	// - status aggregator: receives stats when a policy is applied & updates the policy status
	policyCtrl, err := policy.NewPolicyController(pclient,
		client,
		pInformer.Kyverno().V1().ClusterPolicies(),
		pInformer.Kyverno().V1().Policies(),
		pInformer.Kyverno().V1().GenerateRequests(),
		configData,
		eventGenerator,
		reportReqGen,
		kubeInformer.Core().V1().Namespaces(),
		log.Log.WithName("PolicyController"),
		rCache,
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
	)

	// Configure certificates
	tlsPair, err := client.InitTLSPemPair(clientConfig, serverIP)
	if err != nil {
		setupLog.Error(err, "Failed to initialize TLS key/certificate pair")
		os.Exit(1)
	}

	// Register webhookCfg
	if err = webhookCfg.Register(); err != nil {
		setupLog.Error(err, "Failed to register admission control webhooks")
		os.Exit(1)
	}

	openAPIController, err := openapi.NewOpenAPIController()
	if err != nil {
		setupLog.Error(err, "Failed to create openAPIController")
		os.Exit(1)
	}

	// Sync openAPI definitions of resources
	openAPISync := openapi.NewCRDSync(client, openAPIController)

	supportMutateValidate := utils.HigherThanKubernetesVersion(client, log.Log, 1, 14, 0)

	// WEBHOOK
	// - https server to provide endpoints called based on rules defined in Mutating & Validation webhook configuration
	// - reports the results based on the response from the policy engine:
	// -- annotations on resources with update details on mutation JSON patches
	// -- generate policy violation resource
	// -- generate events on policy and resource
	debug := serverIP != ""
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
		supportMutateValidate,
		cleanUp,
		log.Log.WithName("WebhookServer"),
		openAPIController,
		rCache,
		grc,
		debug,
	)

	if err != nil {
		setupLog.Error(err, "Failed to create webhook server")
		os.Exit(1)
	}

	// Start the components
	pInformer.Start(stopCh)
	kubeInformer.Start(stopCh)
	kubedynamicInformer.Start(stopCh)

	go reportReqGen.Run(2, stopCh)
	go prgen.Run(1, stopCh)
	go grgen.Run(1, stopCh)
	go configData.Run(stopCh)
	go policyCtrl.Run(2, stopCh)
	go eventGenerator.Run(3, stopCh)
	go grc.Run(1, stopCh)
	go grcc.Run(1, stopCh)
	go statusSync.Run(1, stopCh)
	go pCacheController.Run(1, stopCh)
	go auditHandler.Run(10, stopCh)
	openAPISync.Run(1, stopCh)

	// verifies if the admission control is enabled and active
	server.RunAsync(stopCh)

	go backwardcompatibility.AddLabels(pclient, pInformer.Kyverno().V1().GenerateRequests())
	go backwardcompatibility.AddCloneLabel(client, pInformer.Kyverno().V1().ClusterPolicies())
	<-stopCh

	// by default http.Server waits indefinitely for connections to return to idle and then shuts down
	// adding a threshold will handle zombie connections
	// adjust the context deadline to 5 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()

	// cleanup webhookconfigurations followed by webhook shutdown
	server.Stop(ctx)

	// resource cleanup
	// remove webhook configurations
	<-cleanUp
	setupLog.Info("Kyverno shutdown successful")
}
