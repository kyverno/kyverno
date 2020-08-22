package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/nirmata/kyverno/pkg/openapi"
	"github.com/nirmata/kyverno/pkg/policycache"

	"github.com/nirmata/kyverno/pkg/checker"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions"
	"github.com/nirmata/kyverno/pkg/config"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	event "github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/generate"
	generatecleanup "github.com/nirmata/kyverno/pkg/generate/cleanup"
	"github.com/nirmata/kyverno/pkg/policy"
	"github.com/nirmata/kyverno/pkg/policystatus"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	"github.com/nirmata/kyverno/pkg/signal"
	"github.com/nirmata/kyverno/pkg/utils"
	"github.com/nirmata/kyverno/pkg/version"
	"github.com/nirmata/kyverno/pkg/webhookconfig"
	"github.com/nirmata/kyverno/pkg/webhooks"
	webhookgenerate "github.com/nirmata/kyverno/pkg/webhooks/generate"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

const resyncPeriod = 15 * time.Minute

var (
	kubeconfig                     string
	serverIP                       string
	webhookTimeout                 int
	runValidationInMutatingWebhook string
	profile                        bool
	//TODO: this has been added to backward support command line arguments
	// will be removed in future and the configuration will be set only via configmaps
	filterK8Resources string

	excludeGroupRole string
	excludeUsername  string
	// User FQDN as CSR CN
	fqdncn   bool
	setupLog = log.Log.WithName("setup")
)

