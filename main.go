package main

import (
	"context"
	"flag"
	"time"

	"github.com/golang/glog"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	event "github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/namespace"
	"github.com/nirmata/kyverno/pkg/policy"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	"github.com/nirmata/kyverno/pkg/signal"
	"github.com/nirmata/kyverno/pkg/utils"
	"github.com/nirmata/kyverno/pkg/webhookconfig"
	"github.com/nirmata/kyverno/pkg/webhooks"
	kubeinformers "k8s.io/client-go/informers"
)

var (
	kubeconfig     string
	serverIP       string
	cpu            bool
	memory         bool
	webhookTimeout int
	//TODO: this has been added to backward support command line arguments
	// will be removed in future and the configuration will be set only via configmaps
	filterK8Resources string
)

// TODO: tune resync time differently for each informer
const defaultReSyncTime = 10 * time.Second

func main() {
	defer glog.Flush()
	printVersionInfo()
	// profile cpu and memory consuption
	prof = enableProfiling(cpu, memory)
	// cleanUp Channel
	cleanUp := make(chan struct{})
	//  handle os signals
	stopCh := signal.SetupSignalHandler()
	// CLIENT CONFIG
	clientConfig, err := createClientConfig(kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %v\n", err)
	}

	// KYVENO CRD CLIENT
	// access CRD resources
	//		- Policy
	//		- PolicyViolation
	pclient, err := kyvernoclient.NewForConfig(clientConfig)
	if err != nil {
		glog.Fatalf("Error creating client: %v\n", err)
	}

	// DYNAMIC CLIENT
	// - client for all registered resources
	client, err := client.NewClient(clientConfig)
	if err != nil {
		glog.Fatalf("Error creating client: %v\n", err)
	}

	// CRD CHECK
	// - verify if the CRD for Policy & PolicyViolation are avialalbe
	if !utils.CRDInstalled(client.DiscoveryClient) {
		glog.Fatalf("Required CRDs unavailable")
	}
	// KUBERNETES CLIENT
	kubeClient, err := utils.NewKubeClient(clientConfig)
	if err != nil {
		glog.Fatalf("Error creating kubernetes client: %v\n", err)
	}

	// WERBHOOK REGISTRATION CLIENT
	webhookRegistrationClient, err := webhookconfig.NewWebhookRegistrationClient(clientConfig, client, serverIP, int32(webhookTimeout))
	if err != nil {
		glog.Fatalf("Unable to register admission webhooks on cluster: %v\n", err)
	}

	// KYVERNO CRD INFORMER
	// watches CRD resources:
	//		- Policy
	//		- PolicyVolation
	// - cache resync time: 10 seconds
	pInformer := kyvernoinformer.NewSharedInformerFactoryWithOptions(pclient, 10*time.Second)

	// KUBERNETES RESOURCES INFORMER
	// watches namespace resource
	// - cache resync time: 10 seconds
	kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 10*time.Second)

	// Configuration Data
	// dyamically load the configuration from configMap
	// - resource filters
	// if the configMap is update, the configuration will be updated :D
	configData := config.NewConfigData(kubeClient, kubeInformer.Core().V1().ConfigMaps(), filterK8Resources)

	// EVENT GENERATOR
	// - generate event with retry mechanism
	egen := event.NewEventGenerator(client, pInformer.Kyverno().V1alpha1().ClusterPolicies())

	// POLICY CONTROLLER
	// - reconciliation policy and policy violation
	// - process policy on existing resources
	// - status aggregator: recieves stats when a policy is applied
	//					    & updates the policy status
	pc, err := policy.NewPolicyController(pclient, client, pInformer.Kyverno().V1alpha1().ClusterPolicies(), pInformer.Kyverno().V1alpha1().ClusterPolicyViolations(), egen, kubeInformer.Admissionregistration().V1beta1().MutatingWebhookConfigurations(), webhookRegistrationClient, configData)
	if err != nil {
		glog.Fatalf("error creating policy controller: %v\n", err)
	}

	// POLICY VIOLATION CONTROLLER
	// policy violation cleanup if the corresponding resource is deleted
	// status: lastUpdatTime
	pvc, err := policyviolation.NewPolicyViolationController(client, pclient, pInformer.Kyverno().V1alpha1().ClusterPolicies(), pInformer.Kyverno().V1alpha1().ClusterPolicyViolations())
	if err != nil {
		glog.Fatalf("error creating policy violation controller: %v\n", err)
	}

	// GENERATE CONTROLLER
	// - watches for Namespace resource and generates resource based on the policy generate rule
	nsc := namespace.NewNamespaceController(pclient, client, kubeInformer.Core().V1().Namespaces(), pInformer.Kyverno().V1alpha1().ClusterPolicies(), pInformer.Kyverno().V1alpha1().ClusterPolicyViolations(), pc.GetPolicyStatusAggregator(), egen, configData)

	// CONFIGURE CERTIFICATES
	tlsPair, err := initTLSPemPair(clientConfig, client)
	if err != nil {
		glog.Fatalf("Failed to initialize TLS key/certificate pair: %v\n", err)
	}

	// WEBHOOK REGISTRATION
	// - validationwebhookconfiguration (Policy)
	// - mutatingwebhookconfiguration (All resources)
	// webhook confgiuration is also generated dynamically in the policy controller
	// based on the policy resources created
	if err = webhookRegistrationClient.Register(); err != nil {
		glog.Fatalf("Failed registering Admission Webhooks: %v\n", err)
	}

	// WEBHOOOK
	// - https server to provide endpoints called based on rules defined in Mutating & Validation webhook configuration
	// - reports the results based on the response from the policy engine:
	// -- annotations on resources with update details on mutation JSON patches
	// -- generate policy violation resource
	// -- generate events on policy and resource
	server, err := webhooks.NewWebhookServer(pclient, client, tlsPair, pInformer.Kyverno().V1alpha1().ClusterPolicies(), pInformer.Kyverno().V1alpha1().ClusterPolicyViolations(), egen, webhookRegistrationClient, pc.GetPolicyStatusAggregator(), configData, cleanUp)
	if err != nil {
		glog.Fatalf("Unable to create webhook server: %v\n", err)
	}
	// Start the components
	pInformer.Start(stopCh)
	kubeInformer.Start(stopCh)
	if err := configData.Run(kubeInformer.Core().V1().ConfigMaps(), stopCh); err != nil {
		glog.Fatalf("Unable loading dynamic configuration: %v\n", err)
	}
	go pc.Run(1, stopCh)
	go pvc.Run(1, stopCh)
	go egen.Run(1, stopCh)
	go nsc.Run(1, stopCh)

	server.RunAsync()
	<-stopCh
	disableProfiling(prof)
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
	glog.Info("successful shutdown of kyverno controller")
}

func init() {
	// profiling feature gate
	// cpu and memory profiling cannot be enabled at same time
	// if both cpu and memory are enabled
	// by default is to profile cpu
	flag.BoolVar(&cpu, "cpu", false, "cpu profilling feature gate, default to false || cpu and memory profiling cannot be enabled at the same time")
	flag.BoolVar(&memory, "memory", false, "memory profilling feature gate, default to false || cpu and memory profiling cannot be enabled at the same time")
	//TODO: this has been added to backward support command line arguments
	// will be removed in future and the configuration will be set only via configmaps
	flag.StringVar(&filterK8Resources, "filterK8Resources", "", "k8 resource in format [kind,namespace,name] where policy is not evaluated by the admission webhook. example --filterKind \"[Deployment, kyverno, kyverno]\" --filterKind \"[Deployment, kyverno, kyverno],[Events, *, *]\"")
	flag.IntVar(&webhookTimeout, "webhooktimeout", 3, "timeout for webhook configurations")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	config.LogDefaultFlags()
	flag.Parse()
}
