package main

import (
	"flag"

	"github.com/golang/glog"
	clientNew "github.com/nirmata/kyverno/pkg/clientNew/clientset/versioned"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/clientNew/informers/externalversions"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	event "github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/policy"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	"github.com/nirmata/kyverno/pkg/webhooks"
	"k8s.io/sample-controller/pkg/signals"
)

var (
	kubeconfig        string
	serverIP          string
	filterK8Resources string
)

func main() {
	defer glog.Flush()
	printVersionInfo()

	// CLIENT CONFIG
	clientConfig, err := createClientConfig(kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %v\n", err)
	}

	// DYNAMIC CLIENT
	// - client for all registered resources
	client, err := client.NewClient(clientConfig)
	if err != nil {
		glog.Fatalf("Error creating client: %v\n", err)
	}

	// KYVENO CRD CLIENT
	// access CRD resources
	//		- Policy
	//		- PolicyViolation
	pclient, err := clientNew.NewForConfig(clientConfig)
	if err != nil {
		glog.Fatalf("Error creating client: %v\n", err)
	}

	// KYVERNO CRD INFORMER
	// watches CRD resources:
	//		- Policy
	//		- PolicyVolation
	// - cache resync time: 10 seconds
	pInformer := kyvernoinformer.NewSharedInformerFactoryWithOptions(pclient, 10)

	// POLICY CONTROLLER
	// - reconciliation policy and policy violation
	// - status: violation count
	pc, err := policy.NewPolicyController(pclient, client, pInformer.Kyverno().V1alpha1().Policies(), pInformer.Kyverno().V1alpha1().PolicyViolations())
	if err != nil {
		glog.Fatalf("error creating policy controller: %v\n", err)
	}

	// POLICY VIOLATION CONTROLLER
	// status: lastUpdatTime
	pvc, err := policyviolation.NewPolicyViolationController(client, pclient, pInformer.Kyverno().V1alpha1().Policies(), pInformer.Kyverno().V1alpha1().PolicyViolations())
	if err != nil {
		glog.Fatalf("error creating policy violation controller: %v\n", err)
	}

	// EVENT GENERATOR
	// - generate event with retry
	egen := event.NewEventGenerator(client, pInformer.Kyverno().V1alpha1().Policies())

	// TODO : Process Existing
	tlsPair, err := initTLSPemPair(clientConfig, client)
	if err != nil {
		glog.Fatalf("Failed to initialize TLS key/certificate pair: %v\n", err)
	}
	server, err := webhooks.NewWebhookServer(client, tlsPair, pInformer.Kyverno().V1alpha1().Policies(), egen, filterK8Resources)
	if err != nil {
		glog.Fatalf("Unable to create webhook server: %v\n", err)
	}

	webhookRegistrationClient, err := webhooks.NewWebhookRegistrationClient(clientConfig, client, serverIP)
	if err != nil {
		glog.Fatalf("Unable to register admission webhooks on cluster: %v\n", err)
	}

	stopCh := signals.SetupSignalHandler()

	if err = webhookRegistrationClient.Register(); err != nil {
		glog.Fatalf("Failed registering Admission Webhooks: %v\n", err)
	}

	//--------
	pInformer.Start(stopCh)
	go pc.Run(1, stopCh)
	go pvc.Run(1, stopCh)
	go egen.Run(1, stopCh)

	//TODO add WG for the go routines?
	//--------
	// eventController.Run(stopCh)
	// genControler.Run(stopCh)
	// annotationsController.Run(stopCh)
	// if err = policyController.Run(stopCh); err != nil {
	// 	glog.Fatalf("Error running PolicyController: %v\n", err)
	// }
	server.RunAsync()
	<-stopCh
	server.Stop()
	// genControler.Stop()
	// eventController.Stop()
	// annotationsController.Stop()
	// policyController.Stop()
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&serverIP, "serverIP", "", "IP address where Kyverno controller runs. Only required if out-of-cluster.")
	flag.StringVar(&filterK8Resources, "filterK8Resources", "", "k8 resource in format [kind,namespace,name] where policy is not evaluated by the admission webhook. example --filterKind \"[Deployment, kyverno, kyverno]\" --filterKind \"[Deployment, kyverno, kyverno],[Events, *, *]\"")
	config.LogDefaultFlags()
	flag.Parse()
}