func main() {
	klog.InitFlags(nil)
	log.SetLogger(klogr.New())
	flag.StringVar(&filterK8Resources, "filterK8Resources", "", "k8 resource in format [kind,namespace,name] where policy is not evaluated by the admission webhook. example --filterKind \"[Deployment, kyverno, kyverno]\" --filterKind \"[Deployment, kyverno, kyverno],[Events, *, *]\"")
	flag.StringVar(&excludeGroupRole, "excludeGroupRole", "", "")
	flag.StringVar(&excludeUsername, "excludeUsername", "", "")
	flag.IntVar(&webhookTimeout, "webhooktimeout", 3, "timeout for webhook configurations")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flag.StringVar(&runValidationInMutatingWebhook, "runValidationInMutatingWebhook", "", "Validation will also be done using the mutation webhook, set to 'true' to enable. Older kubernetes versions do not work properly when a validation webhook is registered.")
	flag.BoolVar(&profile, "profile", false, "Set this flag to 'true', to enable profiling.")
	if err := flag.Set("v", "2"); err != nil {
		setupLog.Error(err, "failed to set log level")
		os.Exit(1)
	}

	// Generate CSR with CN as FQDN due to https://github.com/nirmata/kyverno/issues/542
	flag.BoolVar(&fqdncn, "fqdn-as-cn", false, "use FQDN as Common Name in CSR")
	flag.Parse()

	if profile {
		go http.ListenAndServe("localhost:6060", nil)
	}

	version.PrintVersionInfo(log.Log)
	cleanUp := make(chan struct{})
	stopCh := signal.SetupSignalHandler()
	clientConfig, err := config.CreateClientConfig(kubeconfig, log.Log)
	if err != nil {
		setupLog.Error(err, "Failed to build kubeconfig")
		os.Exit(1)
	}

	// KYVERNO CRD CLIENT
	// access CRD resources
	//		- Policy
	//		- PolicyViolation
	pclient, err := kyvernoclient.NewForConfig(clientConfig)
	if err != nil {
		setupLog.Error(err, "Failed to create client")
		os.Exit(1)
	}

	// DYNAMIC CLIENT
	// - client for all registered resources
	client, err := dclient.NewClient(clientConfig, 5*time.Minute, stopCh, log.Log)
	if err != nil {
		setupLog.Error(err, "Failed to create client")
		os.Exit(1)
	}

	// CRD CHECK
	// - verify if the CRD for Policy & PolicyViolation are available
	if !utils.CRDInstalled(client.DiscoveryClient, log.Log) {
		setupLog.Error(fmt.Errorf("CRDs not installed"), "Failed to access Kyverno CRDs")
		os.Exit(1)
	}

	kubeClient, err := utils.NewKubeClient(clientConfig)
	if err != nil {
		setupLog.Error(err, "Failed to create kubernetes client")
		os.Exit(1)
	}

	// TODO: To be removed for v1.2.0
	utils.CleanupOldCrd(client, log.Log)

	kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, resyncPeriod)
	kubedynamicInformer := client.NewDynamicSharedInformerFactory(resyncPeriod)

	webhookRegistrationClient := webhookconfig.NewWebhookRegistrationClient(
		clientConfig,
		client,
		serverIP,
		int32(webhookTimeout),
		log.Log)

	// Resource Mutating Webhook Watcher
	lastReqTime := checker.NewLastReqTime(log.Log.WithName("LastReqTime"))
	rWebhookWatcher := webhookconfig.NewResourceWebhookRegister(
		lastReqTime,
		kubeInformer.Admissionregistration().V1beta1().MutatingWebhookConfigurations(),
		kubeInformer.Admissionregistration().V1beta1().ValidatingWebhookConfigurations(),
		webhookRegistrationClient,
		runValidationInMutatingWebhook,
		log.Log.WithName("ResourceWebhookRegister"),
	)

	// KYVERNO CRD INFORMER
	// watches CRD resources:
	//		- Policy
	//		- PolicyVolation
	pInformer := kyvernoinformer.NewSharedInformerFactoryWithOptions(pclient, resyncPeriod)

	// Configuration Data
	// dynamically load the configuration from configMap
	// - resource filters
	// if the configMap is update, the configuration will be updated :D
	configData := config.NewConfigData(
		kubeClient,
		kubeInformer.Core().V1().ConfigMaps(),
		filterK8Resources,
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

	// POLICY VIOLATION GENERATOR
	// -- generate policy violation
	pvgen := policyviolation.NewPVGenerator(pclient,
		client,
		pInformer.Kyverno().V1().ClusterPolicyViolations(),
		pInformer.Kyverno().V1().PolicyViolations(),
		statusSync.Listener,
		log.Log.WithName("PolicyViolationGenerator"),
	)

	// POLICY CONTROLLER
	// - reconciliation policy and policy violation
	// - process policy on existing resources
	// - status aggregator: receives stats when a policy is applied & updates the policy status
	policyCtrl, err := policy.NewPolicyController(pclient,
		client,
		pInformer.Kyverno().V1().ClusterPolicies(),
		pInformer.Kyverno().V1().Policies(),
		pInformer.Kyverno().V1().ClusterPolicyViolations(),
		pInformer.Kyverno().V1().PolicyViolations(),
		configData,
		eventGenerator,
		pvgen,
		rWebhookWatcher,
		kubeInformer.Core().V1().Namespaces(),
		log.Log.WithName("PolicyController"),
	)

	if err != nil {
		setupLog.Error(err, "Failed to create policy controller")
		os.Exit(1)
	}

	// GENERATE REQUEST GENERATOR
	grgen := webhookgenerate.NewGenerator(pclient, stopCh, log.Log.WithName("GenerateRequestGenerator"))

	// GENERATE CONTROLLER
	// - applies generate rules on resources based on generate requests created by webhook
	grc := generate.NewController(
		pclient,
		client,
		pInformer.Kyverno().V1().ClusterPolicies(),
		pInformer.Kyverno().V1().GenerateRequests(),
		eventGenerator,
		kubedynamicInformer,
		statusSync.Listener,
		log.Log.WithName("GenerateController"),
		configData,
	)

	// GENERATE REQUEST CLEANUP
	// -- cleans up the generate requests that have not been processed(i.e. state = [Pending, Failed]) for more than defined timeout
	grcc := generatecleanup.NewController(
		pclient,
		client,
		pInformer.Kyverno().V1().ClusterPolicies(),
		pInformer.Kyverno().V1().GenerateRequests(),
		kubedynamicInformer,
		log.Log.WithName("GenerateCleanUpController"),
	)

	pCacheController := policycache.NewPolicyCacheController(
		pInformer.Kyverno().V1().ClusterPolicies(),
		pInformer.Kyverno().V1().Policies(),
		log.Log.WithName("PolicyCacheController"),
	)

	auditHandler := webhooks.NewValidateAuditHandler(
		pCacheController.Cache,
		eventGenerator,
		statusSync.Listener,
		pvgen,
		kubeInformer.Rbac().V1().RoleBindings(),
		kubeInformer.Rbac().V1().ClusterRoleBindings(),
		log.Log.WithName("ValidateAuditHandler"),
		configData,
	)

	// CONFIGURE CERTIFICATES
	tlsPair, err := client.InitTLSPemPair(clientConfig, fqdncn)
	if err != nil {
		setupLog.Error(err, "Failed to initialize TLS key/certificate pair")
		os.Exit(1)
	}

	// WEBHOOK REGISTRATION
	// - mutating,validatingwebhookconfiguration (Policy)
	// - verifymutatingwebhookconfiguration (Kyverno Deployment)
	// resource webhook confgiuration is generated dynamically in the webhook server and policy controller
	// based on the policy resources created
	if err = webhookRegistrationClient.Register(); err != nil {
		setupLog.Error(err, "Failed to register Admission webhooks")
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
	server, err := webhooks.NewWebhookServer(
		pclient,
		client,
		tlsPair,
		pInformer.Kyverno().V1().ClusterPolicies(),
		kubeInformer.Rbac().V1().RoleBindings(),
		kubeInformer.Rbac().V1().ClusterRoleBindings(),
		kubeInformer.Rbac().V1().Roles(),
		kubeInformer.Rbac().V1().ClusterRoles(),
		eventGenerator,
		pCacheController.Cache,
		webhookRegistrationClient,
		statusSync.Listener,
		configData,
		pvgen,
		grgen,
		rWebhookWatcher,
		auditHandler,
		supportMutateValidate,
		cleanUp,
		log.Log.WithName("WebhookServer"),
		openAPIController,
	)

	if err != nil {
		setupLog.Error(err, "Failed to create webhook server")
		os.Exit(1)
	}

	// Start the components
	pInformer.Start(stopCh)
	kubeInformer.Start(stopCh)
	kubedynamicInformer.Start(stopCh)
	go grgen.Run(1)
	go rWebhookWatcher.Run(stopCh)
	go configData.Run(stopCh)
	go policyCtrl.Run(3, stopCh)
	go eventGenerator.Run(1, stopCh)
	go grc.Run(1, stopCh)
	go grcc.Run(1, stopCh)
	go pvgen.Run(1, stopCh)
	go statusSync.Run(1, stopCh)
	go pCacheController.Run(1, stopCh)
	go auditHandler.Run(10, stopCh)
	openAPISync.Run(1, stopCh)

	// verifys if the admission control is enabled and active
	// resync: 60 seconds
	// deadline: 60 seconds (send request)
	// max deadline: deadline*3 (set the deployment annotation as false)
	server.RunAsync(stopCh)

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
