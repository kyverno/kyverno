package main

import (
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
	"github.com/nirmata/kyverno/pkg/utils"
	"github.com/nirmata/kyverno/pkg/webhooks"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/sample-controller/pkg/signals"
)

var (
	kubeconfig        string
	serverIP          string
	filterK8Resources string
	cpu               bool
	memory            bool
	webhookTimeout    int
)

func main() {
	defer glog.Flush()
	printVersionInfo()
	// profile cpu and memory consuption
	prof = enableProfiling(cpu, memory)

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

	// KUBERNETES CLIENT
	kubeClient, err := utils.NewKubeClient(clientConfig)
	if err != nil {
		glog.Fatalf("Error creating kubernetes client: %v\n", err)
	}

	// KYVERNO CRD INFORMER
	// watches CRD resources:
	//		- Policy
	//		- PolicyVolation
	// - cache resync time: 10 seconds
	pInformer := kyvernoinformer.NewSharedInformerFactoryWithOptions(pclient, 10*time.Second)

	// EVENT GENERATOR
	// - generate event with retry mechanism
	egen := event.NewEventGenerator(client, pInformer.Kyverno().V1alpha1().Policies())

	// POLICY CONTROLLER
	// - reconciliation policy and policy violation
	// - process policy on existing resources
	// - status aggregator: recieves stats when a policy is applied
	//					    & updates the policy status
	pc, err := policy.NewPolicyController(pclient, client, pInformer.Kyverno().V1alpha1().Policies(), pInformer.Kyverno().V1alpha1().PolicyViolations(), egen)
	if err != nil {
		glog.Fatalf("error creating policy controller: %v\n", err)
	}

	// POLICY VIOLATION CONTROLLER
	// policy violation cleanup if the corresponding resource is deleted
	// status: lastUpdatTime
	pvc, err := policyviolation.NewPolicyViolationController(client, pclient, pInformer.Kyverno().V1alpha1().Policies(), pInformer.Kyverno().V1alpha1().PolicyViolations())
	if err != nil {
		glog.Fatalf("error creating policy violation controller: %v\n", err)
	}

	// KUBERNETES RESOURCES INFORMER
	// watches namespace resource
	// - cache resync time: 10 seconds
	kubeInformer := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 10*time.Second)

	// GENERATE CONTROLLER
	// - watches for Namespace resource and generates resource based on the policy generate rule
	nsc := namespace.NewNamespaceController(pclient, client, kubeInformer.Core().V1().Namespaces(), pInformer.Kyverno().V1alpha1().Policies(), pInformer.Kyverno().V1alpha1().PolicyViolations(), pc.GetPolicyStatusAggregator(), egen)

	// CONFIGURE CERTIFICATES
	tlsPair, err := initTLSPemPair(clientConfig, client)
	if err != nil {
		glog.Fatalf("Failed to initialize TLS key/certificate pair: %v\n", err)
	}

	// WERBHOOK REGISTRATION CLIENT
	webhookRegistrationClient, err := webhooks.NewWebhookRegistrationClient(clientConfig, client, serverIP, int32(webhookTimeout))
	if err != nil {
		glog.Fatalf("Unable to register admission webhooks on cluster: %v\n", err)
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
	server, err := webhooks.NewWebhookServer(pclient, client, tlsPair, pInformer.Kyverno().V1alpha1().Policies(), pInformer.Kyverno().V1alpha1().PolicyViolations(), egen, webhookRegistrationClient, pc.GetPolicyStatusAggregator(), filterK8Resources)
	if err != nil {
		glog.Fatalf("Unable to create webhook server: %v\n", err)
	}

	stopCh := signals.SetupSignalHandler()

	// Start the components
	pInformer.Start(stopCh)
	kubeInformer.Start(stopCh)
	go pc.Run(1, stopCh)
	go pvc.Run(1, stopCh)
	go egen.Run(1, stopCh)
	go nsc.Run(1, stopCh)

	//TODO add WG for the go routines?
	server.RunAsync()

	<-stopCh
	disableProfiling(prof)
	server.Stop()
}

func init() {
	// profiling feature gate
	// cpu and memory profiling cannot be enabled at same time
	// if both cpu and memory are enabled
	// by default is to profile cpu
	flag.BoolVar(&cpu, "cpu", false, "cpu profilling feature gate, default to false || cpu and memory profiling cannot be enabled at the same time")
	flag.BoolVar(&memory, "memory", false, "memory profilling feature gate, default to false || cpu and memory profiling cannot be enabled at the same time")

	flag.IntVar(&webhookTimeout, "webhooktimeout", 2, "timeout for webhook configurations")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flag.StringVar(&filterK8Resources, "filterK8Resources", "", "k8 resource in format [kind,namespace,name] where policy is not evaluated by the admission webhook. example --filterKind \"[Deployment, kyverno, kyverno]\" --filterKind \"[Deployment, kyverno, kyverno],[Events, *, *]\"")
	config.LogDefaultFlags()
	flag.Parse()
}
