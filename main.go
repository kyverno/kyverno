package main

import (
	"flag"
	"log"

	"github.com/nirmata/kube-policy/policycontroller"
	"github.com/nirmata/kube-policy/server"
	"github.com/nirmata/kube-policy/webhooks"

	client "github.com/nirmata/kube-policy/client"
	policyclientset "github.com/nirmata/kube-policy/pkg/client/clientset/versioned"
	informers "github.com/nirmata/kube-policy/pkg/client/informers/externalversions"
	event "github.com/nirmata/kube-policy/pkg/event"
	policyviolation "github.com/nirmata/kube-policy/pkg/policyviolation"
	"k8s.io/sample-controller/pkg/signals"
)

var (
	kubeconfig string
	cert       string
	key        string
)

func main() {
	clientConfig, err := createClientConfig(kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v\n", err)
	}

	client, err := client.NewDynamicClient(clientConfig, nil)
	if err != nil {
		log.Fatalf("Error creating client: %v\n", err)
	}

	policyClientset, err := policyclientset.NewForConfig(clientConfig)
	if err != nil {
		log.Fatalf("Error creating policyClient: %v\n", err)
	}

	//TODO wrap the policyInformer inside a factory
	policyInformerFactory := informers.NewSharedInformerFactory(policyClientset, 0)
	policyInformer := policyInformerFactory.Nirmata().V1alpha1().Policies()

	eventController := event.NewEventController(client, policyInformer.Lister(), nil)
	violationBuilder := policyviolation.NewPolicyViolationBuilder(client, policyInformer.Lister(), policyClientset, eventController, nil)

	policyController := policycontroller.NewPolicyController(policyClientset,
		client,
		policyInformer,
		violationBuilder,
		eventController,
		nil)

	mutationWebhook, err := webhooks.CreateMutationWebhook(clientConfig,
		client,
		policyInformer.Lister(),
		eventController,
		nil)
	if err != nil {
		log.Fatalf("Error creating mutation webhook: %v\n", err)
	}

	tlsPair, err := initTlsPemPair(cert, key, clientConfig, client)
	if err != nil {
		log.Fatalf("Failed to initialize TLS key/certificate pair: %v\n", err)
	}

	server, err := server.NewWebhookServer(tlsPair, mutationWebhook, nil)
	if err != nil {
		log.Fatalf("Unable to create webhook server: %v\n", err)
	}
	server.RunAsync()

	stopCh := signals.SetupSignalHandler()
	policyInformerFactory.Start(stopCh)
	if err = eventController.Run(stopCh); err != nil {
		log.Fatalf("Error running EventController: %v\n", err)
	}

	if err = policyController.Run(stopCh); err != nil {
		log.Fatalf("Error running PolicyController: %v\n", err)
	}

	<-stopCh
	server.Stop()
	policyController.Stop()
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&cert, "cert", "", "TLS certificate used in connection with cluster.")
	flag.StringVar(&key, "key", "", "Key, used in TLS connection.")
	flag.Parse()
}
